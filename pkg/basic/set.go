package basic

type Set[T comparable] map[T]struct{}

func (s Set[T]) Add(elem T) {
	s[elem] = struct{}{}
}

func (s Set[T]) Remove(elem T) bool {
	if _, ok := s[elem]; ok {
		delete(s, elem)
		return ok
	}
	return false
}

func (s Set[T]) Contains(elem T) bool {
	_, ok := s[elem]
	return ok
}

func (s Set[T]) Clear() {
	var keys []T
	for k := range s {
		keys = append(keys, k)
	}
	for _, k := range keys {
		delete(s, k)
	}
}

func (s Set[T]) Empty() bool {
	return len(s) == 0
}

func (s Set[T]) Size() int {
	return len(s)
}

func (s Set[T]) ToSlice() []T {
	r := make([]T, s.Size(), s.Size())
	i := 0
	for k, _ := range s {
		r[i] = k
		i++
	}
	return r
}
