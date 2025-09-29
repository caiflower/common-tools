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
	"fmt"
	"testing"
	"time"
)

func TestNewLRUCache(t *testing.T) {
	cache := NewLRUCache(2)
	go cache.Put("key1", "value1")
	go cache.Put("key2", "value2")
	go cache.Put("key3", "value3")
	go cache.Put("key1", "value1")
	fmt.Println(cache)

	cache1 := NewLRUCache(3)
	go cache1.Put("key1", "value1")
	go cache1.Put("key2", "value2")
	go cache1.Put("key3", "value3")
	go cache1.Put("key2", "value2")
	go cache1.Put("key1", "value1")
	go cache1.Put("key3", "value3")

	time.Sleep(1 * time.Second)

	for i := 1; i <= 3; i++ {
		go fmt.Println(cache1.Get(fmt.Sprintf("key%d", i)))
		go fmt.Println(cache1.Size())
	}

	time.Sleep(1 * time.Second)
}
