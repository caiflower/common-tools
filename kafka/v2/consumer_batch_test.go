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
	"sync/atomic"
	"testing"
	"time"

	"github.com/IBM/sarama"
	xkafka "github.com/caiflower/common-tools/kafka"
	"github.com/caiflower/common-tools/pkg/basic"
)

func TestConsumerGroupHandlerCollectsBatchBySize(t *testing.T) {
	cfg := xkafka.Config{
		ConsumerBatchSize: 3,
		ConsumerBatchWait: time.Second,
	}
	client := &KafkaClient{cfg: &cfg, ctx: context.Background()}
	handler := &consumerGroupHandler{KafkaClient: client}

	messages := make(chan *sarama.ConsumerMessage, cfg.ConsumerBatchSize)
	for i := int64(0); i < int64(cfg.ConsumerBatchSize); i++ {
		messages <- &sarama.ConsumerMessage{
			Topic:     "batch-topic",
			Partition: 2,
			Offset:    i,
			Value:     []byte{byte(i)},
		}
	}
	claim := &testConsumerGroupClaim{
		topic:     "batch-topic",
		partition: 2,
		messages:  messages,
	}

	batch, ok := handler.collectBatch(&testConsumerGroupSession{ctx: context.Background()}, claim)
	if !ok {
		t.Fatal("expected batch collection to continue")
	}
	if len(batch) != cfg.ConsumerBatchSize {
		t.Fatalf("expected %d messages, got %d", cfg.ConsumerBatchSize, len(batch))
	}
	for i, msg := range batch {
		if msg.Offset != int64(i) {
			t.Fatalf("expected offset %d at index %d, got %d", i, i, msg.Offset)
		}
	}
}

func TestConsumerGroupHandlerCollectsBatchByWait(t *testing.T) {
	cfg := xkafka.Config{
		ConsumerBatchSize: 10,
		ConsumerBatchWait: 30 * time.Millisecond,
	}
	client := &KafkaClient{cfg: &cfg, ctx: context.Background()}
	handler := &consumerGroupHandler{KafkaClient: client}

	messages := make(chan *sarama.ConsumerMessage, 1)
	messages <- &sarama.ConsumerMessage{
		Topic:     "batch-topic",
		Partition: 1,
		Offset:    10,
		Value:     []byte("partial"),
	}
	claim := &testConsumerGroupClaim{
		topic:     "batch-topic",
		partition: 1,
		messages:  messages,
	}

	start := time.Now()
	batch, ok := handler.collectBatch(&testConsumerGroupSession{ctx: context.Background()}, claim)
	elapsed := time.Since(start)

	if !ok {
		t.Fatal("expected partial batch to be returned")
	}
	if len(batch) != 1 {
		t.Fatalf("expected 1 message, got %d", len(batch))
	}
	if elapsed < cfg.ConsumerBatchWait/2 {
		t.Fatalf("expected batch wait, returned after %v", elapsed)
	}
}

func TestKafkaClientProcessBatchRetriesWholeBatch(t *testing.T) {
	cfg := xkafka.Config{
		ConsumerRetryCount: 2,
	}
	client := &KafkaClient{cfg: &cfg, ctx: context.Background()}
	item := &batchItem{
		messages: []*sarama.ConsumerMessage{
			{Topic: "batch-topic", Partition: 0, Offset: 1},
			{Topic: "batch-topic", Partition: 0, Offset: 2},
		},
		values: []interface{}{
			&sarama.ConsumerMessage{Topic: "batch-topic", Partition: 0, Offset: 1},
			&sarama.ConsumerMessage{Topic: "batch-topic", Partition: 0, Offset: 2},
		},
	}

	var calls int32
	var batchSizes []int
	client.processBatch(item, func(messages []interface{}) error {
		atomic.AddInt32(&calls, 1)
		batchSizes = append(batchSizes, len(messages))
		if len(batchSizes) == 1 {
			return context.DeadlineExceeded
		}
		return nil
	}, nil)

	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Fatalf("expected 2 callback calls, got %d", got)
	}
	if len(batchSizes) != 2 || batchSizes[0] != 2 || batchSizes[1] != 2 {
		t.Fatalf("expected whole batch on every attempt, got sizes %v", batchSizes)
	}
	if !item.done.Load() {
		t.Fatal("expected successful batch to be marked done")
	}
}

