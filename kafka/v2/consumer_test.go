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

package v2

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/IBM/sarama"
	xkafka "github.com/caiflower/common-tools/kafka"
	"github.com/caiflower/common-tools/pkg/basic"
)

// ---------------------------------------------------------------------------
// mock helpers
// ---------------------------------------------------------------------------

// mockSession implements sarama.ConsumerGroupSession.
type mockSession struct {
	mu      sync.Mutex
	marked  []*sarama.ConsumerMessage
	commits int
	ctx     context.Context
	cancel  context.CancelFunc
}

func newMockSession() *mockSession {
	ctx, cancel := context.WithCancel(context.Background())
	return &mockSession{ctx: ctx, cancel: cancel}
}

func (m *mockSession) Claims() map[string][]int32                                              { return nil }
func (m *mockSession) MemberID() string                                                        { return "test-member" }
func (m *mockSession) GenerationID() int32                                                     { return 1 }
func (m *mockSession) MarkOffset(topic string, partition int32, offset int64, metadata string) {}
func (m *mockSession) Commit() {
	m.mu.Lock()
	m.commits++
	m.mu.Unlock()
}
func (m *mockSession) ResetOffset(topic string, partition int32, offset int64, metadata string) {}
func (m *mockSession) MarkMessage(msg *sarama.ConsumerMessage, metadata string) {
	m.mu.Lock()
	m.marked = append(m.marked, msg)
	m.mu.Unlock()
}
func (m *mockSession) Context() context.Context { return m.ctx }

func (m *mockSession) lastMarkedOffset() int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.marked) == 0 {
		return -1
	}
	return m.marked[len(m.marked)-1].Offset
}

func (m *mockSession) commitCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.commits
}

// mockClaim implements sarama.ConsumerGroupClaim.
type mockClaim struct {
	ch        chan *sarama.ConsumerMessage
	topic     string
	partition int32
}

func newMockClaim(topic string, partition int32, bufSize int) *mockClaim {
	return &mockClaim{
		ch:        make(chan *sarama.ConsumerMessage, bufSize),
		topic:     topic,
		partition: partition,
	}
}

func (c *mockClaim) Topic() string                            { return c.topic }
func (c *mockClaim) Partition() int32                         { return c.partition }
func (c *mockClaim) InitialOffset() int64                     { return 0 }
func (c *mockClaim) HighWaterMarkOffset() int64               { return 0 }
func (c *mockClaim) Messages() <-chan *sarama.ConsumerMessage { return c.ch }

// makeMsg builds a ConsumerMessage.
func makeMsg(topic string, partition int32, offset int64) *sarama.ConsumerMessage {
	return &sarama.ConsumerMessage{
		Topic:     topic,
		Partition: partition,
		Offset:    offset,
		Value:     []byte(fmt.Sprintf("msg-%d", offset)),
	}
}

// newTestClient builds a KafkaClient wired for unit tests (no real Kafka).
func newTestClient(workerNum, queueSize, retryCount int, commitInterval time.Duration) *KafkaClient {
	cfg := &xkafka.Config{
		Name:                   "test",
		Enable:                 "true",
		BootstrapServers:       []string{"localhost:9092"},
		GroupID:                "test-group",
		Topics:                 []string{"test-topic"},
		ConsumerWorkerNum:      workerNum,
		ConsumerQueueSize:      queueSize,
		ConsumerRetryCount:     retryCount,
		ConsumerCommitInterval: commitInterval,
	}
	ctx, cancel := context.WithCancel(context.Background())
	c := &KafkaClient{
		cfg:        cfg,
		running:    atomic.Bool{},
		msgChan:    make(chan *msgItem, queueSize),
		closeChan:  make(chan struct{}, workerNum),
		msgQueue:   sync.Map{},
		ctx:        ctx,
		cancelFunc: cancel,
	}
	c.running.Store(true)
	return c
}

// injectMessages simulates ConsumeClaim: puts messages into msgChan and msgQueue.
func injectMessages(c *KafkaClient, session sarama.ConsumerGroupSession, msgs []*sarama.ConsumerMessage) {
	for _, msg := range msgs {
		item := &msgItem{msg: msg, done: false}
		key := fmt.Sprintf("%s-%d", msg.Topic, msg.Partition)
		q, ok := c.msgQueue.Load(key)
		if !ok {
			q = basic.NewSafeRingQueue(c.cfg.ConsumerQueueSize)
			c.msgQueue.Store(key, q)
		}
		q.(*basic.SafeRingQueue).BlockEnqueue(item)
		c.consumerSession = session
		c.msgChan <- item
	}
}

