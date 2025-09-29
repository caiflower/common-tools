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

func TestHeap_Contains(t *testing.T) {
	nums := []int{4, 5, 1, 6, 2, 7, 3, 8, 10, 20, 13, 30, 10}
	h := PriorityQueue[int]{
		Max: false,
	}

	for i := 0; i < len(nums); i++ {
		h.Offer(nums[i])
	}

	for i := 0; i < len(nums); i++ {
		poll, err := h.Poll()
		if err != nil {
			panic(err)
		}
		fmt.Println(poll)
	}
}
