package basic

type LinkedHashMap struct {
	itemMap map[string]*linkedHashMapNode
	head    *linkedHashMapNode
	tail    *linkedHashMapNode
}

func NewLinkHashMap() *LinkedHashMap {
	return &LinkedHashMap{
		itemMap: make(map[string]*linkedHashMapNode),
	}
}

type linkedHashMapNode struct {
	key   string
	value interface{}
	prev  *linkedHashMapNode
	next  *linkedHashMapNode
}

func (m *LinkedHashMap) Put(k string, v interface{}) {
	var n *linkedHashMapNode

	if _n, ok := m.itemMap[k]; !ok {
		n = &linkedHashMapNode{
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

func (m *LinkedHashMap) Get(k string) (interface{}, bool) {
	if _n, ok := m.itemMap[k]; ok {
		m.movetoHead(_n)
		return _n.value, true
	} else {
		return nil, false
	}
}

func (m *LinkedHashMap) Remove(k string) {
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

func (m *LinkedHashMap) RemoveFirst() string {
	if m.tail != nil {
		key := m.head.key
		m.Remove(key)
		return key
	} else {
		return ""
	}
}

func (m *LinkedHashMap) RemoveLast() string {
	if m.tail != nil {
		key := m.tail.key
		m.Remove(key)
		return key
	} else {
		return ""
	}
}

func (m *LinkedHashMap) Size() int {
	return len(m.itemMap)
}

func (m *LinkedHashMap) Contains(key string) bool {
	_, ok := m.itemMap[key]
	return ok
}

func (m *LinkedHashMap) Keys() []string {
	p := m.head
	var res []string
	for p != nil {
		res = append(res, p.key)
		p = p.next
	}
	return res
}

func (m *LinkedHashMap) Values() []interface{} {
	p := m.head
	var res []interface{}
	for p != nil {
		res = append(res, p.value)
		p = p.next
	}
	return res
}

func (m *LinkedHashMap) movetoHead(n *linkedHashMapNode) {
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
