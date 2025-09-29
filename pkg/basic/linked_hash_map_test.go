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

 package basic

import (
	"fmt"
	"testing"
)

func TestLinkedHashMap(t *testing.T) {
	linkedHashMap := NewLinkHashMap()
	linkedHashMap.Put("key1", "value1")
	linkedHashMap.Put("key2", "value2")
	linkedHashMap.Put("key3", "value3")

	fmt.Println(linkedHashMap)

	get, b := linkedHashMap.Get("key2")
	fmt.Println(get == "value2")
	fmt.Println(b)

	fmt.Println(linkedHashMap)

	linkedHashMap.Remove("key3")
	fmt.Println(linkedHashMap)
	linkedHashMap.Remove("key2")
	fmt.Println(linkedHashMap)
	linkedHashMap.Remove("key1")
	fmt.Println(linkedHashMap)
}
