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

import "errors"

var nilElement = errors.New("size is 0")

type Ordered interface {
	// String sort and uuid
	String() string
}

type ObjectPriorityQueue[T Ordered] struct {
	arr  []T
	zero T
	size int
	Max  bool
}

func (h *ObjectPriorityQueue[T]) Offer(e T) {
	if h.size < cap(h.arr) {
		h.arr = h.arr[:h.size+1]
		h.arr[h.size] = e
	} else {
		h.arr = append(h.arr, e)
	}
	h.size++
	h.up(h.size - 1)
}

func (h *ObjectPriorityQueue[T]) down(i int) {
	t := i
	if _t := i*2 + 1; _t < h.size && h.compare(t, _t) {
		t = _t
	}
	if _t := i*2 + 2; _t < h.size && h.compare(t, _t) {
		t = _t
	}

	if t != i {
		h.swap(i, t)
		h.down(t)
	}
}

func (h *ObjectPriorityQueue[T]) up(i int) {
	for i > 0 {
		parent := (i - 1) / 2
		if h.compare(parent, i) {
			h.swap(parent, i)
			i = parent
		} else {
			break
		}
	}
}

func (h *ObjectPriorityQueue[T]) compare(i, j int) bool {
	if h.Max {
		return h.arr[i].String() < h.arr[j].String()
	} else {
		return h.arr[i].String() > h.arr[j].String()
	}
}

func (h *ObjectPriorityQueue[T]) swap(i, j int) {
	tmp := h.arr[i]
	h.arr[i] = h.arr[j]
	h.arr[j] = tmp
}

func (h *ObjectPriorityQueue[T]) Poll() (T, error) {
	if h.size > 0 {
		res := h.arr[0]
		h.size--
		if h.size > 0 {
			h.arr[0] = h.arr[h.size]
			h.down(0)
		}
		h.arr = h.arr[:h.size]
		if half := cap(h.arr) / 2; h.size < half && cap(h.arr) > 512 {
			newArr := make([]T, h.size, half)
			copy(newArr, h.arr)
			h.arr = newArr
		}
		return res, nil
	} else {
		return h.zero, nilElement
	}
}

func (h *ObjectPriorityQueue[T]) Peek() (T, error) {
	if h.size > 0 {
		return h.arr[0], nil
	} else {
		return h.zero, nilElement
	}
}

func (h *ObjectPriorityQueue[T]) Size() int {
	return h.size
}

func (h *ObjectPriorityQueue[T]) Contains(e T) bool {
	return h.indexOf(e) != -1
}

func (h *ObjectPriorityQueue[T]) indexOf(e T) int {
	for i := 0; i < h.size; i++ {
		if h.arr[i].String() == e.String() {
			return i
		}
	}

	return -1
}
