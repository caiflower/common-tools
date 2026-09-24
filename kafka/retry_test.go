package xkafka

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRetryPolicySucceedsAfterRetry(t *testing.T) {
	attempts := 0
	policy := RetryPolicy{
		MaxAttempts: 3,
		Delay:       func(int) time.Duration { return time.Millisecond },
	}

	err := policy.Run(context.Background(), func(attempt int) error {
		attempts++
		if attempt < 3 {
			return errors.New("retry")
		}
		return nil
	})

	require.NoError(t, err)
	assert.Equal(t, 3, attempts)
}

func TestRetryPolicyReturnsLastErrorAfterExhaustion(t *testing.T) {
	expected := errors.New("still failing")
	attempts := 0
	policy := RetryPolicy{
		MaxAttempts: 2,
		Delay:       func(int) time.Duration { return time.Millisecond },
	}

	err := policy.Run(context.Background(), func(int) error {
		attempts++
		return expected
	})

	assert.ErrorIs(t, err, expected)
	assert.Equal(t, 2, attempts)
}

func TestRetryPolicyStopsOnContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	attempts := 0
	policy := RetryPolicy{
		MaxAttempts: 3,
		Delay: func(int) time.Duration {
			cancel()
			return time.Second
		},
	}

	err := policy.Run(ctx, func(int) error {
		attempts++
		return errors.New("retry")
	})

	assert.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, 1, attempts)
}

func TestRetryDelay(t *testing.T) {
	assert.Equal(t, time.Second, RetryDelay(1))
	assert.Equal(t, 2*time.Second, RetryDelay(2))
	assert.Equal(t, 30*time.Second, RetryDelay(10))
}
