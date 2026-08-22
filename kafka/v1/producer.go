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

package v1

import (
	"errors"
	"strings"

	"github.com/caiflower/common-tools/global"
	xkafka "github.com/caiflower/common-tools/kafka"
	"github.com/caiflower/common-tools/pkg/e"
	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/caiflower/common-tools/pkg/syncx"
	"github.com/caiflower/common-tools/pkg/tools"
	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
)

type Producer interface {
	xkafka.Producer
	GetProducer() *kafka.Producer
}

var _ xkafka.Producer = (*KafkaClient)(nil)

func NewProducerClient(config xkafka.Config) *KafkaClient {
	_ = tools.DoTagFunc(&config, []tools.FnObj{{Fn: tools.SetDefaultValueIfNil}})

	kafkaClient := &KafkaClient{config: &config, lock: syncx.NewSpinLock()}
	if strings.ToUpper(config.Enable) != "TRUE" {
		logger.Warn("[kafka-product] producer '%s' is disable", config.Name)
		return kafkaClient
	} else {
		logger.Info("[kafka-product] producer '%s' config: %s", config.Name, tools.ToJson(config))
	}

	configMap := &kafka.ConfigMap{}
	_ = configMap.SetKey("bootstrap.servers", strings.Join(config.BootstrapServers, ","))
	_ = configMap.SetKey("request.required.acks", config.ProducerAcks)
	_ = configMap.SetKey("request.timeout.ms", int(config.ProducerRequestTimeout.Milliseconds()))
	_ = configMap.SetKey("compression.type", config.ProducerCompressType)
	_ = configMap.SetKey("message.timeout.ms", int(config.ProducerMessageTimeout.Milliseconds()))
	_ = configMap.SetKey("retries", 3)
	_ = configMap.SetKey("retry.backoff.ms", 200)
	_ = configMap.SetKey("max.poll.interval.ms", 180000) // 3分钟

	if config.ProducerIdempotence {
		// librdkafka automatically sets max.in.flight.requests.per.connection=5 when
		// enable.idempotence=true (its idempotent producer supports 5 in-flight requests),
		// so we don't need to set it manually unlike sarama which requires MaxOpenRequests=1.
		_ = configMap.SetKey("enable.idempotence", true)
		if config.ProducerAcks != -1 {
			logger.Warn("[kafka-product] producer '%s' enable.idempotence requires acks=-1, overriding to -1", config.Name)
			_ = configMap.SetKey("request.required.acks", -1)
		}
	}

	if config.SecurityProtocol != "" {
		_ = configMap.SetKey("security.protocol", config.SecurityProtocol)
	}
	if config.SaslMechanism != "" {
		_ = configMap.SetKey("sasl.mechanism", config.SaslMechanism)
	}
	if config.SaslUsername != "" {
		_ = configMap.SetKey("sasl.username", config.SaslUsername)
	}
	if config.SaslPassword != "" {
		_ = configMap.SetKey("sasl.password", config.SaslPassword)
	}
	if config.SSLCaFile != "" {
		_ = configMap.SetKey("ssl.ca.location", config.SSLCaFile)
	}
	if config.SSLCertFile != "" {
		_ = configMap.SetKey("ssl.certificate.location", config.SSLCertFile)
	}
	if config.SSLKeyFile != "" {
		_ = configMap.SetKey("ssl.key.location", config.SSLKeyFile)
	}

	producer, err := kafka.NewProducer(configMap)
	if err != nil {
		logger.Error("[kafka-product] create kafka producer failed. Error: %s", err.Error())
		return kafkaClient
	}

	// Delivery report handler for produced messages
	go func() {
		defer e.OnError("")

		for event := range producer.Events() {
			switch ev := event.(type) {
			case *kafka.Message:
				ev.Opaque = finishProducerMessage(ev.Opaque, ev.TopicPartition.Error)
				if ev.TopicPartition.Error != nil {
					xkafka.AddProducerErrCount(kafkaClient.config, *ev.TopicPartition.Topic, xkafka.AsyncErr)
					logger.Error("[kafka-product]  producer delivery failed. Error: %v. topic %v", ev.TopicPartition.Error, ev.TopicPartition.Topic)
				} else {
					logger.Debug("[kafka-product] producer message [key=%s] to %v success", getTopicPartitionKey(&ev.TopicPartition), ev.TopicPartition.Offset)
				}
			}
		}
	}()

	kafkaClient.Producer = producer
	kafkaClient.running.Store(true)

	global.DefaultResourceManger.Add(kafkaClient)
	return kafkaClient
}

// AddHook registers a producer hook on this client.
func (c *KafkaClient) AddHook(hook xkafka.ProducerHook) {
	if hook == nil {
		return
	}
	c.hooksMu.Lock()
	defer c.hooksMu.Unlock()
	c.producerHooks = append(c.producerHooks, hook)
}

func (c *KafkaClient) producerHooksSnapshot() []xkafka.ProducerHook {
	c.hooksMu.RLock()
	defer c.hooksMu.RUnlock()
	return append([]xkafka.ProducerHook(nil), c.producerHooks...)
}

