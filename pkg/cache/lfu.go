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

type LFUCache struct {
	itemMap  map[string]interface{}
	freq     map[string]int
	freqMap  map[int]*basic.LinkedHashMap
	capacity int
	minUsed  int
	lock     sync.Locker
}

func NewLFUCache(capacity int) *LFUCache {
	return &LFUCache{
		capacity: capacity,
		freq:     make(map[string]int),
		freqMap:  make(map[int]*basic.LinkedHashMap),
		itemMap:  make(map[string]interface{}),
		lock:     syncx.NewSpinLock(),
	}
}

func (c *LFUCache) Put(key string, value interface{}) {
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

func (c *LFUCache) Get(key string) (interface{}, bool) {
	c.lock.Lock()
	defer c.lock.Unlock()

	if v, ok := c.itemMap[key]; ok {
		c.increase(key)
		return v, true
	} else {
		return nil, false
	}
}

func (c *LFUCache) increase(key string) {
	useCnt := c.freq[key]
	if useCnt != 0 {
		c.freqMap[useCnt].Remove(key)
		if useCnt == c.minUsed && c.freqMap[useCnt].Size() == 0 {
			delete(c.freqMap, useCnt)
			c.minUsed = useCnt + 1
		}
	}
	if c.freqMap[useCnt+1] == nil {
		c.freqMap[useCnt+1] = basic.NewLinkHashMap()
	}
	c.freq[key] = useCnt + 1
	c.freqMap[useCnt+1].Put(key, struct{}{})
	c.freqMap[useCnt+1].MoveToHead(key)
}
