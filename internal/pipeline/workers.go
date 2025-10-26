package pipeline

import (
	"context"
	"sync"

	"github/vladiakimenko/logprocessor/internal/bootstrap"
)

// Запускаем обработку пайплайна при помощи пула воркеров
func SpawnPipelineWorkers[T any](ctx context.Context, pipelines []*Pipeline[T], numWorkers int, wg *sync.WaitGroup) {
	jobs := make(chan *Pipeline[T], len(pipelines))

	// Запускаем горутины с воркерами
	wg.Add(numWorkers)
	for i := 0; i < numWorkers; i++ {
		go func() {
			defer wg.Done()
			for p := range jobs {
				select {
				case <-ctx.Done():
					return
				default:
					p.Run(ctx)
				}
			}
		}()
	}
	bootstrap.Logger.Debug("Engaged workers", "amount", numWorkers)

	// Насыпаем инстансы пайплайнов
	go func() {
	outer:
		for _, p := range pipelines {
			select {
			case <-ctx.Done():
				break outer
			case jobs <- p:
				bootstrap.Logger.Debug("Woreker picked up pipeline", "pipeline", p.ID)
			}
		}
		close(jobs)
	}()
}
