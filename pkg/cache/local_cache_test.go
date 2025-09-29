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
