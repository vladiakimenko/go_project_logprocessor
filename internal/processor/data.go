package processor

import (
	"bufio"
	"errors"
	"os"
	"strconv"
	"strings"
)

type LogEntry struct {
	Timestamp    string // время в формате "2024-01-15 10:30:00"
	IP           string // IP адрес клиента
	Method       string // HTTP метод (GET, POST и т.д.)
	URL          string // путь запроса
	StatusCode   int    // HTTP статус код
	ResponseTime int    // время ответа в миллисекундах
}

type Statistics struct {
	TotalRequests   int            // общее количество запросов
	ErrorCount      int            // количество ошибок (статус >= 400)
	RequestsByIP    map[string]int // количество запросов с каждого IP
	AverageRespTime float64        // среднее время ответа
}

// Парсинг строки CSV в структуру LogEntry
func parseCSVRecord(record []string) (LogEntry, error) {
	if len(record) < 6 {
		return LogEntry{}, errors.New("invalid record: not enough fields")
	}

	for i := range record {
		record[i] = strings.TrimSpace(record[i])
	}

	status, err := strconv.Atoi(record[4])
	if err != nil {
		return LogEntry{}, err
	}
	respTime, err := strconv.Atoi(record[5])
	if err != nil {
		return LogEntry{}, err
	}

	entry := LogEntry{
		Timestamp:    record[0],
		IP:           record[1],
		Method:       record[2],
		URL:          record[3],
		StatusCode:   status,
		ResponseTime: respTime,
	}
	return entry, nil
}

// Хелпер для вычисления байтовых оффсетов в файле-источнике для указанного числа воркеров
// Гарантирует, что заданное смещение указывает на новую строку
func ComputeChunks(filePath string, numWorkers int) ([][2]int64, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, err
	}

	fileSize := info.Size()
	chunkSize := fileSize / int64(numWorkers)  // вычисляем размер фрагмента по количесву воркеров
	offsets := make([][2]int64, 0, numWorkers) // начало и конец фрагмента

	var start int64 = 0
	for i := 0; i < numWorkers; i++ {
		var end int64
		if i == numWorkers-1 {
			end = fileSize // конец для последнего фрагмента - EOF
		} else {
			end = start + chunkSize

			// Читаем с примерного оффсета и до символа '\n'
			file.Seek(end, 0)
			reader := bufio.NewReader(file)
			line, err := reader.ReadString('\n')
			if err != nil {
				return nil, err
			}
			end += int64(len(line)) // len(line) включает '\n'
		}

		offsets = append(offsets, [2]int64{start, end}) // сохраняем байтовый оффсет начала и конца фрагмента
		start = end
	}

	return offsets, nil
}
