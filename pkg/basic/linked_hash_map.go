package basic

type LinkedHashMap[K comparable, T any] struct {
	itemMap map[K]*linkedHashMapNode[K, T]
	head    *linkedHashMapNode[K, T]
	tail    *linkedHashMapNode[K, T]
	zeroK   K
	zeroT   T
}

func NewLinkHashMap[K comparable, T any]() *LinkedHashMap[K, T] {
	return &LinkedHashMap[K, T]{
		itemMap: make(map[K]*linkedHashMapNode[K, T]),
	}
}

type linkedHashMapNode[K comparable, T any] struct {
	key   K
	value T
	prev  *linkedHashMapNode[K, T]
	next  *linkedHashMapNode[K, T]
}

func (m *LinkedHashMap[K, T]) Put(k K, v T) {
	var n *linkedHashMapNode[K, T]

	if _n, ok := m.itemMap[k]; !ok {
		n = &linkedHashMapNode[K, T]{
			key:   k,
			value: v,
		}
		m.itemMap[k] = n
	} else {
		n = _n
		n.value = v
	}

	m.movetoHead(n)
}

func (m *LinkedHashMap[K, T]) Get(k K) (T, bool) {
	if _n, ok := m.itemMap[k]; ok {
		m.movetoHead(_n)
		return _n.value, true
	} else {
		return m.zeroT, false
	}
}

func (m *LinkedHashMap[K, T]) Remove(k K) {
	if n, ok := m.itemMap[k]; ok {
		if n.prev != nil {
			n.prev.next = n.next
		}
		if n.next != nil {
			n.next.prev = n.prev
		}

		if m.head == n {
			m.head = n.next
		}
		if m.tail == n {
			m.tail = n.prev
		}
		delete(m.itemMap, k)
	}
}

func (m *LinkedHashMap[K, T]) RemoveFirst() K {
	if m.tail != nil {
		key := m.head.key
		m.Remove(key)
		return key
	} else {
		return m.zeroK
	}
}

func (m *LinkedHashMap[K, T]) RemoveLast() K {
	if m.tail != nil {
		key := m.tail.key
		m.Remove(key)
		return key
	} else {
		return m.zeroK
	}
}

func (m *LinkedHashMap[K, T]) Size() int {
	return len(m.itemMap)
}

func (m *LinkedHashMap[K, T]) Contains(key K) bool {
	_, ok := m.itemMap[key]
	return ok
}

func (m *LinkedHashMap[K, T]) Keys() []K {
	p := m.head
	var res []K
	for p != nil {
		res = append(res, p.key)
		p = p.next
	}
	return res
}

func (m *LinkedHashMap[K, T]) Values() []T {
	p := m.head
	var res []T
	for p != nil {
		res = append(res, p.value)
		p = p.next
	}
	return res
}

func (m *LinkedHashMap[K, T]) movetoHead(n *linkedHashMapNode[K, T]) {
	if m.head == n {
		return
	}

	if m.head == nil { // first node
		m.head = n
		m.tail = n
		return
	}

	// prev
	if n.prev != nil {
		n.prev.next = n.next
	}

	// next
	if n.next != nil {
		n.next.prev = n.prev
	}

	// last node
	if m.tail == n {
		m.tail = n.prev
	}

	m.head.prev = n
	n.next = m.head
	n.prev = nil
	m.head = n
}
