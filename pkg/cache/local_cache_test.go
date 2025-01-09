package cache

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestLocalCache(t *testing.T) {
	LocalCache.Set("test", "testValue", time.Second*5)

	timeout, _ := context.WithTimeout(context.Background(), time.Second*10)

	for {
		select {
		case <-timeout.Done():
			return
		case <-time.After(time.Second * 1):
			get, e := LocalCache.Get("test")
			fmt.Printf("e = %v, value = %v\n", e, get)
		}
	}
}
