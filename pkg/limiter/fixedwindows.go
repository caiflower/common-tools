package limiter

import (
	"context"
	"time"
)

// 固定窗口

type FixedWindowLimiter struct {
	concurrent chan struct{}
}

func NewFixedWindow(concurrent int) *FixedWindowLimiter {
	return &FixedWindowLimiter{
		concurrent: make(chan struct{}, concurrent),
	}
}

func (l *FixedWindowLimiter) TakeToken() {
	l.concurrent <- struct{}{}
}

func (l *FixedWindowLimiter) TakeTokenNonBlocking() bool {
	select {
	case l.concurrent <- struct{}{}:
		return true
	default:
		return false
	}
}

func (l *FixedWindowLimiter) ReleaseToken() {
	<-l.concurrent
}

func (l *FixedWindowLimiter) TakeTokenWithTimeout(timeout time.Duration) bool {
	ctx, _ := context.WithTimeout(context.Background(), timeout)
	for {
		select {
		case <-ctx.Done():
			return false
		case l.concurrent <- struct{}{}:
			return true
		default:

		}
	}
}

func (l *FixedWindowLimiter) Wait() {
	for {
		if len(l.concurrent) == 0 {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func (l *FixedWindowLimiter) Close() {
	close(l.concurrent)
}
