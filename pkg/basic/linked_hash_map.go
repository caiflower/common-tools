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

type LinkedHashMap struct {
	itemMap map[string]*linkedHashMapNode
	head    *linkedHashMapNode // 头结点，插入顺序第一个
	tail    *linkedHashMapNode // 尾结点，插入顺序最后一个
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
	if n, ok := m.itemMap[k]; ok {
		n.value = v
		return
	}
	n := &linkedHashMapNode{
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

func (m *LinkedHashMap) Get(k string) (interface{}, bool) {
	if n, ok := m.itemMap[k]; ok {
		return n.value, true
	}
	return nil, false
}

func (m *LinkedHashMap) Remove(k string) {
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

func (m *LinkedHashMap) RemoveFirst() string {
	if m.head == nil {
		return ""
	}
	key := m.head.key
	m.Remove(key)
	return key
}

func (m *LinkedHashMap) RemoveLast() string {
	if m.tail == nil {
		return ""
	}
	key := m.tail.key
	m.Remove(key)
	return key
}

func (m *LinkedHashMap) Size() int {
	return len(m.itemMap)
}

func (m *LinkedHashMap) Contains(key string) bool {
	_, ok := m.itemMap[key]
	return ok
}

func (m *LinkedHashMap) Keys() []string {
	var res []string
	for p := m.head; p != nil; p = p.next {
		res = append(res, p.key)
	}
	return res
}

func (m *LinkedHashMap) Values() []interface{} {
	var res []interface{}
	for p := m.head; p != nil; p = p.next {
		res = append(res, p.value)
	}
	return res
}

func (m *LinkedHashMap) MoveToHead(key string) {
	node, ok := m.itemMap[key]
	if !ok || node == m.head {
		return
	}
	m.moveToHead(node)
}

func (m *LinkedHashMap) moveToHead(node *linkedHashMapNode) {
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
