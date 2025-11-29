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

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLinkedHashMap_BasicOps(t *testing.T) {
	type putCase struct {
		key   string
		value interface{}
	}
	type getCase struct {
		key      string
		expected interface{}
		ok       bool
	}
	type removeCase struct {
		key      string
		expected bool
	}

	// 1. Put & Get
	m := NewLinkHashMap()
	putCases := []putCase{
		{"key1", "value1"},
		{"key2", "value2"},
		{"key3", "value3"},
	}
	for _, c := range putCases {
		m.Put(c.key, c.value)
	}
	getCases := []getCase{
		{"key1", "value1", true},
		{"key2", "value2", true},
		{"key3", "value3", true},
		{"keyX", nil, false},
	}
	for _, c := range getCases {
		val, ok := m.Get(c.key)
		assert.Equal(t, c.expected, val, c.key)
		assert.Equal(t, c.ok, ok, c.key)
	}

	// 2. Update value
	m.Put("key2", "new2")
	val, ok := m.Get("key2")
	assert.True(t, ok)
	assert.Equal(t, "new2", val)

	// 3. Remove keys
	removeCases := []removeCase{
		{"key3", true},
		{"key2", true},
		{"key1", true},
		{"keyX", false}, // 不存在
	}

	for _, c := range removeCases {
		prevSz := m.Size()
		m.Remove(c.key)
		_, ok := m.Get(c.key)
		assert.False(t, ok)
		if c.expected {
			assert.Equal(t, prevSz-1, m.Size())
		} else {
			assert.Equal(t, prevSz, m.Size())
		}
	}

	// 4. RemoveFirst/RemoveLast 空 map
	assert.Equal(t, "", m.RemoveFirst())
	assert.Equal(t, "", m.RemoveLast())
}

func TestLinkedHashMap_OrderAndSize(t *testing.T) {
	m := NewLinkHashMap()
	keys := []string{"a", "b", "c", "d"}
	for _, k := range keys {
		m.Put(k, k+"-val")
	}
	assert.Equal(t, 4, m.Size())

	// RemoveFirst
	first := m.RemoveFirst()
	assert.Equal(t, "a", first)
	assert.Equal(t, 3, m.Size())
	_, ok := m.Get("a")
	assert.False(t, ok)

	// RemoveLast
	last := m.RemoveLast()
	assert.Equal(t, "d", last)
	assert.Equal(t, 2, m.Size())
	_, ok = m.Get("d")
	assert.False(t, ok)
}

func TestLinkedHashMap_MoveToHead(t *testing.T) {
	m := NewLinkHashMap()
	keys := []string{"a", "b", "c", "d"}
	for _, k := range keys {
		m.Put(k, k+"-val")
	}
	assert.Equal(t, []string{"a", "b", "c", "d"}, m.Keys())
	assert.Equal(t, []interface{}{"a-val", "b-val", "c-val", "d-val"}, m.Values())

	// 移动中间节点到head
	m.MoveToHead("c")
	assert.Equal(t, []string{"c", "a", "b", "d"}, m.Keys())
	assert.Equal(t, 4, m.Size())

	// 移动尾节点到head
	m.MoveToHead("d")
	assert.Equal(t, []string{"d", "c", "a", "b"}, m.Keys())

	// 移动已在head的节点
	m.MoveToHead("d")
	assert.Equal(t, []string{"d", "c", "a", "b"}, m.Keys())

	// 移动不存在的key
	m.MoveToHead("x")
	assert.Equal(t, []string{"d", "c", "a", "b"}, m.Keys())

	// 多次移动
	m.MoveToHead("b")
	assert.Equal(t, []string{"b", "d", "c", "a"}, m.Keys())

	// 确保值没变
	assert.Equal(t, []interface{}{"b-val", "d-val", "c-val", "a-val"}, m.Values())
}
