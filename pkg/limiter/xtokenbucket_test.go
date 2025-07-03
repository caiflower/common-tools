package limiter

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestXTokenBucketNonBlock(t *testing.T) {
	bucket := NewXTokenBucket(1000, 1000)

	success := 0
	failed := 0
	for i := 0; i < 10000; i++ {
		if ok := bucket.TakeTokenNonBlocking(); ok {
			fmt.Printf("get token suceess.\n")
			success++
		} else {
			fmt.Printf("get token failed.\n")
			failed++
		}
	}

	fmt.Printf("success: %d, failed: %d\n", success, failed)
}

func TestXTokenBucket(t *testing.T) {
	bucket := NewXTokenBucket(1000, 1000)
	now := time.Now()
	group := sync.WaitGroup{}

	for i := 0; i < 10000; i++ {
		group.Add(1)
		go func(v int) {
			defer group.Done()
			bucket.TakeToken()
			fmt.Printf("i=%v\n", v)
		}(i)
	}

	group.Wait()
	fmt.Printf("time cost: %v\n", time.Since(now).Seconds())
}
