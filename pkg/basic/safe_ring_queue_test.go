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
	"sync"
	"sync/atomic"
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
		}
		fmt.Printf("%d\n", v)
	}
}

// TestPeek 验证 Peek 不移除元素
func TestPeek(t *testing.T) {
	q := NewSafeRingQueue(3)
	_ = q.Enqueue(42)

	val, err := q.Peek()
	if err != nil || val.(int) != 42 {
		t.Fatalf("Peek: expected 42, got %v, err=%v", val, err)
	}
	// Peek 后元素仍在队列中
	if q.Size() != 1 {
		t.Fatalf("Peek should not remove element, size=%d", q.Size())
	}
	val, err = q.Dequeue()
	if err != nil || val.(int) != 42 {
		t.Fatalf("Dequeue after Peek: expected 42, got %v", val)
	}
}

// TestPeek_Empty 空队列 Peek 应返回错误
func TestPeek_Empty(t *testing.T) {
	q := NewSafeRingQueue(2)
	_, err := q.Peek()
	if err == nil {
		t.Fatal("expected error when peeking empty queue, got nil")
	}
}

// TestFIFO_StrictOrder 单生产者单消费者，严格验证 FIFO 顺序
func TestFIFO_StrictOrder(t *testing.T) {
	const n = 1000000
	q := NewSafeRingQueue(500)

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < n; i++ {
			got := q.BlockDequeue().(int)
			if got != i {
				// 用 panic 而非 t.Fatal，因为在 goroutine 里
				panic(fmt.Sprintf("FIFO violated: expected %d, got %d", i, got))
			}
		}
	}()

	for i := 0; i < n; i++ {
		q.BlockEnqueue(i)
	}
	<-done
}

// TestMultiProducerConsumer_NoLossNoDuplicate 多生产者多消费者，验证无丢失无重复
func TestMultiProducerConsumer_NoLossNoDuplicate(t *testing.T) {
	const (
		producers   = 8
		consumers   = 8
		perProducer = 1000
		total       = producers * perProducer
	)
	q := NewSafeRingQueue(128)

	// 每个生产者发送唯一 ID 段：producer p 发送 [p*perProducer, (p+1)*perProducer)
	var prodWg sync.WaitGroup
	for p := 0; p < producers; p++ {
		prodWg.Add(1)
		base := p * perProducer
		go func(base int) {
			defer prodWg.Done()
			for i := 0; i < perProducer; i++ {
				q.BlockEnqueue(base + i)
			}
		}(base)
	}

	// 消费者收集所有值，用 atomic 计数避免 data race
	results := make(chan int, total)
	var recvCount int64
	var consWg sync.WaitGroup
	for c := 0; c < consumers; c++ {
		consWg.Add(1)
		go func() {
			defer consWg.Done()
			for {
				if atomic.LoadInt64(&recvCount) >= int64(total) {
					return
				}
				v, err := q.Dequeue()
				if err != nil {
					time.Sleep(time.Microsecond)
					continue
				}
				results <- v.(int)
				atomic.AddInt64(&recvCount, 1)
			}
		}()
	}

	prodWg.Wait()
	consWg.Wait()
	close(results)

	// 验证：每个值恰好出现一次
	seen := make(map[int]int, total)
	for v := range results {
		seen[v]++
	}
	if len(seen) != total {
		t.Fatalf("expected %d unique values, got %d", total, len(seen))
	}
	for v, count := range seen {
		if count != 1 {
			t.Fatalf("value %d appeared %d times (expected 1)", v, count)
		}
	}
}

// TestRingWrapAround_Stress 环形绕回压力测试：反复填满再清空，验证索引不越界
func TestRingWrapAround_Stress(t *testing.T) {
	const (
		cap    = 5
		rounds = 2000
	)
	q := NewSafeRingQueue(cap)

	for round := 0; round < rounds; round++ {
		// 填满
		for i := 0; i < cap; i++ {
			if err := q.Enqueue(round*cap + i); err != nil {
				t.Fatalf("round %d: Enqueue(%d) failed: %v", round, i, err)
			}
		}
		// 再入队应失败
		if err := q.Enqueue(-1); err == nil {
			t.Fatalf("round %d: expected full error", round)
		}
		// 清空并验证顺序
		for i := 0; i < cap; i++ {
			val, err := q.Dequeue()
			if err != nil {
				t.Fatalf("round %d: Dequeue failed: %v", round, err)
			}
			expected := round*cap + i
			if val.(int) != expected {
				t.Fatalf("round %d pos %d: expected %d, got %v", round, i, expected, val)
			}
		}
		// 清空后再出队应失败
		if _, err := q.Dequeue(); err == nil {
			t.Fatalf("round %d: expected empty error", round)
		}
		if q.Size() != 0 {
			t.Fatalf("round %d: expected size 0, got %d", round, q.Size())
		}
	}
}

