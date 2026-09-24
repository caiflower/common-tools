package v1

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	xkafka "github.com/caiflower/common-tools/kafka"
	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/stretchr/testify/assert"
)

func TestProcessMessageRetriesDeadLetter(t *testing.T) {
	cfg := &xkafka.Config{
		ConsumerRetryCount:      1,
		DeadLetterRetryCount:    2,
		DeadLetterRetryInterval: time.Millisecond,
	}
	client := &KafkaClient{config: cfg, ctx: context.Background()}
	item := newTestMessageItem()
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
	client := &KafkaClient{config: cfg, ctx: context.Background()}
	item := newTestMessageItem()
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

func newTestMessageItem() *msgItem {
	topic := "topic"
	return &msgItem{
		msg: &kafka.Message{
			TopicPartition: kafka.TopicPartition{
				Topic:     &topic,
				Partition: 1,
				Offset:    2,
			},
		},
	}
}
