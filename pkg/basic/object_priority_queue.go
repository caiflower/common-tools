package basic

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

func (h *ObjectPriorityQueue[T]) Peek() (T, error) {
	if h.size > 0 {
		return h.arr[0], nil
	} else {
		return h.zero, nilElement
	}
}

//func (h *ObjectPriorityQueue[T]) Update(e T) {
//	if index := h.indexOf(e); index != -1 {
//		h.arr[index] = e
//		for i := (h.size / 2) - 1; i >= 0; i-- {
//			h.down(i)
//		}
//		if half := cap(h.arr) / 2; h.size < half/2 {
//			h.arr = h.arr[0:h.size:half]
//		}
//	}
//}
//
//func (h *ObjectPriorityQueue[T]) Delete(e T) T {
//	if index := h.indexOf(e); index != -1 {
//		res := h.arr[index]
//		h.arr[index] = h.arr[h.size-1]
//		h.size--
//		if h.size != 0 {
//			for i := (h.size / 2) - 1; i >= 0; i-- {
//				h.down(i)
//			}
//		}
//		if half := cap(h.arr) / 2; h.size < half/2 {
//			h.arr = h.arr[0:h.size:half]
//		}
//		return res
//	} else {
//		return h.zero
//	}
//}

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
