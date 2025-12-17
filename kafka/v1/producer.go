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
	"strconv"
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
	_ = configMap.SetKey("request.timeout.ms", strconv.Itoa(config.ProducerRequestTimeout))
	_ = configMap.SetKey("compression.type", config.ProducerCompressType)
	_ = configMap.SetKey("message.timeout.ms", config.ProducerMessageTimeout)
	_ = configMap.SetKey("retries", 3)
	_ = configMap.SetKey("retry.backoff.ms", 200)
	_ = configMap.SetKey("max.poll.interval.ms", 180000) // 3分钟

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
	kafkaClient.running = true

	global.DefaultResourceManger.Add(kafkaClient)
	return kafkaClient
}

func (c *KafkaClient) Send(topic string, key string, values ...interface{}) error {
	if strings.ToUpper(c.config.Enable) != "TRUE" || len(values) == 0 {
		return nil
	}

	if topic == "" {
		return errors.New("sync producer cannot send message without topic")
	}

	var err error
	event := make(chan kafka.Event, len(values))

	for _, value := range values {
		xkafka.CountProducer(c.config, topic)
		message := &kafka.Message{TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny}, Key: []byte(key), Value: []byte(tools.ToJson(value))}
		err = c.Producer.Produce(message, event)
		if err != nil {
			xkafka.AddProducerErrCount(c.config, topic, xkafka.SyncErr)
			return err
		}
	}

	for i := 0; i < len(values); i++ {
		evt := <-event
		switch ev := evt.(type) {
		case *kafka.Message:
			if ev.TopicPartition.Error != nil {
				logger.Error("[kafka-product]  producer delivery failed. Error: %v. topic %v", ev.TopicPartition.Error, ev.TopicPartition.Topic)
				err = ev.TopicPartition.Error
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

	for _, value := range values {
		xkafka.CountProducer(c.config, topic)
		message := &kafka.Message{TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny}, Key: []byte(key), Value: []byte(tools.ToJson(value))}
		err := c.Producer.Produce(message, nil)
		if err != nil {
			xkafka.AddProducerErrCount(c.config, topic, xkafka.AsyncErr)
			return err
		}
	}

	return nil
}

func (c *KafkaClient) GetProducer() *kafka.Producer {
	return c.Producer
}
