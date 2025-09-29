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
