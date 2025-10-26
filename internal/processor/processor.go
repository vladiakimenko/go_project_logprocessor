package processor

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"sync/atomic"

	"github/vladiakimenko/logprocessor/internal/bootstrap"
)

// Глобальные счётчики для статистики
var totalRequests int64
var errorCount int64
var avgRespTime int64     // milliseconds * 1000
var requestsByIP sync.Map // map[string]*int64

// мапа для подсчёта топовых значений
var topMaps sync.Map // field -> *sync.Map (value -> *int64)

// структура ключ-значение для подстчёта топ-значений
type kv struct {
	Key   string
	Value int64
}

// Читаем фрагмент файла, конвертим строку в дто, стримим. Функция генератор данных.
func ReadLogsBatch(ctx context.Context, filePath string, start, end int64) (<-chan LogEntry, error) {
	out := make(chan LogEntry)

	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}

	go func() {
		defer close(out)
		defer file.Close()

		section := io.NewSectionReader(file, start, end-start)
		csvReader := csv.NewReader(section)

		// пропускаем header'ы
		if start == 0 {
			_, _ = csvReader.Read()
		}

		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			record, err := csvReader.Read()
			if err != nil {
				if err == io.EOF {
					break
				}
				bootstrap.Logger.Error("Failed to read line", "error", err)
				continue
			}

			entry, err := parseCSVRecord(record)
			if err != nil {
				bootstrap.Logger.Error("Failed to parse line",
					"error", err,
					"line", record,
				)
				continue
			}
			out <- entry
		}
	}()

	return out, nil
}

// Синхронная функция с логикой для фильтрации. Проверяем подходит ли запись под заданные параметры, если да - выплевываем json в stdout.
func FilterOut(entry LogEntry, field string, value any) LogEntry {
	valStr := fmt.Sprint(value)
	matched := false

	switch strings.ToLower(field) {
	case "timestamp":
		matched = entry.Timestamp == valStr
	case "ip":
		matched = entry.IP == valStr
	case "method":
		matched = strings.EqualFold(entry.Method, valStr)
	case "url":
		matched = entry.URL == valStr
	case "status":
		matched = fmt.Sprint(entry.StatusCode) == valStr
	case "response_time":
		matched = fmt.Sprint(entry.ResponseTime) == valStr
	}

	if matched {
		b, _ := json.Marshal(entry)
		fmt.Println(string(b))
	}

	return entry
}

// Синхронная функция для обновления статистических данных
func UpdateStats(entry LogEntry) LogEntry {

	// Атомарно накручиваем счётчики, избегаем блокировки глобальных объектов мьютексами
	atomic.AddInt64(&totalRequests, 1)
	if entry.StatusCode >= 400 {
		atomic.AddInt64(&errorCount, 1)
	}

	// sync.Map реализован так, что даже если несколько горутин вызывают LoadOrStore для одого ключа, добавлено будет только одно значение
	cnt, _ := requestsByIP.LoadOrStore(entry.IP, new(int64))
	atomic.AddInt64(cnt.(*int64), 1)

	// рекрусивно апдейтим среднее значение (также с помощью sync/atomic)
	for {
		old := atomic.LoadInt64(&avgRespTime)
		newAvg := old + int64(entry.ResponseTime*1000) - old/atomic.LoadInt64(&totalRequests)
		if atomic.CompareAndSwapInt64(&avgRespTime, old, newAvg) {
			break
		}
	}

	return entry
}

// Синхронная функция подсчёта топовых значений
func UpdateTop(entry LogEntry, field string) LogEntry {
	var value string

	switch strings.ToLower(field) {
	case "timestamp":
		value = entry.Timestamp
	case "ip":
		value = entry.IP
	case "method":
		value = entry.Method
	case "url":
		value = entry.URL
	case "status":
		value = fmt.Sprint(entry.StatusCode)
	case "response_time":
		value = fmt.Sprint(entry.ResponseTime)
	default:
		return entry
	}

	// Достаём или создаём мапу для этого поля
	existingSubMap, _ := topMaps.LoadOrStore(field, &sync.Map{})
	// ассертим тип
	fieldSubMap := existingSubMap.(*sync.Map)

	// Создаём/получаем счётчик для конкретного значения
	existingCounter, _ := fieldSubMap.LoadOrStore(value, new(int64))
	counter := existingCounter.(*int64)

	// Атомарно инкрементим
	atomic.AddInt64(counter, 1)

	return entry
}

// Подведение результатов работы в зависимости от типа задачи
func MergeResults(taskType, field string) error {
	switch taskType {
	case "stats":
		stats := Statistics{
			TotalRequests:   int(atomic.LoadInt64(&totalRequests)),
			ErrorCount:      int(atomic.LoadInt64(&errorCount)),
			RequestsByIP:    make(map[string]int),
			AverageRespTime: float64(atomic.LoadInt64(&avgRespTime)) / 1000,
		}

		// собираем данные по IP
		requestsByIP.Range(func(key, value interface{}) bool {
			ip := key.(string)
			cnt := value.(*int64)
			stats.RequestsByIP[ip] = int(atomic.LoadInt64(cnt))
			return true
		})

		fmt.Printf(
			"Statistics:\nTotalRequests: %d\nErrorCount: %d\nAverageRespTime: %.2fms\nRequestsByIP:\n",
			stats.TotalRequests, stats.ErrorCount, stats.AverageRespTime,
		)
		for ip, cnt := range stats.RequestsByIP {
			fmt.Printf("  %s: %d\n", ip, cnt)
		}

	case "top":
		if field == "" {
			return fmt.Errorf("field was not provided for the 'top' values task")
		}

		// Читаем подкарту для поля
		subMapAny, ok := topMaps.Load(field)
		if !ok {
			fmt.Printf("No results for field %s\n", field)
			return nil
		}

		subMap := subMapAny.(*sync.Map) // ассертим

		topNumber := bootstrap.Settings.Core.TopValuesNumber
		topList := []kv{}

		subMap.Range(func(key, val any) bool {
			valueStr := key.(string)
			count := atomic.LoadInt64(val.(*int64))
			kvItem := kv{Key: valueStr, Value: count}

			inserted := false
			for i := range topList {
				if count > topList[i].Value {
					// Вставляем элемент на нужную позицию
					topList = append(topList[:i], append([]kv{kvItem}, topList[i:]...)...)
					inserted = true
					break
				}
			}

			if !inserted {
				// Если элемент не вставился в середину, добавляем в конец, если есть место
				if len(topList) < topNumber {
					topList = append(topList, kvItem)
				}
				// Иначе игнорируем
			}

			// обрезаем лишние элементы
			if len(topList) > topNumber {
				topList = topList[:topNumber]
			}

			return true
		})

		// вывод
		fmt.Printf("Top %d values for %s:\n", len(topList), field)
		for _, item := range topList {
			fmt.Printf("  %s: %d\n", item.Key, item.Value)
		}

	case "filter":
		// фильтр уже выводил результаты в процессе, merge не нужен
	default:
		return fmt.Errorf("incorrect taskType provided: %s", taskType)
	}
	return nil
}