func TestKafkaClientProcessBatchDeadLetterOnce(t *testing.T) {
	cfg := xkafka.Config{
		ConsumerRetryCount: 1,
	}
	client := &KafkaClient{cfg: &cfg, ctx: context.Background()}
	messages := []*sarama.ConsumerMessage{
		{Topic: "batch-topic", Partition: 3, Offset: 5},
		{Topic: "batch-topic", Partition: 3, Offset: 6},
	}
	item := newBatchItem(messages)

	var calls int32
	var deadLetterSize int
	client.processBatch(item, func([]interface{}) error {
		return context.DeadlineExceeded
	}, func(messages []interface{}, _ error) error {
		atomic.AddInt32(&calls, 1)
		deadLetterSize = len(messages)
		return nil
	})

	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("expected dead letter handler once, got %d", got)
	}
	if deadLetterSize != len(messages) {
		t.Fatalf("expected %d messages in dead letter batch, got %d", len(messages), deadLetterSize)
	}
	if !item.done.Load() {
		t.Fatal("expected exhausted batch to be marked done after dead letter handling")
	}
}

func TestKafkaClientCloseStopsBatchWorker(t *testing.T) {
	cfg := xkafka.Config{
		Name:              "batch-close-test",
		ConsumerWorkerNum: 1,
	}
	ctx, cancel := context.WithCancel(context.Background())
	client := &KafkaClient{
		cfg:          &cfg,
		ctx:          ctx,
		cancelFunc:   cancel,
		batchMode:    true,
		batchSem:     make(chan struct{}, 1),
		batchHandler: func([]interface{}) error { return nil },
	}
	client.running.Store(true)

	batchChan := make(chan *batchItem)
	client.batchQueues.Store("batch-topic-0", batchChan)
	client.batchWG.Add(1)
	go client.consumeBatch(batchChan)

	done := make(chan struct{})
	go func() {
		client.Close()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Close did not stop the batch worker")
	}
}

func TestConsumerGroupHandlerSetupResetsReplayOffsetOnce(t *testing.T) {
	cfg := xkafka.Config{}
	client := &KafkaClient{cfg: &cfg}
	client.consumerReplayOffsets = map[string]int64{
		topicPartitionKey("batch-topic", 0): 7,
	}
	handler := &consumerGroupHandler{KafkaClient: client}
	session := &testConsumerGroupSession{
		ctx: context.Background(),
		claims: map[string][]int32{
			"batch-topic": {0, 1},
		},
	}

	if err := handler.Setup(session); err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	if len(session.resetOffsets) != 1 {
		t.Fatalf("expected one replayed offset, got %v", session.resetOffsets)
	}
	if got := session.resetOffsets[0]; got.topic != "batch-topic" || got.partition != 0 || got.offset != 7 {
		t.Fatalf("unexpected replay offset: %+v", got)
	}
	if session.commitCount != 1 {
		t.Fatalf("expected replay offset to be committed once, got %d", session.commitCount)
	}

	if err := handler.Setup(session); err != nil {
		t.Fatalf("second setup failed: %v", err)
	}
	if len(session.resetOffsets) != 1 {
		t.Fatalf("expected replay offset not to reset again, got %v", session.resetOffsets)
	}
	if session.commitCount != 1 {
		t.Fatalf("expected no extra commit, got %d", session.commitCount)
	}
}

func TestConsumerGroupHandlerSetupResetsReplayOffsetAfterRebalance(t *testing.T) {
	cfg := xkafka.Config{}
	client := &KafkaClient{cfg: &cfg}
	client.consumerReplayOffsets = map[string]int64{
		topicPartitionKey("batch-topic", 1): 9,
	}
	handler := &consumerGroupHandler{KafkaClient: client}

	firstSession := &testConsumerGroupSession{
		ctx: context.Background(),
		claims: map[string][]int32{
			"batch-topic": {0},
		},
	}
	if err := handler.Setup(firstSession); err != nil {
		t.Fatalf("first setup failed: %v", err)
	}
	if len(firstSession.resetOffsets) != 0 || firstSession.commitCount != 0 {
		t.Fatalf("expected no replay before partition assignment, got offsets=%v commits=%d", firstSession.resetOffsets, firstSession.commitCount)
	}

	secondSession := &testConsumerGroupSession{
		ctx: context.Background(),
		claims: map[string][]int32{
			"batch-topic": {1},
		},
	}
	if err := handler.Setup(secondSession); err != nil {
		t.Fatalf("second setup failed: %v", err)
	}
	if len(secondSession.resetOffsets) != 1 {
		t.Fatalf("expected replay after partition assignment, got %v", secondSession.resetOffsets)
	}
	if got := secondSession.resetOffsets[0]; got.partition != 1 || got.offset != 9 {
		t.Fatalf("unexpected replay offset: %+v", got)
	}
	if secondSession.commitCount != 1 {
		t.Fatalf("expected one replay commit, got %d", secondSession.commitCount)
	}
}

