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

 package cache

import (
	"sync"
)

type LRUCache struct {
	capacity int
	itemMap  map[string]*node
	lock     sync.RWMutex
	head     *node
	tail     *node
}

type node struct {
	key  string
	item interface{}
	prev *node
	next *node
}

func NewLRUCache(capacity int) *LRUCache {
	if capacity <= 0 {
		panic("invalid lru cache capacity")
	}
	return &LRUCache{
		capacity: capacity,
		itemMap:  make(map[string]*node),
		lock:     sync.RWMutex{},
	}
}

func (c *LRUCache) Get(key string) (interface{}, bool) {
	c.lock.RLock()
	if v, ok := c.itemMap[key]; ok {
		c.lock.RUnlock()

		c.lock.Lock()
		c.moveToHead(v)
		defer c.lock.Unlock()
		return v.item, ok
	} else {
		c.lock.RUnlock()
		return nil, false
	}
}

func (c *LRUCache) Put(key string, value interface{}) {
	var n *node
	c.lock.Lock()
	defer c.lock.Unlock()

	if v, ok := c.itemMap[key]; !ok {
		n = &node{
			key:  key,
			item: value,
		}
	} else {
		n = v
		n.item = value
	}

	c.moveToHead(n)
	c.itemMap[key] = n

	if len(c.itemMap) > c.capacity {
		c.removeTail()
	}

	return
}

func (c *LRUCache) removeTail() {
	key := c.tail.key
	if len(c.itemMap) == 1 {
		c.head = nil
		c.tail = nil
	} else {
		c.tail.prev.next = nil
		c.tail = c.tail.prev
	}
	delete(c.itemMap, key)
}

func (c *LRUCache) moveToHead(n *node) {
	if c.head == n {
		return
	}

	// node prev
	if n.prev != nil {
		n.prev.next = n.next
	}
	// node next
	if n.next != nil {
		n.next.prev = n.prev
	}
	// tail
	if n == c.tail {
		c.tail = n.prev
	}

	if c.head != nil {
		c.head.prev = n
		n.next = c.head
		c.head = n
		// n.prev = nil
		n.prev = nil
	} else {
		c.head = n
		c.tail = n
	}
}

func (c *LRUCache) Size() int {
	c.lock.RLock()
	defer c.lock.RUnlock()
	return len(c.itemMap)
}