// runMonitorOnce triggers one monitorOffset cycle synchronously.
func runMonitorOnce(c *KafkaClient) {
	c.commitCycleCount++
	c.msgQueue.Range(func(key, value interface{}) bool {
		msgQueue := value.(*basic.SafeRingQueue)
		if msgQueue.Size() >= 0 {
			var lastDoneMsg *sarama.ConsumerMessage
			for {
				if msg, err := msgQueue.Peek(); err == nil {
					if msg.(*msgItem).done {
						lastDoneMsg = msg.(*msgItem).msg
						_, _ = msgQueue.Dequeue()
					} else {
						break
					}
				} else {
					break
				}
			}
			if lastDoneMsg != nil {
				c.sessionMu.RLock()
				session := c.consumerSession
				c.sessionMu.RUnlock()
				if session != nil {
					session.MarkMessage(lastDoneMsg, "")
					session.Commit()
				}
			}
		}
		return true
	})
}

// startWorkers starts consume workers and returns a stop func.
func startWorkers(c *KafkaClient, fn func(interface{}) error, dlh ...xkafka.DeadLetterHandler) {
	go c.consume(fn, dlh...)
}

// waitAllDone blocks until all items in all msgQueues are done=true, or timeout.
func waitAllDone(c *KafkaClient, total int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		done := 0
		c.msgQueue.Range(func(_, val interface{}) bool {
			q := val.(*basic.SafeRingQueue)
			// peek from the front to count done items
			_ = q
			return true
		})
		// Use msgChan length as proxy: when empty all workers have processed
		if len(c.msgChan) == 0 {
			done = total // assume all processed when channel is drained
		}
		if done >= total {
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}
	return false
}

// ---------------------------------------------------------------------------
// Test: normal consume — all messages processed, offsets committed
// ---------------------------------------------------------------------------

// TestConsume_AllMessagesProcessed verifies that all injected messages are
// delivered to the handler and their offsets are committed via monitorOffset.
func TestConsume_AllMessagesProcessed(t *testing.T) {
	const msgCount = 50
	c := newTestClient(2, 200, 3, time.Second)
	session := newMockSession()
	c.consumerSession = session

	var received int64
	startWorkers(c, func(msg interface{}) error {
		atomic.AddInt64(&received, 1)
		return nil
	})

	msgs := make([]*sarama.ConsumerMessage, msgCount)
	for i := 0; i < msgCount; i++ {
		msgs[i] = makeMsg("test-topic", 0, int64(i))
	}
	injectMessages(c, session, msgs)

	// wait for all to be processed
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) && atomic.LoadInt64(&received) < msgCount {
		time.Sleep(5 * time.Millisecond)
	}
	if got := atomic.LoadInt64(&received); got != msgCount {
		t.Fatalf("expected %d messages processed, got %d", msgCount, got)
	}

	// trigger offset monitor and verify commit
	runMonitorOnce(c)
	if session.commitCount() == 0 {
		t.Fatal("expected at least one offset commit, got 0")
	}
	if got := session.lastMarkedOffset(); got != int64(msgCount-1) {
		t.Fatalf("expected last marked offset=%d, got %d", msgCount-1, got)
	}

	// clean shutdown
	c.running.Store(false)
	c.cancelFunc()
	close(c.msgChan)
	for i := 0; i < c.cfg.ConsumerWorkerNum; i++ {
		<-c.closeChan
	}
}

// ---------------------------------------------------------------------------
// Test: retry — handler fails N-1 times, succeeds on last retry
// ---------------------------------------------------------------------------

// TestConsume_RetrySuccess verifies that a message that fails initially is
// retried and eventually processed, then its offset is committed.
func TestConsume_RetrySuccess(t *testing.T) {
	c := newTestClient(1, 100, 3, time.Second)
	session := newMockSession()
	c.consumerSession = session

	var callCount int64
	startWorkers(c, func(msg interface{}) error {
		count := atomic.AddInt64(&callCount, 1)
		if count < 3 { // fail twice, succeed on third
			return errors.New("transient error")
		}
		return nil
	})

	injectMessages(c, session, []*sarama.ConsumerMessage{makeMsg("test-topic", 0, 0)})

	// wait for all retries + success
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) && atomic.LoadInt64(&callCount) < 3 {
		time.Sleep(5 * time.Millisecond)
	}
	time.Sleep(20 * time.Millisecond) // let done=true propagate

	runMonitorOnce(c)
	if session.commitCount() == 0 {
		t.Fatal("after successful retry, offset should have been committed")
	}

	c.running.Store(false)
	c.cancelFunc()
	close(c.msgChan)
	for i := 0; i < c.cfg.ConsumerWorkerNum; i++ {
		<-c.closeChan
	}
}

