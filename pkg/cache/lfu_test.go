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
)

func TestLFUCache(t *testing.T) {
	cache := NewLFUCache[string, string](2)

	cache.Put("key1", "value1")
	cache.Put("key2", "value2")
	cache.Put("key1", "value1")
	cache.Put("key3", "value3")

	fmt.Println(cache)

	cache.Put("key3", "value3")
	fmt.Println(cache)

	cache.Put("key2", "key2")

	fmt.Println(cache)
}
