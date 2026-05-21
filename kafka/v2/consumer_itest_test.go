//go:build integration

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

func newConsumerMockBroker(t *testing.T) *sarama.MockBroker {
	t.Helper()
	broker := sarama.NewMockBroker(t, 1)

	broker.SetHandlerByMap(map[string]sarama.MockResponse{
		"ApiVersionsRequest": sarama.NewMockApiVersionsResponse(nil),
		"MetadataRequest": sarama.NewMockMetadataResponse(nil).
			SetBroker(broker.Addr(), broker.BrokerID()).
			SetLeader(testTopic, 0, broker.BrokerID()),
		"FindCoordinatorRequest": sarama.NewMockFindCoordinatorResponse(nil).
			SetCoordinator(sarama.CoordinatorGroup, "test-group", broker),
		"JoinGroupRequest": sarama.NewMockJoinGroupResponse(nil).
			SetGroupProtocol(sarama.RangeBalanceStrategyName),
		"SyncGroupRequest": sarama.NewMockSyncGroupResponse(nil).
			SetMemberAssignment(&sarama.ConsumerGroupMemberAssignment{
				Version: 0,
				Topics: map[string][]int32{
					testTopic: {0},
				},
			}),
		"HeartbeatRequest": sarama.NewMockHeartbeatResponse(nil),
		"OffsetRequest": sarama.NewMockOffsetResponse(nil).
			SetOffset(testTopic, 0, sarama.OffsetNewest, 1).
			SetOffset(testTopic, 0, sarama.OffsetOldest, 0),
		"OffsetFetchRequest": sarama.NewMockOffsetFetchResponse(nil).
			SetOffset("test-group", testTopic, 0, 0, "", sarama.ErrNoError),
		"FetchRequest": sarama.NewMockSequence(
			sarama.NewMockFetchResponse(nil, 1).
				SetMessage(testTopic, 0, 0, sarama.StringEncoder("hello-consumer")),
			sarama.NewMockFetchResponse(nil, 1),
		),
		"OffsetCommitRequest": sarama.NewMockOffsetCommitResponse(nil),
	})

	return broker
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
	dlHandler := func(message interface{}, err error) {
		atomic.AddInt32(&callCount, 1)
		dlMsg = string(message.(*sarama.ConsumerMessage).Value)
		dlErr = err
		cancel()
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
