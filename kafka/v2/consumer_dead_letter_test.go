package v2

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/IBM/sarama"
	xkafka "github.com/caiflower/common-tools/kafka"
	"github.com/stretchr/testify/assert"
)

func TestProcessMessageRetriesDeadLetter(t *testing.T) {
	cfg := &xkafka.Config{
		ConsumerRetryCount:      1,
		DeadLetterRetryCount:    2,
		DeadLetterRetryInterval: time.Millisecond,
	}
	client := &KafkaClient{cfg: cfg, ctx: context.Background()}
	item := &msgItem{msg: &sarama.ConsumerMessage{Topic: "topic", Partition: 1, Offset: 2}}
	var businessCalls int32
	var deadLetterCalls int32

	done := client.processMessage(item, func(interface{}) error {
		atomic.AddInt32(&businessCalls, 1)
		return errors.New("business failed")
	}, func(interface{}, error) error {
		if atomic.AddInt32(&deadLetterCalls, 1) == 1 {
			return errors.New("dlq failed")
		}
		return nil
	})

	assert.True(t, done)
	assert.Equal(t, int32(1), atomic.LoadInt32(&businessCalls))
	assert.Equal(t, int32(2), atomic.LoadInt32(&deadLetterCalls))
}

func TestProcessMessageDeadLetterExhaustionLeavesPending(t *testing.T) {
	cfg := &xkafka.Config{
		ConsumerRetryCount:      1,
		DeadLetterRetryCount:    2,
		DeadLetterRetryInterval: time.Millisecond,
	}
	client := &KafkaClient{cfg: cfg, ctx: context.Background()}
	item := &msgItem{msg: &sarama.ConsumerMessage{Topic: "topic", Partition: 1, Offset: 2}}
	var deadLetterCalls int32

	done := client.processMessage(item, func(interface{}) error {
		return errors.New("business failed")
	}, func(interface{}, error) error {
		atomic.AddInt32(&deadLetterCalls, 1)
		return errors.New("dlq failed")
	})

	assert.False(t, done)
	assert.Equal(t, int32(2), atomic.LoadInt32(&deadLetterCalls))
}

func TestProcessBatchRetriesDeadLetter(t *testing.T) {
	cfg := &xkafka.Config{
		ConsumerRetryCount:      1,
		DeadLetterRetryCount:    2,
		DeadLetterRetryInterval: time.Millisecond,
	}
	client := &KafkaClient{cfg: cfg, ctx: context.Background()}
	item := newBatchItem([]*sarama.ConsumerMessage{{Topic: "topic", Partition: 1, Offset: 2}})
	var businessCalls int32
	var deadLetterCalls int32

	client.processBatch(item, func([]interface{}) error {
		atomic.AddInt32(&businessCalls, 1)
		return errors.New("business failed")
	}, func([]interface{}, error) error {
		if atomic.AddInt32(&deadLetterCalls, 1) == 1 {
			return errors.New("dlq failed")
		}
		return nil
	})

	assert.True(t, item.done.Load())
	assert.Equal(t, int32(1), atomic.LoadInt32(&businessCalls))
	assert.Equal(t, int32(2), atomic.LoadInt32(&deadLetterCalls))
}

func TestProcessBatchDeadLetterExhaustionLeavesPending(t *testing.T) {
	cfg := &xkafka.Config{
		ConsumerRetryCount:      1,
		DeadLetterRetryCount:    2,
		DeadLetterRetryInterval: time.Millisecond,
	}
	client := &KafkaClient{cfg: cfg, ctx: context.Background()}
	item := newBatchItem([]*sarama.ConsumerMessage{{Topic: "topic", Partition: 1, Offset: 2}})
	var deadLetterCalls int32

	client.processBatch(item, func([]interface{}) error {
		return errors.New("business failed")
	}, func([]interface{}, error) error {
		atomic.AddInt32(&deadLetterCalls, 1)
		return errors.New("dlq failed")
	})

	assert.False(t, item.done.Load())
	assert.Equal(t, int32(2), atomic.LoadInt32(&deadLetterCalls))
}