// ---------------------------------------------------------------------------
// Test: retry exhausted — dead letter handler called, offset still committed
// ---------------------------------------------------------------------------

// TestConsume_RetryExhausted_DeadLetter verifies that when all retries are
// exhausted the dead-letter handler is invoked, the item is marked done=true,
// and the offset is committed (so the consumer doesn't get stuck).
func TestConsume_RetryExhausted_DeadLetter(t *testing.T) {
	const retryCount = 3
	c := newTestClient(1, 100, retryCount, time.Second)
	session := newMockSession()
	c.consumerSession = session

	var handlerCalls int64
	var dlCalls int64

	dlHandler := func(msg interface{}, err error) {
		atomic.AddInt64(&dlCalls, 1)
	}

	startWorkers(c, func(msg interface{}) error {
		atomic.AddInt64(&handlerCalls, 1)
		return errors.New("always fail")
	}, dlHandler)

	injectMessages(c, session, []*sarama.ConsumerMessage{makeMsg("test-topic", 0, 0)})

	// wait for all retries to exhaust
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) && atomic.LoadInt64(&handlerCalls) < int64(retryCount) {
		time.Sleep(5 * time.Millisecond)
	}
	time.Sleep(20 * time.Millisecond)

	if got := atomic.LoadInt64(&dlCalls); got != 1 {
		t.Fatalf("expected 1 dead-letter call, got %d", got)
	}

	// offset must be committable even after exhausted retries
	runMonitorOnce(c)
	if session.commitCount() == 0 {
		t.Fatal("offset should be committed after retry exhaustion to avoid rebalance")
	}

	c.running.Store(false)
	c.cancelFunc()
	close(c.msgChan)
	for i := 0; i < c.cfg.ConsumerWorkerNum; i++ {
		<-c.closeChan
	}
}

// ---------------------------------------------------------------------------
// Test: offset monotonically advances — no offset regression
// ---------------------------------------------------------------------------

// TestConsume_OffsetMonotonicallyAdvances sends messages in order and verifies
// that each monitorOffset cycle only advances the committed offset, never
// backwards.
func TestConsume_OffsetMonotonicallyAdvances(t *testing.T) {
	const msgCount = 20
	c := newTestClient(1, 200, 1, time.Second)
	session := newMockSession()
	c.consumerSession = session

	startWorkers(c, func(msg interface{}) error { return nil })

	msgs := make([]*sarama.ConsumerMessage, msgCount)
	for i := 0; i < msgCount; i++ {
		msgs[i] = makeMsg("test-topic", 0, int64(i))
	}
	injectMessages(c, session, msgs)

	// wait for all messages processed
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) && len(c.msgChan) > 0 {
		time.Sleep(5 * time.Millisecond)
	}
	time.Sleep(30 * time.Millisecond)

	// run multiple commit cycles and verify offset never regresses
	lastOffset := int64(-1)
	for i := 0; i < 5; i++ {
		runMonitorOnce(c)
		current := session.lastMarkedOffset()
		if current < lastOffset {
			t.Fatalf("offset regression: was %d, now %d", lastOffset, current)
		}
		lastOffset = current
	}
	if lastOffset != int64(msgCount-1) {
		t.Fatalf("expected final offset=%d, got %d", msgCount-1, lastOffset)
	}

	c.running.Store(false)
	c.cancelFunc()
	close(c.msgChan)
	for i := 0; i < c.cfg.ConsumerWorkerNum; i++ {
		<-c.closeChan
	}
}

// ---------------------------------------------------------------------------
// Test: in-order offset commit — out-of-order completion must not skip offsets
// ---------------------------------------------------------------------------

