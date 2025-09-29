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
	"errors"
	"sync"
)

type SafeRingQueue struct {
	data     []interface{}
	capacity int
	head     int
	tail     int
	size     int
	mu       sync.Mutex
	notFull  *sync.Cond
	notEmpty *sync.Cond
}

func NewSafeRingQueue(cap int) *SafeRingQueue {
	q := &SafeRingQueue{
		data:     make([]interface{}, cap),
		capacity: cap,
		head:     0,
		tail:     0,
		size:     0,
	}

	q.notFull = sync.NewCond(&q.mu)
	q.notEmpty = sync.NewCond(&q.mu)
	return q
}

func (q *SafeRingQueue) Enqueue(val interface{}) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.size == q.capacity {
		return errors.New("queue is full")
	}
	q.data[q.tail] = val
	q.tail = (q.tail + 1) % q.capacity
	q.size++
	q.notEmpty.Signal() // 唤醒可能等待的 Dequeue
	return nil
}

func (q *SafeRingQueue) Dequeue() (interface{}, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.size == 0 {
		return nil, errors.New("queue is empty")
	}
	val := q.data[q.head]
	q.data[q.head] = nil
	q.head = (q.head + 1) % q.capacity
	q.size--
	q.notFull.Signal()
	return val, nil
}

func (q *SafeRingQueue) Peek() (interface{}, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.size == 0 {
		return nil, errors.New("queue is empty")
	}
	return q.data[q.head], nil
}

func (q *SafeRingQueue) BlockEnqueue(val interface{}) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for q.size == q.capacity {
		q.notFull.Wait()
	}
	q.data[q.tail] = val
	q.tail = (q.tail + 1) % q.capacity
	q.size++
	q.notEmpty.Signal() // 唤醒可能等待的 Dequeue
}

func (q *SafeRingQueue) BlockDequeue() interface{} {
	q.mu.Lock()
	defer q.mu.Unlock()
	for q.size == 0 {
		q.notEmpty.Wait()
	}
	val := q.data[q.head]
	q.data[q.head] = nil
	q.head = (q.head + 1) % q.capacity
	q.size--
	q.notFull.Signal() // 唤醒可能等待的 Enqueue
	return val
}

func (q *SafeRingQueue) Size() int {
	return q.size
}
