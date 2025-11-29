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

	"github.com/caiflower/common-tools/pkg/basic"
	"github.com/caiflower/common-tools/pkg/syncx"
)

type LFUCache[K comparable, V any] struct {
	itemMap  map[K]V
	freq     map[K]int
	freqMap  map[int]*basic.LinkedHashMap[K, interface{}]
	capacity int
	minUsed  int
	lock     sync.Locker
	zeroV    V
}

func NewLFUCache[K comparable, V any](capacity int) *LFUCache[K, V] {
	return &LFUCache[K, V]{
		capacity: capacity,
		freq:     make(map[K]int),
		freqMap:  make(map[int]*basic.LinkedHashMap[K, interface{}]),
		itemMap:  make(map[K]V),
		lock:     syncx.NewSpinLock(),
	}
}

func (c *LFUCache[K, V]) Put(key K, value V) {
	c.lock.Lock()
	defer c.lock.Unlock()

	if _, ok := c.itemMap[key]; !ok {
		// 先删了，再放
		if len(c.itemMap) == c.capacity {
			deleteKey := c.freqMap[c.minUsed].RemoveLast()
			delete(c.freq, deleteKey)
			delete(c.itemMap, deleteKey)
		}
		c.minUsed = 1
	}

	c.itemMap[key] = value
	c.increase(key)
}

func (c *LFUCache[K, V]) Get(key K) (V, bool) {
	c.lock.Lock()
	defer c.lock.Unlock()

	if v, ok := c.itemMap[key]; ok {
		c.increase(key)
		return v, true
	} else {
		return c.zeroV, false
	}
}

func (c *LFUCache[K, V]) increase(key K) {
	useCnt := c.freq[key]
	if useCnt != 0 {
		c.freqMap[useCnt].Remove(key)
		if useCnt == c.minUsed && c.freqMap[useCnt].Size() == 0 {
			delete(c.freqMap, useCnt)
			c.minUsed = useCnt + 1
		}
	}
	if c.freqMap[useCnt+1] == nil {
		c.freqMap[useCnt+1] = basic.NewLinkHashMap[K, interface{}]()
	}
	c.freq[key] = useCnt + 1
	c.freqMap[useCnt+1].Put(key, struct{}{})
	c.freqMap[useCnt+1].MoveToHead(key)
}
