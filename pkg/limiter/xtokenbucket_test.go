/*
 * Copyright 2024 caiflower Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

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