func TestBuildConsumerReplayOffsets(t *testing.T) {
	offsets := buildConsumerReplayOffsets([]xkafka.ConsumerReplayOffset{
		{Topic: "", Partition: 0, Offset: 1},
		{Topic: "topic", Partition: 0, Offset: 2},
		{Topic: "topic", Partition: 1, Offset: 3},
		{Topic: "topic", Partition: 0, Offset: 4},
	})

	if len(offsets) != 2 {
		t.Fatalf("expected 2 configured offsets, got %v", offsets)
	}
	if got := offsets[topicPartitionKey("topic", 0)]; got != 4 {
		t.Fatalf("expected duplicate offset to use the last value 4, got %d", got)
	}
	if got := offsets[topicPartitionKey("topic", 1)]; got != 3 {
		t.Fatalf("expected partition 1 offset 3, got %d", got)
	}
}

func TestConsumerGroupHandlerConsumesPartitionLocalBatches(t *testing.T) {
	cfg := xkafka.Config{
		Name:              "batch-partition-test",
		ConsumerWorkerNum: 1,
		ConsumerQueueSize: 20,
		ConsumerBatchSize: 2,
		ConsumerBatchWait: 10 * time.Millisecond,
	}
	ctx, cancel := context.WithCancel(context.Background())
	client := &KafkaClient{
		cfg:        &cfg,
		ctx:        ctx,
		cancelFunc: cancel,
		batchMode:  true,
		batchSem:   make(chan struct{}, 1),
	}
	client.running.Store(true)

	received := make(chan []int64, 3)
	client.batchHandler = func(messages []interface{}) error {
		offsets := make([]int64, 0, len(messages))
		for _, message := range messages {
			offsets = append(offsets, message.(*sarama.ConsumerMessage).Offset)
		}
		received <- offsets
		return nil
	}

	messages := make(chan *sarama.ConsumerMessage, 5)
	for i := int64(0); i < 5; i++ {
		messages <- &sarama.ConsumerMessage{
			Topic:     "batch-topic",
			Partition: 4,
			Offset:    i,
		}
	}
	close(messages)

	handler := &consumerGroupHandler{KafkaClient: client}
	session := &testConsumerGroupSession{ctx: context.Background()}
	claim := &testConsumerGroupClaim{
		topic:     "batch-topic",
		partition: 4,
		messages:  messages,
	}
	go func() {
		_ = handler.ConsumeClaim(session, claim)
	}()

	expected := [][]int64{{0, 1}, {2, 3}, {4}}
	for _, want := range expected {
		select {
		case got := <-received:
			if len(got) != len(want) {
				t.Fatalf("expected batch %v, got %v", want, got)
			}
			for i := range want {
				if got[i] != want[i] {
					t.Fatalf("expected batch %v, got %v", want, got)
				}
			}
		case <-time.After(time.Second):
			t.Fatalf("timed out waiting for batch %v", want)
		}
	}

	client.Close()
}

func TestConsumerGroupHandlerFlushesPartialBatchToWorker(t *testing.T) {
	cfg := xkafka.Config{
		Name:              "batch-partial-flush-test",
		ConsumerWorkerNum: 1,
		ConsumerQueueSize: 20,
		ConsumerBatchSize: 10,
		ConsumerBatchWait: 30 * time.Millisecond,
	}
	ctx, cancel := context.WithCancel(context.Background())
	client := &KafkaClient{
		cfg:        &cfg,
		ctx:        ctx,
		cancelFunc: cancel,
		batchMode:  true,
		batchSem:   make(chan struct{}, 1),
	}
	client.running.Store(true)

	received := make(chan []int64, 1)
	client.batchHandler = func(messages []interface{}) error {
		offsets := make([]int64, 0, len(messages))
		for _, message := range messages {
			offsets = append(offsets, message.(*sarama.ConsumerMessage).Offset)
		}
		received <- offsets
		return nil
	}

	messages := make(chan *sarama.ConsumerMessage, 1)
	messages <- &sarama.ConsumerMessage{
		Topic:     "batch-topic",
		Partition: 5,
		Offset:    100,
	}
	handler := &consumerGroupHandler{KafkaClient: client}
	session := &testConsumerGroupSession{ctx: context.Background()}
	claim := &testConsumerGroupClaim{
		topic:     "batch-topic",
		partition: 5,
		messages:  messages,
	}
	go func() {
		_ = handler.ConsumeClaim(session, claim)
	}()

	select {
	case got := <-received:
		if len(got) != 1 || got[0] != 100 {
			t.Fatalf("expected partial batch [100], got %v", got)
		}
	case <-time.After(time.Second):
		t.Fatal("partial batch did not reach the batch worker")
	}

	deadline := time.Now().Add(time.Second)
	for len(session.markedOffsets) == 0 && time.Now().Before(deadline) {
		client.commitCompletedOffsets()
		time.Sleep(time.Millisecond)
	}
	if len(session.markedOffsets) != 1 || session.markedOffsets[0] != 100 {
		t.Fatalf("expected partial batch offset 100 to commit, got %v", session.markedOffsets)
	}

	client.Close()
}

