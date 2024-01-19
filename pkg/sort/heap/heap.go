package heap

import (
	"errors"
)

type Api interface {
	Add(val interface{})
	Pop() (interface{}, error)
	Peek() (interface{}, error)
	IsEmpty() bool
}

type Heap[T int | int8 | int16 | int32 | int64 | float32 | float64] struct {
	common[T]
}

func NewTopMin[T int | int8 | int16 | int32 | int64 | float32 | float64]() *Heap[T] {
	return &Heap[T]{common: common[T]{size: 0, arr: make([]T, 0)}}
}

func (h *Heap[T]) Add(val T) {
	if len(h.arr) > h.size {
		h.arr[h.size] = val
	} else {
		h.arr = append(h.arr, val)
	}
	h.size++
	i := h.size - 1
	for i > 0 {
		p := (i - 1) / 2
		if h.arr[p] > h.arr[i] {
			h.swap(i, p)
			i = p
		} else {
			break
		}
	}
}

func (h *Heap[T]) Pop() (T, error) {
	if h.size == 0 {
		return 0, errors.New("empty heap")
	} else {
		element := h.arr[0]
		h.arr[0] = h.arr[h.size-1]
		h.size--
		i := 0
		for i < h.size {
			left := i*2 + 1
			right := i*2 + 2
			if left < h.size && right < h.size {
				t := -1
				if h.arr[left] < h.arr[right] {
					t = left
				} else {
					t = right
				}
				if h.arr[t] < h.arr[i] {
					h.swap(i, t)
					i = t
				} else {
					break
				}
			} else if left < h.size && h.arr[left] < h.arr[i] {
				h.swap(i, left)
				break
			} else {
				break
			}
		}
		return element, nil
	}
}

type MaxHeap[T int | int8 | int16 | int32 | int64 | float32 | float64] struct {
	common[T]
}

func NewTopMax[T int | int8 | int16 | int32 | int64 | float32 | float64]() *MaxHeap[T] {
	return &MaxHeap[T]{common: common[T]{size: 0, arr: make([]T, 0)}}
}

func (h *MaxHeap[T]) Add(val T) {
	if len(h.arr) > h.size {
		h.arr[h.size] = val
	} else {
		h.arr = append(h.arr, val)
	}
	h.size++
	i := h.size - 1
	for i > 0 {
		p := (i - 1) / 2
		if h.arr[p] < h.arr[i] {
			h.swap(i, p)
			i = p
		} else {
			break
		}
	}
}

func (h *MaxHeap[T]) Pop() (T, error) {
	if h.size == 0 {
		return 0, errors.New("empty heap")
	} else {
		element := h.arr[0]
		h.arr[0] = h.arr[h.size-1]
		h.size--
		i := 0
		for i < h.size {
			left := i*2 + 1
			right := i*2 + 2
			if left < h.size && right < h.size {
				t := -1
				if h.arr[left] > h.arr[right] {
					t = left
				} else {
					t = right
				}
				if h.arr[t] > h.arr[i] {
					h.swap(i, t)
					i = t
				} else {
					break
				}
			} else if left < h.size && h.arr[left] > h.arr[i] {
				h.swap(i, left)
				break
			} else {
				break
			}
		}
		return element, nil
	}
}

func (h *common[T]) Peek() (T, error) {
	if h.size == 0 {
		return 0, errors.New("empty heap")
	} else {
		return h.arr[0], nil
	}
}

func (h *common[T]) swap(i, j int) {
	t := h.arr[j]
	h.arr[j] = h.arr[i]
	h.arr[i] = t
}

func (h *common[T]) IsEmpty() bool {
	return h.size == 0
}

type common[T int | int8 | int16 | int32 | int64 | float32 | float64] struct {
	size int
	arr  []T
}
