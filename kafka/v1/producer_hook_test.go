package v1

import (
	"errors"
	"testing"

	xkafka "github.com/caiflower/common-tools/kafka"
	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/stretchr/testify/assert"
)

type recordingProducerHook struct {
	starts       int
	completes    int
	messageDones int
	completeErr  error
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
	c.hook.completeErr = err
}

func (c *recordingProducerContext) MessageDone(err error) {
	c.hook.messageDones++
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

	messageDoneProducerContexts(contexts, errors.New("enqueue failed"))
	assert.Equal(t, 2, first.messageDones)
	assert.Equal(t, 2, second.messageDones)
}

func TestSendMessagesPartialEnqueueFailureReturnsError(t *testing.T) {
	client := &KafkaClient{config: &xkafka.Config{Enable: "true"}}
	hook := &recordingProducerHook{}
	client.AddHook(hook)

	enqueueErr := errors.New("enqueue failed")
	produceCalls := 0
	produce := func(message *kafka.Message, deliveryChan chan kafka.Event) error {
		produceCalls++
		if produceCalls == 1 {
			return enqueueErr
		}
		deliveryChan <- message
		return nil
	}

	err := client.sendMessages("orders", "", []interface{}{"a", "b"}, produce)
	assert.ErrorIs(t, err, enqueueErr)
	assert.Equal(t, 2, produceCalls)
	assert.Equal(t, 1, hook.starts)
	assert.Equal(t, 2, hook.messageDones)
	assert.Equal(t, 1, hook.completes)
	assert.ErrorIs(t, hook.completeErr, enqueueErr)
}

func TestSendMessagesAllEnqueueFailuresDoNotBlock(t *testing.T) {
	client := &KafkaClient{config: &xkafka.Config{Enable: "true"}}
	hook := &recordingProducerHook{}
	client.AddHook(hook)

	enqueueErr := errors.New("enqueue failed")
	produceCalls := 0
	produce := func(message *kafka.Message, deliveryChan chan kafka.Event) error {
		produceCalls++
		return enqueueErr
	}

	err := client.sendMessages("orders", "", []interface{}{"a", "b", "c"}, produce)
	assert.ErrorIs(t, err, enqueueErr)
	assert.Equal(t, 3, produceCalls)
	assert.Equal(t, 3, hook.messageDones)
	assert.Equal(t, 1, hook.completes)
}

func TestSendMessagesAggregatesEnqueueAndDeliveryErrors(t *testing.T) {
	client := &KafkaClient{config: &xkafka.Config{Enable: "true"}}
	hook := &recordingProducerHook{}
	client.AddHook(hook)

	enqueueErr := errors.New("enqueue failed")
	deliveryErr := errors.New("delivery failed")
	produceCalls := 0
	produce := func(message *kafka.Message, deliveryChan chan kafka.Event) error {
		produceCalls++
		if produceCalls == 1 {
			return enqueueErr
		}
		message.TopicPartition.Error = deliveryErr
		deliveryChan <- message
		return nil
	}

	err := client.sendMessages("orders", "", []interface{}{"a", "b"}, produce)
	assert.ErrorIs(t, err, enqueueErr)
	assert.ErrorIs(t, err, deliveryErr)
	assert.Equal(t, 2, hook.messageDones)
	assert.ErrorIs(t, hook.completeErr, enqueueErr)
	assert.ErrorIs(t, hook.completeErr, deliveryErr)
}

func TestAsyncSendMessagesContinuesAfterEnqueueFailure(t *testing.T) {
	client := &KafkaClient{config: &xkafka.Config{Enable: "true"}}
	hook := &recordingProducerHook{}
	client.AddHook(hook)

	enqueueErr := errors.New("enqueue failed")
	produceCalls := 0
	produce := func(message *kafka.Message, deliveryChan chan kafka.Event) error {
		produceCalls++
		if produceCalls == 1 {
			return enqueueErr
		}
		return nil
	}

	err := client.asyncSendMessages("orders", "", []interface{}{"a", "b"}, produce)
	assert.ErrorIs(t, err, enqueueErr)
	assert.Equal(t, 2, produceCalls)
	assert.Equal(t, 1, hook.starts)
	assert.Equal(t, 1, hook.messageDones)
	assert.Equal(t, 1, hook.completes)
	assert.ErrorIs(t, hook.completeErr, enqueueErr)
}

func TestAsyncSendMessagesAggregatesEnqueueErrors(t *testing.T) {
	client := &KafkaClient{config: &xkafka.Config{Enable: "true"}}
	hook := &recordingProducerHook{}
	client.AddHook(hook)

	firstErr := errors.New("first enqueue failed")
	secondErr := errors.New("second enqueue failed")
	produceCalls := 0
	produce := func(message *kafka.Message, deliveryChan chan kafka.Event) error {
		produceCalls++
		switch produceCalls {
		case 1:
			return firstErr
		case 2:
			return secondErr
		}
		return nil
	}

	err := client.asyncSendMessages("orders", "", []interface{}{"a", "b", "c"}, produce)
	assert.ErrorIs(t, err, firstErr)
	assert.ErrorIs(t, err, secondErr)
	assert.Equal(t, 3, produceCalls)
	assert.Equal(t, 2, hook.messageDones)
	assert.Equal(t, 1, hook.completes)
	assert.ErrorIs(t, hook.completeErr, firstErr)
	assert.ErrorIs(t, hook.completeErr, secondErr)
}