func TestConsumerGroupHandlerDoesNotDispatchPartialBatchAfterRebalance(t *testing.T) {
	cfg := xkafka.Config{
		Name:              "batch-rebalance-test",
		ConsumerWorkerNum: 1,
		ConsumerQueueSize: 20,
		ConsumerBatchSize: 10,
		ConsumerBatchWait: time.Second,
	}
	ctx, cancel := context.WithCancel(context.Background())
	client := &KafkaClient{
		cfg:        &cfg,
		ctx:        ctx,
		cancelFunc: cancel,
		batchMode:  true,
		batchSem:   make(chan struct{}, 1),
	}
	client.running.Store(true)

	var calls int32
	client.batchHandler = func([]interface{}) error {
		atomic.AddInt32(&calls, 1)
		return nil
	}

	messages := make(chan *sarama.ConsumerMessage, 1)
	messages <- &sarama.ConsumerMessage{
		Topic:     "batch-topic",
		Partition: 6,
		Offset:    200,
	}
	sessionCtx, cancelSession := context.WithCancel(context.Background())
	session := &testConsumerGroupSession{ctx: sessionCtx}
	handler := &consumerGroupHandler{KafkaClient: client}
	claim := &testConsumerGroupClaim{
		topic:     "batch-topic",
		partition: 6,
		messages:  messages,
	}

	done := make(chan struct{})
	go func() {
		_ = handler.ConsumeClaim(session, claim)
		close(done)
	}()

	deadline := time.Now().Add(time.Second)
	for len(messages) != 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if len(messages) != 0 {
		t.Fatal("claim did not start collecting the partial batch")
	}

	cancelSession()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("claim did not stop after the session was canceled")
	}

	if got := atomic.LoadInt32(&calls); got != 0 {
		t.Fatalf("expected no batch handler call after rebalance, got %d", got)
	}
	key := topicPartitionKey("batch-topic", 6)
	if _, ok := client.batchQueues.Load(key); ok {
		t.Fatal("expected no batch worker to be created after rebalance")
	}
	if _, ok := client.msgQueue.Load(key); ok {
		t.Fatal("expected no offset queue to be created after rebalance")
	}

	client.Close()
}

