package v2

import (
	"context"
	"testing"

	"github.com/IBM/sarama"
	xkafka "github.com/caiflower/common-tools/kafka"
	"github.com/caiflower/common-tools/pkg/basic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCleanupRemovesRevokedPartitionQueues(t *testing.T) {
	client := &KafkaClient{cfg: &xkafka.Config{}}
	for _, key := range []string{"topic-0", "topic-1"} {
		queue := basic.NewSafeRingQueue(2)
		require.NoError(t, queue.Enqueue(&msgItem{
			msg: &sarama.ConsumerMessage{Topic: "topic", Partition: int32(key[len(key)-1] - '0')},
		}))
		client.msgQueue.Store(key, queue)
	}
	handler := &consumerGroupHandler{KafkaClient: client}
	session := &testConsumerGroupSession{
		ctx:    context.Background(),
		claims: map[string][]int32{"topic": {0}},
	}

	require.NoError(t, handler.Cleanup(session))

	_, revokedExists := client.msgQueue.Load("topic-0")
	_, retainedExists := client.msgQueue.Load("topic-1")
	assert.False(t, revokedExists, "revoked partition queue must be removed")
	assert.True(t, retainedExists, "retained partition queue must remain")
}
