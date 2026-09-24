package v1

import (
	"testing"

	"github.com/caiflower/common-tools/pkg/basic"
	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRemoveRevokedPartitionQueues(t *testing.T) {
	topic := "topic"
	client := &KafkaClient{}
	for partition := int32(0); partition < 2; partition++ {
		queue := basic.NewSafeRingQueue(2)
		require.NoError(t, queue.Enqueue(&msgItem{msg: &kafka.Message{}}))
		client.msgQueue.Store(
			getTopicPartitionKey(&kafka.TopicPartition{Topic: &topic, Partition: partition}),
			queue,
		)
	}

	client.removeRevokedPartitionQueues([]kafka.TopicPartition{
		{Topic: &topic, Partition: 0},
	})

	_, revokedExists := client.msgQueue.Load("topic-0")
	_, retainedExists := client.msgQueue.Load("topic-1")
	assert.False(t, revokedExists, "revoked partition queue must be removed")
	assert.True(t, retainedExists, "retained partition queue must remain")
}