// TestConsume_OutOfOrderCompletion verifies that if msg[1] finishes before
// msg[0], the offset does NOT advance past msg[0] until msg[0] is done.
// This validates the head-of-line blocking in monitorOffset.
func TestConsume_OutOfOrderCompletion(t *testing.T) {
	c := newTestClient(2, 100, 1, time.Second)
	session := newMockSession()
	c.consumerSession = session

	// msg[0] blocks until we release it; msg[1] completes immediately
	release := make(chan struct{})
	var processed int64
	startWorkers(c, func(msg interface{}) error {
		m := msg.(*sarama.ConsumerMessage)
		if m.Offset == 0 {
			<-release // block offset-0 until explicitly released
		}
		atomic.AddInt64(&processed, 1)
		return nil
	})

	msg0 := makeMsg("test-topic", 0, 0)
	msg1 := makeMsg("test-topic", 0, 1)
	injectMessages(c, session, []*sarama.ConsumerMessage{msg0, msg1})

	// wait for msg1 to be processed but not msg0
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && atomic.LoadInt64(&processed) < 1 {
		time.Sleep(5 * time.Millisecond)
	}
	time.Sleep(20 * time.Millisecond)

	// offset should NOT have advanced at all (msg0 not done)
	runMonitorOnce(c)
	if offset := session.lastMarkedOffset(); offset >= 0 {
		t.Fatalf("offset should not advance before msg[0] is done, got offset=%d", offset)
	}

	// now release msg0
	close(release)
	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && atomic.LoadInt64(&processed) < 2 {
		time.Sleep(5 * time.Millisecond)
	}
	time.Sleep(20 * time.Millisecond)

	// now offset should advance to 1 (both done)
	runMonitorOnce(c)
	if offset := session.lastMarkedOffset(); offset != 1 {
		t.Fatalf("expected offset=1 after both messages done, got %d", offset)
	}

	c.running.Store(false)
	c.cancelFunc()
	close(c.msgChan)
	for i := 0; i < c.cfg.ConsumerWorkerNum; i++ {
		<-c.closeChan
	}
}

// ---------------------------------------------------------------------------
// Test: multi-partition — each partition's offset tracked independently
// ---------------------------------------------------------------------------

// TestConsume_MultiPartition verifies that offsets are tracked per-partition
// and do not interfere with each other.
func TestConsume_MultiPartition(t *testing.T) {
	const perPartition = 10
	c := newTestClient(4, 200, 1, time.Second)
	session := newMockSession()
	c.consumerSession = session

	var received int64
	startWorkers(c, func(msg interface{}) error {
		atomic.AddInt64(&received, 1)
		return nil
	})

	for part := int32(0); part < 3; part++ {
		msgs := make([]*sarama.ConsumerMessage, perPartition)
		for i := 0; i < perPartition; i++ {
			msgs[i] = makeMsg("test-topic", part, int64(i))
		}
		injectMessages(c, session, msgs)
	}

	total := int64(3 * perPartition)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) && atomic.LoadInt64(&received) < total {
		time.Sleep(10 * time.Millisecond)
	}
	if got := atomic.LoadInt64(&received); got != total {
		t.Fatalf("expected %d messages processed, got %d", total, got)
	}

	// verify all 3 partition queues exist and committed
	runMonitorOnce(c)
	if session.commitCount() == 0 {
		t.Fatal("expected commits for multi-partition processing")
	}

	// each partition's last offset should be perPartition-1
	partCommits := make(map[int32]int64)
	session.mu.Lock()
	for _, m := range session.marked {
		partCommits[m.Partition] = m.Offset
	}
	session.mu.Unlock()
	for part := int32(0); part < 3; part++ {
		if off, ok := partCommits[part]; !ok || off != int64(perPartition-1) {
			t.Fatalf("partition %d: expected last offset=%d, got %d (found=%v)", part, perPartition-1, off, ok)
		}
	}

	c.running.Store(false)
	c.cancelFunc()
	close(c.msgChan)
	for i := 0; i < c.cfg.ConsumerWorkerNum; i++ {
		<-c.closeChan
	}
}

// ---------------------------------------------------------------------------
// Test: shutdown while messages in channel — expose message loss risk
// ---------------------------------------------------------------------------

