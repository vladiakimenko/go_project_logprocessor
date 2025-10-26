package pipeline

import (
	"context"

	"github/vladiakimenko/logprocessor/internal/bootstrap"
)

// Генератор данных (не зависит от входного канала)
type SourceJob[T any] func(ctx context.Context) <-chan T

// Обработчик данных принимает входной канал
type Job[T any] func(ctx context.Context, in <-chan T)

// Синхронная функция с логикой обработки
type JobFunction[T any] func(item T) T

type Pipeline[T any] struct {
	ID        int
	Source    SourceJob[T] // генератор данных
	Processor Job[T]       // обработчик данных
}

// Cоединяет источник и последующие задачи пайплайна
func (p *Pipeline[T]) Run(ctx context.Context) {
	bootstrap.Logger.Debug("Pipeline started", "pipeline", p.ID)
	sourceCh := p.Source(ctx)
	bootstrap.Logger.Debug("Source channel created for pipeline", "pipeline", p.ID)
	bootstrap.Logger.Debug("Starting processor execution", "pipeline", p.ID)
	p.Processor(ctx, sourceCh)
	bootstrap.Logger.Debug("Pipeline finished", "pipeline", p.ID)
}

// Хелпер, который врапает синхронную функцию, превращая ее в джобу для пайплайна
func MakeJob[T any](fn JobFunction[T]) Job[T] {
	return func(ctx context.Context, in <-chan T) {
		for item := range in {
			select {
			case <-ctx.Done():
				return
			default:
				fn(item)
			}
		}
	}

}