// TestSizeConsistency 并发操作下 Size() 始终在 [0, capacity] 范围内
func TestSizeConsistency(t *testing.T) {
	const cap = 32
	q := NewSafeRingQueue(cap)

	var wg sync.WaitGroup
	stop := make(chan struct{})

	// 持续入队
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				_ = q.Enqueue(1)
			}
		}
	}()

	// 持续出队
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				_, _ = q.Dequeue()
			}
		}
	}()

	// 持续检查 Size() 合法性
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				s := q.Size()
				if s < 0 || s > cap {
					t.Errorf("Size() out of range: %d (capacity=%d)", s, cap)
					close(stop)
					return
				}
			}
		}
	}()

	time.Sleep(300 * time.Millisecond)
	close(stop)
	wg.Wait()
}

// TestPeekConsistency Peek 返回的值必须等于下一次 Dequeue 的值（单线程）
func TestPeekConsistency(t *testing.T) {
	q := NewSafeRingQueue(8)
	for i := 0; i < 8; i++ {
		_ = q.Enqueue(i * 10)
	}
	for q.Size() > 0 {
		peeked, err := q.Peek()
		if err != nil {
			t.Fatalf("Peek failed: %v", err)
		}
		dequeued, err := q.Dequeue()
		if err != nil {
			t.Fatalf("Dequeue failed: %v", err)
		}
		if peeked != dequeued {
			t.Fatalf("Peek=%v != Dequeue=%v", peeked, dequeued)
		}
	}
}

// TestBlockEnqueue_ManyWaiters 多个 goroutine 同时阻塞在 BlockEnqueue，出队后依次被唤醒
func TestBlockEnqueue_ManyWaiters(t *testing.T) {
	const (
		cap     = 2
		waiters = 10
	)
	q := NewSafeRingQueue(cap)
	// 先填满
	_ = q.Enqueue(0)
	_ = q.Enqueue(0)

	var wg sync.WaitGroup
	enqueued := make(chan int, waiters)
	for i := 1; i <= waiters; i++ {
		wg.Add(1)
		go func(val int) {
			defer wg.Done()
			q.BlockEnqueue(val)
			enqueued <- val
		}(i)
	}

	// 逐个出队，每次应唤醒一个等待者
	time.Sleep(50 * time.Millisecond) // 确保所有 goroutine 已阻塞
	for i := 0; i < waiters+cap; i++ {
		_, _ = q.Dequeue()
		time.Sleep(time.Millisecond)
	}

	wg.Wait()
	close(enqueued)

	if len(enqueued) != waiters {
		t.Fatalf("expected %d enqueued, got %d", waiters, len(enqueued))
	}
}

// TestHighConcurrency_DataIntegrity 高并发下数据完整性：收发总量严格匹配
func TestHighConcurrency_DataIntegrity(t *testing.T) {
	const (
		cap        = 256
		goroutines = 20
		perG       = 500
		total      = goroutines * perG
	)
	q := NewSafeRingQueue(cap)
	counter := make(chan struct{}, total)

	var wg sync.WaitGroup
	// 生产者
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < perG; j++ {
				q.BlockEnqueue(j)
			}
		}()
	}
	// 消费者
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < perG; j++ {
				q.BlockDequeue()
				counter <- struct{}{}
			}
		}()
	}

	wg.Wait()
	if len(counter) != total {
		t.Fatalf("expected %d consumed, got %d", total, len(counter))
	}
	if q.Size() != 0 {
		t.Fatalf("queue should be empty after all consumed, size=%d", q.Size())
	}
}

// TestSize_DataRace 验证 Size() 的并发安全性
// 使用 go test -race 运行时，当前无锁的 Size() 实现会触发数据竞争检测
func TestSize_DataRace(t *testing.T) {
	q := NewSafeRingQueue(1000)

	var wg sync.WaitGroup

	// 并发写
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 500; i++ {
			_ = q.Enqueue(i)
		}
	}()

	// 并发读 Size()
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 500; i++ {
			_ = q.Size()
		}
	}()

	wg.Wait()
}
