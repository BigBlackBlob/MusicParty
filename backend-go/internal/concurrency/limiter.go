package concurrency

import (
	"context"

	"golang.org/x/sync/semaphore"
)

type Limiter struct {
	semaphore *semaphore.Weighted
}

func NewLimiter(maxConcurrent int64) *Limiter {
	return &Limiter{semaphore: semaphore.NewWeighted(maxConcurrent)}
}

func (l *Limiter) Run(ctx context.Context, operation func(context.Context) error) error {
	if err := l.semaphore.Acquire(ctx, 1); err != nil {
		return err
	}
	defer l.semaphore.Release(1)
	return operation(ctx)
}
