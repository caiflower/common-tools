package cache

import (
	"sync"
)

type LRUCache[K comparable, T any] struct {
	capacity int
	itemMap  map[K]*node[K, T]
	lock     sync.RWMutex
	head     *node[K, T]
	tail     *node[K, T]
	zero     T
}

type node[K comparable, T any] struct {
	key  K
	item T
	prev *node[K, T]
	next *node[K, T]
}

func NewLRUCache[K comparable, T any](capacity int) *LRUCache[K, T] {
	if capacity <= 0 {
		panic("invalid lru cache capacity")
	}
	return &LRUCache[K, T]{
		capacity: capacity,
		itemMap:  make(map[K]*node[K, T]),
		lock:     sync.RWMutex{},
	}
}

func (c *LRUCache[K, T]) Get(key K) (T, bool) {
	c.lock.RLock()
	if v, ok := c.itemMap[key]; ok {
		c.lock.RUnlock()

		c.lock.Lock()
		c.moveToHead(v)
		defer c.lock.Unlock()
		return v.item, ok
	} else {
		c.lock.RUnlock()
		return c.zero, false
	}
}

func (c *LRUCache[K, T]) Put(key K, value T) {
	var n *node[K, T]
	c.lock.Lock()
	defer c.lock.Unlock()

	if v, ok := c.itemMap[key]; !ok {
		n = &node[K, T]{
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

func (c *LRUCache[K, T]) removeTail() {
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

func (c *LRUCache[K, T]) moveToHead(n *node[K, T]) {
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

func (c *LRUCache[K, T]) Size() int {
	c.lock.RLock()
	defer c.lock.RUnlock()
	return len(c.itemMap)
}
