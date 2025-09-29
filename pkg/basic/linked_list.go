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

type LinkedList[T comparable] struct {
	size  int
	zero  T
	first *linkedListNode[T]
	last  *linkedListNode[T]
}

type linkedListNode[T comparable] struct {
	item T
	prev *linkedListNode[T]
	next *linkedListNode[T]
}

func (l *LinkedList[T]) AddFirst(item T) {
	n := &linkedListNode[T]{
		item: item,
	}

	if l.first == nil {
		l.first = n
		l.last = n
	} else {
		n.next = l.first
		l.first.prev = n
		l.first = n
	}

	l.size++
}

func (l *LinkedList[T]) AddLast(item T) {
	n := &linkedListNode[T]{
		item: item,
	}

	if l.last == nil {
		l.last = n
		l.first = n
	} else {
		n.prev = l.last
		l.last.next = n
		l.last = n
	}

	l.size++
}

func (l *LinkedList[T]) RemoveFirst() (T, error) {
	if l.size == 0 {
		return l.zero, nilElement
	}

	res := l.first.item
	if l.first.next != nil {
		l.first = l.first.next
	} else {
		l.first = nil
		l.last = nil
	}
	l.size--

	return res, nil
}

func (l *LinkedList[T]) RemoveLast() (T, error) {
	if l.size == 0 {
		return l.zero, nilElement
	}

	res := l.last.item
	if l.last.prev != nil {
		l.last = l.last.prev
	} else {
		l.first = nil
		l.last = nil
	}
	l.size--

	return res, nil
}

func (l *LinkedList[T]) Size() int {
	return l.size
}

func (l *LinkedList[T]) Contains(item T) bool {
	return l.indexOf(item) != -1
}

func (l *LinkedList[T]) indexOf(item T) int {
	p := l.first
	index := 0
	for p != nil {
		if p.item == item {
			return index
		}
		p = p.next
	}
	return -1
}
