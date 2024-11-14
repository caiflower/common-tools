package limiter

import (
	"fmt"
	"testing"
	"time"
)

func TestFixedWindow(t *testing.T) {
	limiter := NewFixedWindow(10)
	fmt.Printf("limiter = %v\n", limiter)

	for i := 0; i < 1000; i++ {
		go func(v int) {
			defer limiter.ReleaseToken()
			limiter.TakeToken()
			fmt.Printf("i=%v\n", v)
		}(i)
	}

	limiter.Wait()
}

func TestFixedWindowWithTimeout(t *testing.T) {
	limiter := NewFixedWindow(10)
	fmt.Printf("limiter = %v\n", limiter)

	for i := 0; i < 1000; i++ {
		go func(v int) {
			if get := limiter.TakeTokenWithTimeout(1 * time.Millisecond); get {
				fmt.Printf("i=%v\n", v)
				time.Sleep(10 * time.Millisecond)
				defer limiter.ReleaseToken()
			} else {
				fmt.Printf("get Token failed\n")
			}
		}(i)
	}

	limiter.Wait()
}