func TestListenBatchConsumesPartialBatchViaMockBroker(t *testing.T) {
	broker := newConsumerMockBroker(t)
	defer broker.Close()

	cfg := newTestConsumerConfig(broker.Addr())
	cfg.ConsumerBatchSize = 10
	cfg.ConsumerBatchWait = 50 * time.Millisecond

	client := NewConsumerClient(cfg)
	defer client.Close()

	received := make(chan []string, 1)
	client.ListenBatch(func(messages []interface{}) error {
		values := make([]string, 0, len(messages))
		for _, message := range messages {
			values = append(values, string(message.(*KafkaMessage).Value))
		}
		received <- values
		return nil
	})

	select {
	case values := <-received:
		if len(values) != 1 || values[0] != "hello-consumer" {
			t.Fatalf("expected mock broker batch [hello-consumer], got %v", values)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for mock broker batch")
	}
}

func TestListenBatchConsumesBySizeViaMockBroker(t *testing.T) {
	broker := newConsumerMockBrokerWithOptions(t, consumerMockBrokerOptions{
		partitions: map[string][]int32{
			testTopic: {0},
		},
		messages: map[string]map[int32][]string{
			testTopic: {
				0: {"one", "two", "three"},
			},
		},
	})
	defer broker.Close()

	cfg := newTestConsumerConfig(broker.Addr())
	cfg.ConsumerBatchSize = 2
	cfg.ConsumerBatchWait = 50 * time.Millisecond

	client := NewConsumerClient(cfg)
	defer client.Close()

	received := make(chan []string, 2)
	client.ListenBatch(func(messages []interface{}) error {
		values := make([]string, 0, len(messages))
		for _, message := range messages {
			values = append(values, string(message.(*KafkaMessage).Value))
		}
		received <- values
		return nil
	})

	first := receiveBatchValues(t, received)
	second := receiveBatchValues(t, received)
	if len(first) != 2 || first[0] != "one" || first[1] != "two" {
		t.Fatalf("expected first size-triggered batch [one two], got %v", first)
	}
	if len(second) != 1 || second[0] != "three" {
		t.Fatalf("expected second partial batch [three], got %v", second)
	}
}

func TestListenBatchKeepsPartitionsSeparateViaMockBroker(t *testing.T) {
	broker := newConsumerMockBrokerWithOptions(t, consumerMockBrokerOptions{
		partitions: map[string][]int32{
			testTopic: {0, 1},
		},
		messages: map[string]map[int32][]string{
			testTopic: {
				0: {"p0-one", "p0-two"},
				1: {"p1-one"},
			},
		},
	})
	defer broker.Close()

	cfg := newTestConsumerConfig(broker.Addr())
	cfg.ConsumerBatchSize = 2
	cfg.ConsumerBatchWait = 30 * time.Millisecond

	client := NewConsumerClient(cfg)
	defer client.Close()

	type partitionBatch struct {
		partition int32
		values    []string
	}
	received := make(chan partitionBatch, 2)
	client.ListenBatch(func(messages []interface{}) error {
		partition := messages[0].(*KafkaMessage).Partition
		values := make([]string, 0, len(messages))
		for _, message := range messages {
			msg := message.(*KafkaMessage)
			if msg.Partition != partition {
				t.Fatalf("batch mixed partitions %d and %d", partition, msg.Partition)
			}
			values = append(values, string(msg.Value))
		}
		received <- partitionBatch{partition: partition, values: values}
		return nil
	})

	got := make(map[int32][]string, 2)
	for i := 0; i < 2; i++ {
		select {
		case batch := <-received:
			got[batch.partition] = batch.values
		case <-time.After(5 * time.Second):
			t.Fatalf("timed out waiting for partition batch %d", i+1)
		}
	}

	if values := got[0]; len(values) != 2 || values[0] != "p0-one" || values[1] != "p0-two" {
		t.Fatalf("unexpected partition 0 batch: %v", values)
	}
	if values := got[1]; len(values) != 1 || values[0] != "p1-one" {
		t.Fatalf("unexpected partition 1 batch: %v", values)
	}
}

func TestListenBatchRetriesWholeBatchViaMockBroker(t *testing.T) {
	broker := newConsumerMockBrokerWithOptions(t, consumerMockBrokerOptions{
		partitions: map[string][]int32{
			testTopic: {0},
		},
		messages: map[string]map[int32][]string{
			testTopic: {
				0: {"retry-one", "retry-two"},
			},
		},
	})
	defer broker.Close()

	cfg := newTestConsumerConfig(broker.Addr())
	cfg.ConsumerBatchSize = 2
	cfg.ConsumerBatchWait = 20 * time.Millisecond
	cfg.ConsumerRetryCount = 2

	client := NewConsumerClient(cfg)
	defer client.Close()

	var calls int32
	received := make(chan []string, 1)
	deadLetter := make(chan []string, 1)
	client.ListenBatch(func(messages []interface{}) error {
		call := atomic.AddInt32(&calls, 1)
		values := make([]string, 0, len(messages))
		for _, message := range messages {
			values = append(values, string(message.(*KafkaMessage).Value))
		}
		if call == 1 {
			return errors.New("retry batch")
		}
		received <- values
		return nil
	}, func(messages []interface{}, _ error) error {
		values := make([]string, 0, len(messages))
		for _, message := range messages {
			values = append(values, string(message.(*KafkaMessage).Value))
		}
		deadLetter <- values
		return nil
	})

	values := receiveBatchValues(t, received)
	if len(values) != 2 || values[0] != "retry-one" || values[1] != "retry-two" {
		t.Fatalf("expected whole batch retry [retry-one retry-two], got %v", values)
	}
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Fatalf("expected 2 batch attempts, got %d", got)
	}
	select {
	case values := <-deadLetter:
		t.Fatalf("did not expect dead letter after successful retry, got %v", values)
	default:
	}
}

