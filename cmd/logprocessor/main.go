package main

import (
	"context"
	"fmt"
	"github/vladiakimenko/logprocessor/internal/bootstrap"
	"github/vladiakimenko/logprocessor/internal/pipeline"
	"github/vladiakimenko/logprocessor/internal/processor"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

// типы для обработки аргументов запуска
const (
	TaskFilter string = "filter"
	TaskStats  string = "stats"
	TaskTop    string = "top"
)

type Task struct {
	Type  string
	Func  func(item processor.LogEntry) processor.LogEntry
	Field string
	Value interface{}
}

// валидация задачи из аргументов запуска
func parseTask() (*Task, error) {
	if len(os.Args) < 2 {
		return nil, fmt.Errorf("task argument required: filter|stats|top")
	}

	task := &Task{}
	task.Type = os.Args[1]

	switch task.Type {
	case TaskStats:
		task.Func = processor.UpdateStats
	case TaskTop:
		if len(os.Args) < 3 {
			return nil, fmt.Errorf("top task requires a <field>")
		}
		task.Field = os.Args[2]
	case TaskFilter:
		if len(os.Args) < 4 {
			return nil, fmt.Errorf("filter task requires <field> <value>")
		}
		task.Field = os.Args[2]
		task.Value = os.Args[3]
	default:
		return nil, fmt.Errorf("unknown task: %s", task.Type)
	}

	return task, nil
}

func main() {
	/* Нет никакого смысла читать построчно, а потом распараллеливать нересурсоёмкую обработку.
	Также считаем недопустимым читать весь файл в оперативную память перед обработкой.
	Поэтому подумаем, как шардировать весь процесс от начала и до конца */

	// парсим аргументы
	task, err := parseTask()
	if err != nil {
		bootstrap.Logger.Error("Error parsing task: ", "error", err)
		os.Exit(1)
	}
	bootstrap.Logger.Debug("Task successfully parsed", "type", task.Type, "field", task.Field, "value", task.Value)

	// вычисляем байтовые оффсеты для одновременного чтения данных из файла
	filePath := bootstrap.Settings.Core.FilePath
	numWorkers := bootstrap.Settings.Core.WorkersNumber

	offsets, err := processor.ComputeChunks(
		filePath,
		numWorkers,
	)
	if err != nil {
		bootstrap.Logger.Error("Error calculating offsets: ", "error", err)
		os.Exit(1)
	}
	bootstrap.Logger.Debug("Computed file offsets", "totalChunks", len(offsets), "offsets", offsets)

	// создаём контекст с отменой и ловим ctrl+x
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// ловим SIGINT/SIGTERM
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\nReceived ctrl+, shutting down...")
		cancel()
	}()

	// выбираем job-функцию в зависимости от типа задачи
	var jobFn func(processor.LogEntry) processor.LogEntry
	switch task.Type {
	case TaskStats:
		jobFn = processor.UpdateStats
	case TaskTop:
		field := task.Field
		jobFn = func(entry processor.LogEntry) processor.LogEntry {
			return processor.UpdateTop(entry, field)
		}
	case TaskFilter:
		field := task.Field
		value := task.Value
		jobFn = func(entry processor.LogEntry) processor.LogEntry {
			return processor.FilterOut(entry, field, value)
		}
	}

	// создаём пайплайны
	var pipelines []*pipeline.Pipeline[processor.LogEntry]
	for i, pos := range offsets {
		start, end := pos[0], pos[1]
		p := &pipeline.Pipeline[processor.LogEntry]{
			ID: i,
			Source: func(ctx context.Context) <-chan processor.LogEntry {
				ch, _ := processor.ReadLogsBatch(ctx, filePath, start, end)
				return ch
			},
			Processor: pipeline.MakeJob(jobFn),
		}
		pipelines = append(pipelines, p)
	}

	// создаём WaitGroup для воркеров
	var wg sync.WaitGroup

	// запускаем обработку
	pipeline.SpawnPipelineWorkers(ctx, pipelines, numWorkers, &wg)

	// Ждём завершения всех воркеров
	wg.Wait()
	bootstrap.Logger.Debug("All workers finsihed duties")

	// подводим итоги
	processor.MergeResults(task.Type, task.Field)
}
