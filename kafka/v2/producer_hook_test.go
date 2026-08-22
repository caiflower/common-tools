package v2

import (
	"errors"
	"testing"

	"github.com/IBM/sarama"
	xkafka "github.com/caiflower/common-tools/kafka"
	"github.com/stretchr/testify/assert"
)

type recordingProducerHook struct {
	starts       int
	completes    int
	messageDones int
	messageErrs  []error
}

func (h *recordingProducerHook) BeforeSend(topic, key string, values []interface{}) xkafka.ProducerContext {
	h.starts++
	return &recordingProducerContext{hook: h}
}

type recordingProducerContext struct {
	hook *recordingProducerHook
}

func (c *recordingProducerContext) Complete(err error) {
	c.hook.completes++
}

func (c *recordingProducerContext) MessageDone(err error) {
	c.hook.messageDones++
	c.hook.messageErrs = append(c.hook.messageErrs, err)
}

func TestAddHookStoresInOrder(t *testing.T) {
	client := &KafkaClient{}
	first := &recordingProducerHook{}
	second := &recordingProducerHook{}

	client.AddHook(first)
	client.AddHook(second)
	client.AddHook(nil)

	assert.Equal(t, []xkafka.ProducerHook{first, second}, client.producerHooksSnapshot())
}

func TestProducerHookMessageChain(t *testing.T) {
	first := &recordingProducerHook{}
	second := &recordingProducerHook{}
	hooks := []xkafka.ProducerHook{first, second}

	contexts := beforeSend(hooks, "orders", "key-1", []interface{}{"a", "b"})
	assert.Equal(t, 1, first.starts)
	assert.Equal(t, 1, second.starts)

	value := wrapProducerMessage(contexts, "user-metadata")
	value = finishProducerMessage(value, errors.New("delivery failed"))

	assert.Equal(t, "user-metadata", value)
	assert.Equal(t, 1, first.messageDones)
	assert.Equal(t, 1, second.messageDones)

	plainValue := wrapProducerMessage(nil, "plain")
	assert.Equal(t, "plain", plainValue)
	assert.Equal(t, "plain", finishProducerMessage(plainValue, nil))

	completeProducerContexts(contexts, nil)
	assert.Equal(t, 1, first.completes)
	assert.Equal(t, 1, second.completes)

	for _, ctx := range contexts {
		ctx.MessageDone(errors.New("enqueue failed"))
	}
	assert.Equal(t, 2, first.messageDones)
	assert.Equal(t, 2, second.messageDones)
}

func TestReportSyncMessageResults(t *testing.T) {
	first := &recordingProducerHook{}
	second := &recordingProducerHook{}
	hooks := []xkafka.ProducerHook{first, second}
	contexts := beforeSend(hooks, "orders", "", []interface{}{"a", "b"})

	msg1 := &sarama.ProducerMessage{}
	msg2 := &sarama.ProducerMessage{}
	reportSyncMessageResults(contexts, []*sarama.ProducerMessage{msg1, msg2}, sarama.ProducerErrors{
		{Msg: msg2, Err: errors.New("delivery failed")},
	})

	expected := []error{nil, errors.New("delivery failed")}
	assert.Equal(t, expected, first.messageErrs)
	assert.Equal(t, expected, second.messageErrs)
}
