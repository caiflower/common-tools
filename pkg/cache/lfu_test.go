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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLFUCache_BasicOps(t *testing.T) {
	c := NewLFUCache[string, interface{}](3)
	c.Put("a", 1)
	c.Put("b", 2)
	c.Put("c", 3)

	v, ok := c.Get("a")
	assert.True(t, ok)
	assert.Equal(t, 1, v)

	v, ok = c.Get("b")
	assert.True(t, ok)
	assert.Equal(t, 2, v)

	v, ok = c.Get("c")
	assert.True(t, ok)
	assert.Equal(t, 3, v)

	// 覆盖写入
	c.Put("a", 100)
	v, ok = c.Get("a")
	assert.True(t, ok)
	assert.Equal(t, 100, v)
}

func TestLFUCache_EvictLowFreq(t *testing.T) {
	c := NewLFUCache[string, interface{}](2)
	c.Put("x", 1)
	c.Put("y", 2)
	// 访问x多次提升频率
	c.Get("x")
	c.Get("x")
	// 插入新key，应该淘汰y（低频）
	c.Put("z", 3)

	v, ok := c.Get("x")
	assert.True(t, ok)
	assert.Equal(t, 1, v)

	v, ok = c.Get("y")
	assert.False(t, ok)
	assert.Nil(t, v)

	v, ok = c.Get("z")
	assert.True(t, ok)
	assert.Equal(t, 3, v)
}

func TestLFUCache_EvictTie(t *testing.T) {
	c := NewLFUCache[string, interface{}](2)
	c.Put("a", 1)
	c.Put("b", 2)
	// 两者频率一样，淘汰顺序应按插入顺序（a先淘汰）
	c.Put("c", 3)

	v, ok := c.Get("a")
	assert.False(t, ok)
	assert.Nil(t, v)

	v, ok = c.Get("b")
	assert.True(t, ok)
	assert.Equal(t, 2, v)

	v, ok = c.Get("c")
	assert.True(t, ok)
	assert.Equal(t, 3, v)
}

func TestLFUCache_EdgeCase(t *testing.T) {
	c := NewLFUCache[string, interface{}](1)
	c.Put("only", 42)
	v, ok := c.Get("only")
	assert.True(t, ok)
	assert.Equal(t, 42, v)

	c.Put("new", 99) // 应淘汰 "only"
	v, ok = c.Get("only")
	assert.False(t, ok)
	assert.Nil(t, v)
	v, ok = c.Get("new")
	assert.True(t, ok)
	assert.Equal(t, 99, v)
}
