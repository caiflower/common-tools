package v2

import (
	"errors"
	"testing"

	"github.com/IBM/sarama"
	xkafka "github.com/caiflower/common-tools/kafka"
	"github.com/stretchr/testify/assert"
)

func TestHandleAsyncProducerErrorIgnoresNilError(t *testing.T) {
	handleAsyncProducerError(&xkafka.Config{}, nil)
}

func TestHandleAsyncProducerErrorIgnoresNilMessage(t *testing.T) {
	handleAsyncProducerError(&xkafka.Config{}, &sarama.ProducerError{})
}

// 同步生产的聚合错误必须展开：sarama 的 "kafka: Failed to deliver N messages." 不带原因，
// 真正的原因在每条 ProducerError.Err 里（ISR 不足 / topic 不存在 / 无权限 ...）。
func TestDescribeSyncProducerErrorUnwrapsProducerErrors(t *testing.T) {
	msgs := []*sarama.ProducerMessage{
		{Topic: "dev_llm_opc_event_topic", Partition: 2, Offset: -1},
		{Topic: "dev_llm_opc_event_topic", Partition: 5, Offset: -1},
	}
	err := sarama.ProducerErrors{
		{Msg: msgs[0], Err: sarama.ErrNotEnoughReplicas},
		{Msg: msgs[1], Err: sarama.ErrUnknownTopicOrPartition},
	}

	lines := describeSyncProducerError("fallback-topic", err)
	assert.Len(t, lines, 2)
	assert.Contains(t, lines[0], "topic=dev_llm_opc_event_topic")
	assert.Contains(t, lines[0], "partition=2")
	assert.Contains(t, lines[0], "fewer in-sync replicas than required")
	assert.Contains(t, lines[1], "partition=5")
	assert.Contains(t, lines[1], "topic or partition that does not exist")
}

func TestDescribeSyncProducerErrorFallsBackForPlainError(t *testing.T) {
	assert.Equal(t, []string{"topic=t err=boom"}, describeSyncProducerError("t", errors.New("boom")))
	assert.Empty(t, describeSyncProducerError("t", nil))

	// 逐条都为空时回退成聚合错误，保证不会「什么都不打」。
	err := sarama.ProducerErrors{nil}
	assert.Len(t, describeSyncProducerError("t", err), 1)
}