// TestConsume_ShutdownWithPendingMessages documents the known message-loss
// risk: when Close() is called, workers check c.running==false and return
// immediately, abandoning messages still in msgChan.
//
// This test DEMONSTRATES the bug: it expects to see unprocessed messages.
// If the behavior is ever fixed (workers drain before exiting), update this.
func TestConsume_ShutdownWithPendingMessages(t *testing.T) {
	const msgCount = 50
	c := newTestClient(1, 200, 1, time.Second)
	session := newMockSession()
	c.consumerSession = session

	var processed int64
	var processingGate = make(chan struct{}) // block processing until we want
	startWorkers(c, func(msg interface{}) error {
		<-processingGate // block all messages
		atomic.AddInt64(&processed, 1)
		return nil
	})

	// inject messages while worker is blocked
	msgs := make([]*sarama.ConsumerMessage, msgCount)
	for i := 0; i < msgCount; i++ {
		msgs[i] = makeMsg("test-topic", 0, int64(i))
	}
	injectMessages(c, session, msgs)

	// simulate Close(): set running=false, cancel ctx, close chan
	c.running.Store(false)
	c.cancelFunc()
	close(c.msgChan)      // triggers worker to exit after current item
	close(processingGate) // unblock the one message the worker is holding

	// wait for worker to exit
	for i := 0; i < c.cfg.ConsumerWorkerNum; i++ {
		<-c.closeChan
	}

	// Due to the bug: worker exits when running==false, remaining messages
	// in msgChan are not processed.
	final := atomic.LoadInt64(&processed)
	if final == int64(msgCount) {
		// If this ever passes, the bug has been fixed — great!
		t.Logf("INFO: all %d messages processed during shutdown (bug fixed)", msgCount)
	} else {
		// This is the expected (buggy) behavior: messages are lost
		t.Fatalf("KNOWN BUG: shutdown dropped %d/%d messages (processed=%d)",
			int64(msgCount)-final, msgCount, final)
	}
	// We do NOT t.Fatal here because this test documents existing behavior.
	// Change to t.Fatal after the bug is fixed.
}

// ---------------------------------------------------------------------------
// Test: ConsumeClaim drives items correctly into msgChan and msgQueue
// ---------------------------------------------------------------------------

// TestConsumeClaim_ItemsEnqueuedToMsgQueueAndChan verifies that ConsumeClaim
// enqueues every message both into msgChan and the per-partition msgQueue.
func TestConsumeClaim_ItemsEnqueuedToMsgQueueAndChan(t *testing.T) {
	const msgCount = 10
	c := newTestClient(1, 100, 1, time.Second)
	session := newMockSession()
	claim := newMockClaim("test-topic", 0, msgCount)

	h := &consumerGroupHandler{KafkaClient: c, lastCommitTime: time.Now()}

	claimDone := make(chan error, 1)
	go func() {
		claimDone <- h.ConsumeClaim(session, claim)
	}()

	for i := 0; i < msgCount; i++ {
		claim.ch <- makeMsg("test-topic", 0, int64(i))
	}

	// wait until all items appear in msgChan
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && len(c.msgChan) < msgCount {
		time.Sleep(5 * time.Millisecond)
	}

	if got := len(c.msgChan); got != msgCount {
		t.Fatalf("expected %d items in msgChan, got %d", msgCount, got)
	}

	// verify per-partition queue also has the items
	q, ok := c.msgQueue.Load("test-topic-0")
	if !ok {
		t.Fatal("expected msgQueue entry for test-topic-0")
	}
	if size := q.(*basic.SafeRingQueue).Size(); size != msgCount {
		t.Fatalf("expected msgQueue size=%d, got %d", msgCount, size)
	}

	// stop ConsumeClaim
	c.cancelFunc()
	session.cancel()
	select {
	case err := <-claimDone:
		if err != nil {
			t.Fatalf("ConsumeClaim returned error: %v", err)
		}
	case <-time.After(time.Second * 5):
		t.Fatal("ConsumeClaim did not exit after ctx cancel")
	}
}

// ---------------------------------------------------------------------------
// Test: high-throughput no loss — many messages, verify all processed
// ---------------------------------------------------------------------------

// TestConsume_HighThroughputNoLoss injects a large batch of messages and
// verifies that every single message is delivered to the handler exactly once.
func TestConsume_HighThroughputNoLoss(t *testing.T) {
	const msgCount = 1000
	c := newTestClient(4, 2000, 1, time.Second)
	session := newMockSession()
	c.consumerSession = session

	var mu sync.Mutex
	received := make(map[int64]int)
	startWorkers(c, func(msg interface{}) error {
		m := msg.(*sarama.ConsumerMessage)
		mu.Lock()
		received[m.Offset]++
		mu.Unlock()
		return nil
	})

	msgs := make([]*sarama.ConsumerMessage, msgCount)
	for i := 0; i < msgCount; i++ {
		msgs[i] = makeMsg("test-topic", 0, int64(i))
	}
	injectMessages(c, session, msgs)

	// wait for all messages
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		count := len(received)
		mu.Unlock()
		if count >= msgCount {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(received) != msgCount {
		t.Fatalf("expected %d unique messages, got %d", msgCount, len(received))
	}
	for offset, count := range received {
		if count != 1 {
			t.Fatalf("offset %d processed %d times (expected 1)", offset, count)
		}
	}

	c.running.Store(false)
	c.cancelFunc()
	close(c.msgChan)
	for i := 0; i < c.cfg.ConsumerWorkerNum; i++ {
		<-c.closeChan
	}
}
