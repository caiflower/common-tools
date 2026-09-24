/*
 * Copyright 2026 caiflower Authors
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
	"sync/atomic"
	"testing"
	"time"

	"github.com/IBM/sarama"
	xkafka "github.com/caiflower/common-tools/kafka"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testTopic = "test-topic"

type consumerMockBrokerOptions struct {
	partitions       map[string][]int32
	messages         map[string]map[int32][]string
	committedOffsets map[string]map[int32]int64
}

func newConsumerMockBroker(t *testing.T) *sarama.MockBroker {
	t.Helper()
	return newConsumerMockBrokerWithOptions(t, consumerMockBrokerOptions{
		partitions: map[string][]int32{
			testTopic: {0},
		},
		messages: map[string]map[int32][]string{
			testTopic: {
				0: {"hello-consumer"},
			},
		},
	})
}

func newConsumerMockBrokerWithOptions(t *testing.T, opts consumerMockBrokerOptions) *sarama.MockBroker {
	t.Helper()
	broker, _ := newConsumerMockBrokerHarnessWithOptions(t, opts)
	return broker
}

func newConsumerMockBrokerHarnessWithOptions(t *testing.T, opts consumerMockBrokerOptions) (*sarama.MockBroker, *sarama.MockOffsetFetchResponse) {
	t.Helper()
	broker := sarama.NewMockBroker(t, 1)

	metadata := sarama.NewMockMetadataResponse(nil).
		SetBroker(broker.Addr(), broker.BrokerID())
	offsets := sarama.NewMockOffsetResponse(nil)
	offsetFetch := sarama.NewMockOffsetFetchResponse(nil)
	fetch := sarama.NewMockFetchResponse(nil, 100)
	for topic, partitions := range opts.partitions {
		for _, partition := range partitions {
			messages := opts.messages[topic][partition]
			newestOffset := int64(len(messages))

			metadata.SetLeader(topic, partition, broker.BrokerID())
			offsets.
				SetOffset(topic, partition, sarama.OffsetNewest, newestOffset).
				SetOffset(topic, partition, sarama.OffsetOldest, 0)
			committedOffset, ok := opts.committedOffsets[topic][partition]
			if !ok {
				committedOffset = 0
			}
			offsetFetch.SetOffset("test-group", topic, partition, committedOffset, "", sarama.ErrNoError)
			fetch.SetHighWaterMark(topic, partition, newestOffset)
			for offset, value := range messages {
				fetch.SetMessage(topic, partition, int64(offset), sarama.StringEncoder(value))
			}
		}
	}

	broker.SetHandlerByMap(map[string]sarama.MockResponse{
		"ApiVersionsRequest": sarama.NewMockApiVersionsResponse(nil),
		"MetadataRequest":    metadata,
		"FindCoordinatorRequest": sarama.NewMockFindCoordinatorResponse(nil).
			SetCoordinator(sarama.CoordinatorGroup, "test-group", broker),
		"JoinGroupRequest": sarama.NewMockJoinGroupResponse(nil).
			SetGroupProtocol(sarama.RangeBalanceStrategyName),
		"SyncGroupRequest": sarama.NewMockSyncGroupResponse(nil).
			SetMemberAssignment(&sarama.ConsumerGroupMemberAssignment{
				Version: 0,
				Topics:  opts.partitions,
			}),
		"HeartbeatRequest":    sarama.NewMockHeartbeatResponse(nil),
		"OffsetRequest":       offsets,
		"OffsetFetchRequest":  offsetFetch,
		"FetchRequest":        fetch,
		"OffsetCommitRequest": sarama.NewMockOffsetCommitResponse(nil),
	})

	return broker, offsetFetch
}

func newTestConsumerConfig(brokerAddr string) xkafka.Config {
	return xkafka.Config{
		Name:                      "itest-consumer",
		Enable:                    "true",
		BootstrapServers:          []string{brokerAddr},
		GroupID:                   "test-group",
		Topics:                    []string{testTopic},
		ConsumerAutoOffsetReset:   "earliest",
		ConsumerWorkerNum:         1,
		ConsumerQueueSize:         100,
		ConsumerCommitInterval:    1 * time.Second,
		ConsumerSessionTimeout:    20 * time.Second,
		ConsumerHeartBeatInterval: 6 * time.Second,
	}
}

func TestV2ConsumerGroup_ConsumeViaMockBroker(t *testing.T) {
	broker := newConsumerMockBroker(t)
	defer broker.Close()

	cfg := newTestConsumerConfig(broker.Addr())

	config := sarama.NewConfig()
	config.Consumer.Group.Session.Timeout = cfg.ConsumerSessionTimeout
	config.Consumer.Group.Heartbeat.Interval = cfg.ConsumerHeartBeatInterval
	config.Consumer.MaxProcessingTime = 500 * time.Millisecond
	config.Consumer.Return.Errors = true
	config.Consumer.Offsets.AutoCommit.Enable = false
	config.Consumer.Offsets.Initial = sarama.OffsetOldest
	config.Version = sarama.V2_6_0_0

	group, err := sarama.NewConsumerGroup(cfg.BootstrapServers, cfg.GroupID, config)
	require.NoError(t, err)
	defer group.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var receivedMsg string
	handler := &testConsumerGroupHandler{
		handler: func(msg *sarama.ConsumerMessage) error {
			receivedMsg = string(msg.Value)
			cancel()
			return nil
		},
		ready: make(chan struct{}),
	}

	go func() {
		for {
			if err := group.Consume(ctx, cfg.Topics, handler); err != nil {
				return
			}
			if ctx.Err() != nil {
				return
			}
		}
	}()

	<-handler.ready
	<-ctx.Done()

	assert.Equal(t, "hello-consumer", receivedMsg)
}

func TestV2Consumer_RetryExhaustedThenDeadLetter(t *testing.T) {
	cfg := xkafka.Config{
		Name:                   "itest-retry-consumer",
		Enable:                 "true",
		BootstrapServers:       []string{"localhost:9092"},
		GroupID:                "test-group",
		ConsumerRetryCount:     2,
		ConsumerWorkerNum:      1,
		ConsumerQueueSize:      10,
		ConsumerCommitInterval: 1 * time.Second,
	}

	client := &KafkaClient{
		cfg:       &cfg,
		msgChan:   make(chan *msgItem, cfg.ConsumerQueueSize),
		closeChan: make(chan struct{}, cfg.ConsumerWorkerNum),
	}
	ctx, cancel := context.WithCancel(context.Background())
	client.ctx = ctx
	client.cancelFunc = cancel
	client.running.Store(true)

	var callCount int32
	var dlMsg string
	var dlErr error
	dlHandler := func(message interface{}, err error) error {
		atomic.AddInt32(&callCount, 1)
		dlMsg = string(message.(*sarama.ConsumerMessage).Value)
		dlErr = err
		cancel()
		return nil
	}

	go client.consume(func(message interface{}) error {
		return assert.AnError
	}, dlHandler)

	client.msgChan <- &msgItem{
		msg: &sarama.ConsumerMessage{
			Topic:     testTopic,
			Partition: 0,
			Offset:    0,
			Value:     []byte("retry-msg"),
		},
	}

	<-ctx.Done()

	assert.Equal(t, "retry-msg", dlMsg)
	assert.Error(t, dlErr)
	assert.Equal(t, int32(1), atomic.LoadInt32(&callCount), "dead letter handler should be called exactly once")
}

func TestV2Consumer_RetrySuccessOnSecondAttempt(t *testing.T) {
	cfg := xkafka.Config{
		Name:                   "itest-retry-success-consumer",
		Enable:                 "true",
		BootstrapServers:       []string{"localhost:9092"},
		GroupID:                "test-group",
		ConsumerRetryCount:     3,
		ConsumerWorkerNum:      1,
		ConsumerQueueSize:      10,
		ConsumerCommitInterval: 1 * time.Second,
	}

	client := &KafkaClient{
		cfg:       &cfg,
		msgChan:   make(chan *msgItem, cfg.ConsumerQueueSize),
		closeChan: make(chan struct{}, cfg.ConsumerWorkerNum),
	}
	ctx, cancel := context.WithCancel(context.Background())
	client.ctx = ctx
	client.cancelFunc = cancel
	client.running.Store(true)

	var callCount int32
	var successMsg string

	go client.consume(func(message interface{}) error {
		count := atomic.AddInt32(&callCount, 1)
		if count <= 1 {
			return assert.AnError
		}
		successMsg = string(message.(*sarama.ConsumerMessage).Value)
		cancel()
		return nil
	})

	client.msgChan <- &msgItem{
		msg: &sarama.ConsumerMessage{
			Topic:     testTopic,
			Partition: 0,
			Offset:    0,
			Value:     []byte("retry-then-success"),
		},
	}

	<-ctx.Done()

	assert.Equal(t, "retry-then-success", successMsg)
	assert.True(t, atomic.LoadInt32(&callCount) >= 2, "expected at least 2 calls, got %d", callCount)
}

type testConsumerGroupHandler struct {
	handler func(msg *sarama.ConsumerMessage) error
	ready   chan struct{}
}

func (h *testConsumerGroupHandler) Setup(session sarama.ConsumerGroupSession) error {
	close(h.ready)
	return nil
}

func (h *testConsumerGroupHandler) Cleanup(session sarama.ConsumerGroupSession) error {
	return nil
}

func (h *testConsumerGroupHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for msg := range claim.Messages() {
		if h.handler != nil {
			_ = h.handler(msg)
		}
		session.MarkMessage(msg, "")
	}
	return nil
}
