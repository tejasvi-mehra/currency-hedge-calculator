package runner

import (
	"context"

	"golang.org/x/sync/errgroup"
)

// Runnable is a unit of work that can be coordinated in parallel.
type Runnable interface {
	Run(ctx context.Context) error
}

// RunnableFunc adapts a function into the Runnable interface.
type RunnableFunc func(ctx context.Context) error

// Run executes the runnable function.
func (f RunnableFunc) Run(ctx context.Context) error {
	return f(ctx)
}

// RunParallel starts all runnables and cancels siblings when one fails.
func RunParallel(ctx context.Context, runnables ...Runnable) error {
	group, groupCtx := errgroup.WithContext(ctx)
	for _, runnable := range runnables {
		if runnable == nil {
			continue
		}
		current := runnable
		group.Go(func() error {
			return current.Run(groupCtx)
		})
	}
	return group.Wait()
}
