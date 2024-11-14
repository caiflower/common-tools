package limiter

import (
	"fmt"
	"testing"
	"time"
)

func TestTokenBucket(t *testing.T) {
	bucket := NewTokenBucket(1000)
	bucket.Startup()
	defer bucket.Close()

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

func TestTokenBucketFull(t *testing.T) {
	bucket := NewTokenBucket(1000)

	bucket.Startup()

	time.Sleep(3 * time.Second)

	bucket.Close()

	time.Sleep(1 * time.Second)
}
