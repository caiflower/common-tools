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
	"fmt"
	"testing"
	"time"
)

func TestRingQueueBasic(t *testing.T) {
	q := newBlockingQueue(3)
	// 入队
	if err := q.Enqueue(1); err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}
	if err := q.Enqueue(2); err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}
	if err := q.Enqueue(3); err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}
	if err := q.Enqueue(4); err == nil {
		t.Fatalf("Enqueue should fail when queue is full")
	}
	if q.Size() != 3 {
		t.Fatalf("Enqueue size should be 3, but got %d", q.Size())
	}

	// 出队
	v, err := q.Dequeue()
	if err != nil || v != 1 {
		t.Fatalf("Dequeue failed: %v, got: %v", err, v)
	}
	if q.Size() != 2 {
		t.Fatalf("Enqueue size should be 2, but got %d", q.Size())
	}
	v, err = q.Dequeue()
	if err != nil || v != 2 {
		t.Fatalf("Dequeue failed: %v, got: %v", err, v)
	}
	if q.Size() != 1 {
		t.Fatalf("Enqueue size should be 1, but got %d", q.Size())
	}

	// 再入队
	if err := q.Enqueue(4); err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}
	if q.Size() != 2 {
		t.Fatalf("Enqueue size should be 2, but got %d", q.Size())
	}

	v, err = q.Dequeue()
	if err != nil || v != 3 {
		t.Fatalf("Dequeue failed: %v, got: %v", err, v)
	}
	if q.Size() != 1 {
		t.Fatalf("Enqueue size should be 1, but got %d", q.Size())
	}
	v, err = q.Dequeue()
	if err != nil || v != 4 {
		t.Fatalf("Dequeue failed: %v, got: %v", err, v)
	}
	if q.Size() != 0 {
		t.Fatalf("Enqueue size should be 0, but got %d", q.Size())
	}

	// 空队列出队
	_, err = q.Dequeue()
	if err == nil {
		t.Fatalf("Dequeue should fail when queue is empty")
	}
}

// 初始化条件变量
func newBlockingQueue(cap int) *SafeRingQueue {
	q := NewSafeRingQueue(cap)
	return q
}

func TestBlockEnqueueDequeue(t *testing.T) {
	q := newBlockingQueue(1)
	done := make(chan struct{})

	// 先填满队列
	q.BlockEnqueue("A")

	go func() {
		// 这个 BlockEnqueue 会阻塞，直到主 goroutine消费掉一个
		q.BlockEnqueue("B")
		close(done)
	}()

	time.Sleep(100 * time.Millisecond) // 保证协程已阻塞

	select {
	case <-done:
		t.Fatalf("BlockEnqueue should block when queue is full")
	default:
	}

	// 消费一个，唤醒 BlockEnqueue
	v := q.BlockDequeue()
	if v != "A" {
		t.Fatalf("BlockDequeue got: %v, want: A", v)
	}

	// 等待入队完成
	<-done
	v = q.BlockDequeue()
	if v != "B" {
		t.Fatalf("BlockDequeue got: %v, want: B", v)
	}
}

func TestBlockDequeueBlocks(t *testing.T) {
	q := newBlockingQueue(1)
	done := make(chan struct{})

	go func() {
		v := q.BlockDequeue()
		if v != "hello" {
			t.Errorf("BlockDequeue got: %v, want: hello", v)
		}
		close(done)
	}()

	time.Sleep(100 * time.Millisecond) // 保证协程已阻塞

	select {
	case <-done:
		t.Fatalf("BlockDequeue should block when queue is empty")
	default:
	}

	// 入队，唤醒 BlockDequeue
	q.BlockEnqueue("hello")
	<-done
}

func TestConcurrentBlockEnqueueDequeue(t *testing.T) {
	q := newBlockingQueue(2)
	produced := make(chan int, 10)
	consumed := make(chan int, 10)

	go func() {
		for i := 1; i <= 5; i++ {
			q.BlockEnqueue(i)
			produced <- i
		}
		close(produced)
	}()

	go func() {
		for i := 1; i <= 5; i++ {
			v := q.BlockDequeue().(int)
			consumed <- v
		}
		close(consumed)
	}()

	// 检查顺序
	for v := range produced {
		cv := <-consumed
		if v != cv {
			t.Fatalf("Produced %d, consumed %d", v, cv)
		} else {
			fmt.Printf("%d\n", v)
		}
	}
}