func (c *KafkaClient) Send(topic string, key string, values ...interface{}) error {
	if strings.ToUpper(c.config.Enable) != "TRUE" || len(values) == 0 {
		return nil
	}

	if topic == "" {
		return errors.New("sync producer cannot send message without topic")
	}

	return c.sendMessages(topic, key, values, c.Producer.Produce)
}

type produceFunc func(message *kafka.Message, deliveryChan chan kafka.Event) error

func (c *KafkaClient) sendMessages(topic string, key string, values []interface{}, produce produceFunc) error {
	var err error
	event := make(chan kafka.Event, len(values))
	defer close(event)
	hooks := c.producerHooksSnapshot()
	contexts := beforeSend(hooks, topic, key, values)
	defer func() {
		completeProducerContexts(contexts, err)
	}()

	// Keep sending the rest of the batch after an enqueue failure and report
	// every error instead of returning at the first one.
	success := 0
	for _, value := range values {
		xkafka.CountProducer(c.config, topic)
		message := &kafka.Message{TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny}, Key: []byte(key), Value: []byte(tools.ToJson(value))}
		produceErr := produce(message, event)
		if produceErr != nil {
			messageDoneProducerContexts(contexts, produceErr)
			xkafka.AddProducerErrCount(c.config, topic, xkafka.SyncErr)
			logger.Errorf("[kafka-product] send message failed. error: %v", produceErr)
			err = errors.Join(err, produceErr)
		} else {
			success++
		}
	}

	for i := 0; i < success; i++ {
		evt := <-event
		switch ev := evt.(type) {
		case *kafka.Message:
			messageDoneProducerContexts(contexts, ev.TopicPartition.Error)
			if ev.TopicPartition.Error != nil {
				logger.Error("[kafka-product]  producer delivery failed. Error: %v. topic %v", ev.TopicPartition.Error, ev.TopicPartition.Topic)
				err = errors.Join(err, ev.TopicPartition.Error)
				xkafka.AddProducerErrCount(c.config, topic, xkafka.SyncErr)
			} else {
				logger.Debug("[kafka-product] producer message [key=%s] to %v success", getTopicPartitionKey(&ev.TopicPartition), ev.TopicPartition.Offset)
			}
		}
	}

	return err
}

func (c *KafkaClient) AsyncSend(topic string, key string, values ...interface{}) error {
	if strings.ToUpper(c.config.Enable) != "TRUE" || len(values) == 0 {
		return nil
	}

	if topic == "" {
		return errors.New("async producer cannot send message without topic")
	}

	return c.asyncSendMessages(topic, key, values, c.Producer.Produce)
}

func (c *KafkaClient) asyncSendMessages(topic string, key string, values []interface{}, produce produceFunc) error {
	hooks := c.producerHooksSnapshot()
	contexts := beforeSend(hooks, topic, key, values)
	var err error
	defer func() {
		completeProducerContexts(contexts, err)
	}()

	// Keep sending the rest of the batch after an enqueue failure and report
	// every error instead of returning at the first one.
	for _, value := range values {
		xkafka.CountProducer(c.config, topic)
		message := &kafka.Message{TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny}, Key: []byte(key), Value: []byte(tools.ToJson(value))}
		message.Opaque = wrapProducerMessage(contexts, message.Opaque)
		produceErr := produce(message, nil)
		if produceErr != nil {
			messageDoneProducerContexts(contexts, produceErr)
			xkafka.AddProducerErrCount(c.config, topic, xkafka.AsyncErr)
			logger.Errorf("[kafka-product] async send message failed. error: %v", produceErr)
			err = errors.Join(err, produceErr)
		}
	}

	return err
}

func (c *KafkaClient) GetProducer() *kafka.Producer {
	return c.Producer
}

func beforeSend(hooks []xkafka.ProducerHook, topic, key string, values []interface{}) []xkafka.ProducerContext {
	contexts := make([]xkafka.ProducerContext, len(hooks))
	for i, hook := range hooks {
		if hook != nil {
			contexts[i] = hook.BeforeSend(topic, key, values)
		}
	}
	return contexts
}

type producerMessageMeta struct {
	contexts []xkafka.ProducerContext
	value    interface{}
}

func wrapProducerMessage(contexts []xkafka.ProducerContext, userValue interface{}) interface{} {
	if len(contexts) == 0 {
		return userValue
	}
	return &producerMessageMeta{contexts: contexts, value: userValue}
}

func finishProducerMessage(meta interface{}, err error) interface{} {
	message, ok := meta.(*producerMessageMeta)
	if !ok {
		return meta
	}
	for _, ctx := range message.contexts {
		if ctx != nil {
			ctx.MessageDone(err)
		}
	}
	return message.value
}

func completeProducerContexts(contexts []xkafka.ProducerContext, err error) {
	for _, ctx := range contexts {
		if ctx != nil {
			ctx.Complete(err)
		}
	}
}

func messageDoneProducerContexts(contexts []xkafka.ProducerContext, err error) {
	for _, ctx := range contexts {
		if ctx != nil {
			ctx.MessageDone(err)
		}
	}
}
