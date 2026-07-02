package backoff

import (
	"context"
	"time"
)

// Strategy returns a delay for a given attempt number.
type Strategy interface {
	Duration(attempt int) time.Duration
}

// Exponential computes exponentially increasing delays bounded by Max.
type Exponential struct {
	Initial time.Duration
	Max     time.Duration
}

// Duration returns the delay for the supplied attempt (1-indexed).
func (b Exponential) Duration(attempt int) time.Duration {
	if attempt <= 1 {
		return b.clamp(b.Initial)
	}

	delay := b.Initial
	for i := 1; i < attempt; i++ {
		delay *= 2
		if delay >= b.Max {
			return b.clamp(b.Max)
		}
	}
	return b.clamp(delay)
}

func (b Exponential) clamp(value time.Duration) time.Duration {
	if value <= 0 {
		value = 100 * time.Millisecond
	}
	if b.Max > 0 && value > b.Max {
		return b.Max
	}
	return value
}

// Sleep blocks for strategy-defined delay or until context cancellation.
func Sleep(ctx context.Context, strategy Strategy, attempt int) error {
	if strategy == nil {
		return nil
	}
	delay := strategy.Duration(attempt)
	if delay <= 0 {
		return nil
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
