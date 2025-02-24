package basic

import (
	"errors"

	"golang.org/x/exp/constraints"
)

var (
	nilElement = errors.New("PriorityQueue size is 0")
)

type PriorityQueue[T constraints.Ordered] struct {
	arr  []T
	zero T
	size int
	Max  bool
}

func (h *PriorityQueue[T]) Offer(e T) {
	if cap(h.arr) >= h.size {
		h.arr = append(h.arr, e)
	} else {
		h.arr[h.size-1] = e
	}
	h.size++
	for i := (h.size / 2) - 1; i >= 0; i-- {
		h.down(i)
	}
}

func (h *PriorityQueue[T]) down(i int) {
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

func (h *PriorityQueue[T]) compare(i, j int) bool {
	if h.Max {
		return h.arr[i] < h.arr[j]
	} else {
		return h.arr[i] > h.arr[j]
	}
}

func (h *PriorityQueue[T]) swap(i, j int) {
	tmp := h.arr[i]
	h.arr[i] = h.arr[j]
	h.arr[j] = tmp
}

func (h *PriorityQueue[T]) Poll() (T, error) {
	if h.size > 0 {
		res := h.arr[0]
		h.arr[0] = h.arr[h.size-1]
		h.size--
		if h.size != 0 {
			for i := (h.size / 2) - 1; i >= 0; i-- {
				h.down(i)
			}
		}
		if half := cap(h.arr) / 2; h.size < half/2 {
			h.arr = h.arr[0:h.size:half]
		}
		return res, nil
	} else {
		return h.zero, nilElement
	}
}

func (h *PriorityQueue[T]) Peek() (T, error) {
	if h.size > 0 {
		return h.arr[0], nil
	} else {
		return h.zero, nilElement
	}
}

func (h *PriorityQueue[T]) Size() int {
	return h.size
}

func (h *PriorityQueue[T]) Contains(e T) bool {
	return h.indexOf(e) != -1
}

func (h *PriorityQueue[T]) indexOf(e T) int {
	for i := 0; i < h.size; i++ {
		if h.arr[i] == e {
			return i
		}
	}

	return -1
}
