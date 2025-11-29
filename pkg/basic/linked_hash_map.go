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
	if n, ok := m.itemMap[k]; ok {
		n.value = v
		return
	}
	n := &linkedHashMapNode[K, T]{
		key:   k,
		value: v,
	}
	m.itemMap[k] = n
	if m.tail == nil {
		m.head = n
		m.tail = n
	} else {
		m.tail.next = n
		n.prev = m.tail
		m.tail = n
	}
}

func (m *LinkedHashMap[K, T]) Get(k K) (T, bool) {
	if _n, ok := m.itemMap[k]; ok {
		return _n.value, true
	} else {
		return m.zeroT, false
	}
}

func (m *LinkedHashMap[K, T]) Remove(k K) {
	n, ok := m.itemMap[k]
	if !ok {
		return
	}

	if n.prev != nil {
		n.prev.next = n.next
	} else {
		m.head = n.next
	}
	if n.next != nil {
		n.next.prev = n.prev
	} else {
		m.tail = n.prev
	}
	delete(m.itemMap, k)
}

func (m *LinkedHashMap[K, T]) RemoveFirst() K {
	if m.head == nil {
		return m.zeroK
	}
	key := m.head.key
	m.Remove(key)
	return key
}

func (m *LinkedHashMap[K, T]) RemoveLast() K {
	if m.tail == nil {
		return m.zeroK
	}
	key := m.tail.key
	m.Remove(key)
	return key
}

func (m *LinkedHashMap[K, T]) Size() int {
	return len(m.itemMap)
}

func (m *LinkedHashMap[K, T]) Contains(key K) bool {
	_, ok := m.itemMap[key]
	return ok
}

func (m *LinkedHashMap[K, T]) Keys() []K {
	var res []K
	for p := m.head; p != nil; p = p.next {
		res = append(res, p.key)
	}
	return res
}

func (m *LinkedHashMap[K, T]) Values() []T {
	var res []T
	for p := m.head; p != nil; p = p.next {
		res = append(res, p.value)
	}
	return res
}

func (m *LinkedHashMap[K,T]) MoveToHead(key K) {
	node, ok := m.itemMap[key]
	if !ok || node == m.head {
		return
	}
	m.moveToHead(node)
}

func (m *LinkedHashMap[K,T]) moveToHead(node *linkedHashMapNode[K,T]) {
	// 断链
	if node.prev != nil {
		node.prev.next = node.next
	} else {
		m.head = node.next
	}
	if node.next != nil {
		node.next.prev = node.prev
	} else {
		m.tail = node.prev
	}
	// 插入到head
	node.prev = nil
	node.next = m.head
	if m.head != nil {
		m.head.prev = node
	}
	m.head = node
	if m.tail == nil {
		m.tail = node
	}
}
