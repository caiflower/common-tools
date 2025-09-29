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
	"strconv"
	"testing"
)

type objectHeapItem struct {
	priority int
	Name     string
}

func (o objectHeapItem) String() string {
	return strconv.Itoa(o.priority)
}

func TestObjectHeap(t *testing.T) {

	h := ObjectPriorityQueue[objectHeapItem]{
		Max: false,
	}
	h.Offer(objectHeapItem{priority: 1, Name: "1"})
	h.Offer(objectHeapItem{priority: 2, Name: "2"})
	h.Offer(objectHeapItem{priority: 3, Name: "3"})
	h.Offer(objectHeapItem{priority: 4, Name: "4"})

	//
	for i := 0; i < 4; i++ {
		poll, err := h.Poll()
		if err != nil {
			panic(err)
		}
		fmt.Println(poll.Name)
	}
}