func TestListenBatchDeadLettersWholeBatchViaMockBroker(t *testing.T) {
	broker := newConsumerMockBrokerWithOptions(t, consumerMockBrokerOptions{
		partitions: map[string][]int32{
			testTopic: {0},
		},
		messages: map[string]map[int32][]string{
			testTopic: {
				0: {"dead-one", "dead-two"},
			},
		},
	})
	defer broker.Close()

	cfg := newTestConsumerConfig(broker.Addr())
	cfg.ConsumerBatchSize = 2
	cfg.ConsumerBatchWait = 20 * time.Millisecond
	cfg.ConsumerRetryCount = 1

	client := NewConsumerClient(cfg)
	defer client.Close()

	var calls int32
	deadLetter := make(chan []string, 1)
	client.ListenBatch(func([]interface{}) error {
		atomic.AddInt32(&calls, 1)
		return errors.New("dead batch")
	}, func(messages []interface{}, _ error) error {
		values := make([]string, 0, len(messages))
		for _, message := range messages {
			values = append(values, string(message.(*KafkaMessage).Value))
		}
		deadLetter <- values
		return nil
	})

	select {
	case values := <-deadLetter:
		if len(values) != 2 || values[0] != "dead-one" || values[1] != "dead-two" {
			t.Fatalf("expected whole dead letter batch [dead-one dead-two], got %v", values)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for dead letter batch")
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("expected one batch attempt before dead letter, got %d", got)
	}
}

func TestListenBatchReplaysFromConfiguredOffsetViaMockBroker(t *testing.T) {
	broker := newConsumerMockBrokerWithOptions(t, consumerMockBrokerOptions{
		partitions: map[string][]int32{
			testTopic: {0},
		},
		messages: map[string]map[int32][]string{
			testTopic: {
				0: {"offset-zero", "offset-one", "offset-two"},
			},
		},
		committedOffsets: map[string]map[int32]int64{
			testTopic: {
				0: 3,
			},
		},
	})
	defer broker.Close()

	cfg := newTestConsumerConfig(broker.Addr())
	cfg.ConsumerBatchSize = 10
	cfg.ConsumerBatchWait = 20 * time.Millisecond
	cfg.ConsumerCommitInterval = 5 * time.Millisecond
	cfg.ConsumerReplayOffsets = []xkafka.ConsumerReplayOffset{
		{
			Topic:     testTopic,
			Partition: 0,
			Offset:    1,
		},
	}

	client := NewConsumerClient(cfg)
	defer client.Close()

	received := make(chan []string, 1)
	client.ListenBatch(func(messages []interface{}) error {
		values := make([]string, 0, len(messages))
		for _, message := range messages {
			values = append(values, string(message.(*KafkaMessage).Value))
		}
		received <- values
		return nil
	})

	values := receiveBatchValues(t, received)
	if len(values) != 2 || values[0] != "offset-one" || values[1] != "offset-two" {
		t.Fatalf("expected replay batch [offset-one offset-two], got %v", values)
	}
}

func TestListenBatchReplaysWithoutCommittedOffsetViaMockBroker(t *testing.T) {
	broker := newConsumerMockBrokerWithOptions(t, consumerMockBrokerOptions{
		partitions: map[string][]int32{
			testTopic: {0},
		},
		messages: map[string]map[int32][]string{
			testTopic: {
				0: {"offset-zero", "offset-one", "offset-two"},
			},
		},
		committedOffsets: map[string]map[int32]int64{
			testTopic: {
				0: -1,
			},
		},
	})
	defer broker.Close()

	cfg := newTestConsumerConfig(broker.Addr())
	cfg.ConsumerBatchSize = 10
	cfg.ConsumerBatchWait = 20 * time.Millisecond
	cfg.ConsumerReplayOffsets = []xkafka.ConsumerReplayOffset{
		{
			Topic:     testTopic,
			Partition: 0,
			Offset:    1,
		},
	}

	client := NewConsumerClient(cfg)
	defer client.Close()

	received := make(chan []string, 1)
	client.ListenBatch(func(messages []interface{}) error {
		values := make([]string, 0, len(messages))
		for _, message := range messages {
			values = append(values, string(message.(*KafkaMessage).Value))
		}
		received <- values
		return nil
	})

	values := receiveBatchValues(t, received)
	if len(values) != 2 || values[0] != "offset-one" || values[1] != "offset-two" {
		t.Fatalf("expected replay batch [offset-one offset-two] without committed offset, got %v", values)
	}
}

func TestListenReplaysFromConfiguredOffsetViaMockBroker(t *testing.T) {
	broker := newConsumerMockBrokerWithOptions(t, consumerMockBrokerOptions{
		partitions: map[string][]int32{
			testTopic: {0},
		},
		messages: map[string]map[int32][]string{
			testTopic: {
				0: {"offset-zero", "offset-one", "offset-two"},
			},
		},
		committedOffsets: map[string]map[int32]int64{
			testTopic: {
				0: 3,
			},
		},
	})
	defer broker.Close()

	cfg := newTestConsumerConfig(broker.Addr())
	cfg.ConsumerReplayOffsets = []xkafka.ConsumerReplayOffset{
		{
			Topic:     testTopic,
			Partition: 0,
			Offset:    1,
		},
	}

	client := NewConsumerClient(cfg)
	defer client.Close()

	received := make(chan string, 2)
	client.Listen(func(message interface{}) error {
		received <- string(message.(*KafkaMessage).Value)
		return nil
	})

	for _, want := range []string{"offset-one", "offset-two"} {
		select {
		case got := <-received:
			if got != want {
				t.Fatalf("expected replayed message %q, got %q", want, got)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("timed out waiting for replayed message %q", want)
		}
	}
}

func TestListenReplaysMultipleTopicsAndPartitionsViaMockBroker(t *testing.T) {
	const (
		topicA = "replay-topic-a"
		topicB = "replay-topic-b"
	)

	broker := newConsumerMockBrokerWithOptions(t, consumerMockBrokerOptions{
		partitions: map[string][]int32{
			topicA: {0, 1},
			topicB: {0, 1},
		},
		messages: map[string]map[int32][]string{
			topicA: {
				0: {"a0-0", "a0-1", "a0-2"},
				1: {"a1-0", "a1-1", "a1-2"},
			},
			topicB: {
				0: {"b0-0", "b0-1"},
				1: {"b1-0", "b1-1", "b1-2"},
			},
		},
		committedOffsets: map[string]map[int32]int64{
			topicA: {
				0: 3,
				1: 3,
			},
			topicB: {
				0: 2,
				1: 2,
			},
		},
	})
	defer broker.Close()

	cfg := newTestConsumerConfig(broker.Addr())
	cfg.Topics = []string{topicA, topicB}
	cfg.ConsumerWorkerNum = 4
	cfg.ConsumerReplayOffsets = []xkafka.ConsumerReplayOffset{
		{Topic: topicA, Partition: 0, Offset: 1},
		{Topic: topicA, Partition: 1, Offset: 2},
		{Topic: topicB, Partition: 0, Offset: 1},
		{Topic: topicB, Partition: 1, Offset: 1},
	}

	client := NewConsumerClient(cfg)
	defer client.Close()

	expected := []string{
		"a0-1", "a0-2",
		"a1-2",
		"b0-1",
		"b1-1", "b1-2",
	}
	received := make(chan string, len(expected)+10)
	client.Listen(func(message interface{}) error {
		received <- string(message.(*KafkaMessage).Value)
		return nil
	})

	counts := make(map[string]int, len(expected))
	for range expected {
		select {
		case value := <-received:
			counts[value]++
		case <-time.After(5 * time.Second):
			t.Fatalf("timed out waiting for replay messages, got %v", counts)
		}
	}

	for _, value := range expected {
		if counts[value] != 1 {
			t.Fatalf("expected message %q exactly once, got counts %v", value, counts)
		}
	}
}

func TestListenReplayCommitPersistsAcrossRestartViaMockBroker(t *testing.T) {
	broker, offsetFetch := newConsumerMockBrokerHarnessWithOptions(t, consumerMockBrokerOptions{
		partitions: map[string][]int32{
			testTopic: {0},
		},
		messages: map[string]map[int32][]string{
			testTopic: {
				0: {"offset-zero", "offset-one", "offset-two"},
			},
		},
	})
	defer broker.Close()

	cfg := newTestConsumerConfig(broker.Addr())
	cfg.ConsumerBatchSize = 10
	cfg.ConsumerBatchWait = 20 * time.Millisecond
	cfg.ConsumerReplayOffsets = []xkafka.ConsumerReplayOffset{
		{Topic: testTopic, Partition: 0, Offset: 2},
	}

	client := NewConsumerClient(cfg)
	received := make(chan []string, 1)
	client.ListenBatch(func(messages []interface{}) error {
		values := make([]string, 0, len(messages))
		for _, message := range messages {
			values = append(values, string(message.(*KafkaMessage).Value))
		}
		received <- values
		return nil
	})

	values := receiveBatchValues(t, received)
	if len(values) != 1 || values[0] != "offset-two" {
		t.Fatalf("expected replay batch [offset-two], got %v", values)
	}
	waitForQueueItemDone(t, client, topicPartitionKey(testTopic, 0))

	deadline := time.Now().Add(3 * time.Second)
	var committedOffset int64
	for time.Now().Before(deadline) {
		if offset, ok := committedOffsetFromHistory(broker, "test-group", testTopic, 0); ok && offset == 3 {
			committedOffset = offset
			break
		}
		time.Sleep(time.Millisecond)
	}
	if committedOffset != 3 {
		t.Fatalf("expected replay completion offset 3 to be committed, got %d", committedOffset)
	}
	client.Close()

	// The mock broker does not persist offset commits by itself. Feed the
	// actual committed offset back into the offset-fetch response to simulate
	// a restart against a broker that did persist it.
	offsetFetch.SetOffset("test-group", testTopic, 0, committedOffset, "", sarama.ErrNoError)

	restartCfg := cfg
	restartCfg.ConsumerReplayOffsets = nil
	restartClient := NewConsumerClient(restartCfg)
	defer restartClient.Close()

	historyStart := len(broker.History())
	restarted := make(chan string, 1)
	restartClient.Listen(func(message interface{}) error {
		restarted <- string(message.(*KafkaMessage).Value)
		return nil
	})
	waitForBrokerRequest(t, broker, historyStart, func(req interface{}) bool {
		_, ok := req.(*sarama.FetchRequest)
		return ok
	})

	select {
	case value := <-restarted:
		t.Fatalf("expected no message after restart from committed offset, got %q", value)
	case <-time.After(100 * time.Millisecond):
	}
}

func committedOffsetFromHistory(broker *sarama.MockBroker, group, topic string, partition int32) (int64, bool) {
	var (
		lastOffset int64
		found      bool
	)
	for _, entry := range broker.History() {
		req, ok := entry.Request.(*sarama.OffsetCommitRequest)
		if !ok || req.ConsumerGroup != group {
			continue
		}
		offset, _, err := req.Offset(topic, partition)
		if err != nil {
			continue
		}
		lastOffset = offset
		found = true
	}
	return lastOffset, found
}

func waitForQueueItemDone(t *testing.T, client *KafkaClient, key string) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		value, ok := client.msgQueue.Load(key)
		if ok {
			if item, err := value.(*basic.SafeRingQueue).Peek(); err == nil {
				if queued, ok := item.(queuedItem); ok && queued.isDone() {
					return
				}
			}
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("timed out waiting for queued item %s to complete", key)
}

func waitForBrokerRequest(t *testing.T, broker *sarama.MockBroker, start int, match func(interface{}) bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		history := broker.History()
		for _, entry := range history[start:] {
			if match(entry.Request) {
				return
			}
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("timed out waiting for broker request")
}

func receiveBatchValues(t *testing.T, received <-chan []string) []string {
	t.Helper()
	select {
	case values := <-received:
		return values
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for batch")
		return nil
	}
}

func TestKafkaClientCommitsOnlyContiguousCompletedBatches(t *testing.T) {
	cfg := xkafka.Config{Name: "batch-commit-test"}
	session := &testConsumerGroupSession{ctx: context.Background()}
	first := &batchItem{messages: []*sarama.ConsumerMessage{{Topic: "batch-topic", Partition: 0, Offset: 1}}}
	second := &batchItem{messages: []*sarama.ConsumerMessage{{Topic: "batch-topic", Partition: 0, Offset: 2}}}
	queue := basic.NewSafeRingQueue(2)
	_ = queue.Enqueue(first)
	_ = queue.Enqueue(second)
	client := &KafkaClient{
		cfg:             &cfg,
		ctx:             context.Background(),
		consumerSession: session,
	}
	client.msgQueue.Store("batch-topic-0", queue)

	client.commitCompletedOffsets()
	if len(session.markedOffsets) != 0 {
		t.Fatalf("expected no offset commit while the first batch is incomplete, got %v", session.markedOffsets)
	}
	if got := queue.Size(); got != 2 {
		t.Fatalf("expected both batches to remain queued, got %d", got)
	}

	first.markDone()
	client.commitCompletedOffsets()
	if len(session.markedOffsets) != 1 || session.markedOffsets[0] != 1 {
		t.Fatalf("expected first batch offset to commit, got %v", session.markedOffsets)
	}
	if got := queue.Size(); got != 1 {
		t.Fatalf("expected completed leading batch to be dequeued, got %d", got)
	}

	second.markDone()
	client.commitCompletedOffsets()
	if len(session.markedOffsets) != 2 || session.markedOffsets[1] != 2 {
		t.Fatalf("expected second batch offset to commit, got %v", session.markedOffsets)
	}
	if got := queue.Size(); got != 0 {
		t.Fatalf("expected all completed batches to be dequeued, got %d", got)
	}
}

type testConsumerGroupClaim struct {
	topic     string
	partition int32
	messages  <-chan *sarama.ConsumerMessage
}

func (c *testConsumerGroupClaim) Topic() string                            { return c.topic }
func (c *testConsumerGroupClaim) Partition() int32                         { return c.partition }
func (c *testConsumerGroupClaim) InitialOffset() int64                     { return 0 }
func (c *testConsumerGroupClaim) HighWaterMarkOffset() int64               { return 0 }
func (c *testConsumerGroupClaim) Messages() <-chan *sarama.ConsumerMessage { return c.messages }

type testConsumerGroupSession struct {
	ctx           context.Context
	markedOffsets []int64
	claims        map[string][]int32
	resetOffsets  []consumerSessionResetOffset
	commitCount   int
}

type consumerSessionResetOffset struct {
	topic     string
	partition int32
	offset    int64
}

func (s *testConsumerGroupSession) Claims() map[string][]int32 { return s.claims }
func (s *testConsumerGroupSession) MemberID() string           { return "" }
func (s *testConsumerGroupSession) GenerationID() int32        { return 0 }
func (s *testConsumerGroupSession) MarkOffset(string, int32, int64, string) {
}
func (s *testConsumerGroupSession) Commit() {
	s.commitCount++
}
func (s *testConsumerGroupSession) ResetOffset(topic string, partition int32, offset int64, _ string) {
	s.resetOffsets = append(s.resetOffsets, consumerSessionResetOffset{
		topic:     topic,
		partition: partition,
		offset:    offset,
	})
}
func (s *testConsumerGroupSession) MarkMessage(msg *sarama.ConsumerMessage, _ string) {
	s.markedOffsets = append(s.markedOffsets, msg.Offset)
}
func (s *testConsumerGroupSession) Context() context.Context { return s.ctx }
