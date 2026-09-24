package xkafka

import (
	"context"
	"time"
)

// RetryPolicy controls a bounded retry loop.
// MaxAttempts includes the first execution.
type RetryPolicy struct {
	MaxAttempts int
	Delay       func(attempt int) time.Duration
}

// Run executes fn until it succeeds or the policy is exhausted.
// The attempt passed to fn starts at 1.
func (p RetryPolicy) Run(ctx context.Context, fn func(attempt int) error) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if p.MaxAttempts < 1 {
		p.MaxAttempts = 1
	}

	var lastErr error
	for attempt := 1; attempt <= p.MaxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}

		lastErr = fn(attempt)
		if lastErr == nil {
			return nil
		}
		if attempt == p.MaxAttempts {
			return lastErr
		}

		var delay time.Duration
		if p.Delay != nil {
			delay = p.Delay(attempt)
		}
		if delay <= 0 {
			continue
		}

		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}

	return lastErr
}

// RetryDelay returns the default exponential retry delay: 1s, 2s, 4s... capped at 30s.
func RetryDelay(attempt int) time.Duration {
	return RetryDelayWithBase(attempt, time.Second, 30*time.Second)
}

// RetryDelayWithBase returns an exponential delay capped at max.
func RetryDelayWithBase(attempt int, base, max time.Duration) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	if base <= 0 {
		return 0
	}

	delay := base
	for i := 1; i < attempt && delay < max; i++ {
		delay *= 2
	}
	if max > 0 && delay > max {
		return max
	}
	return delay
}
