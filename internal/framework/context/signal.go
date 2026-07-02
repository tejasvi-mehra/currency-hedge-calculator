package context

import (
	"context"
	"os/signal"
	"syscall"
)

// WithShutdownSignals returns a context canceled on SIGINT or SIGTERM.
func WithShutdownSignals(parent context.Context) (context.Context, context.CancelFunc) {
	return signal.NotifyContext(parent, syscall.SIGINT, syscall.SIGTERM)
}
