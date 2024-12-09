package v1

import (
	"strings"

	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/caiflower/common-tools/pkg/syncx"
	"github.com/caiflower/common-tools/pkg/tools"
	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
)

type Producer interface {
	Send(interface{}) error
	AsyncSend(interface{}) error
	GetProducer() *kafka.Producer
	Close()
}

func NewProducerClient(config Config) Producer {
	kafkaClient := &KafkaClient{config: config, lock: syncx.NewSpinLock()}
	if !config.Enable {
		logger.Warn("kafka Producer %s is disable", config.Name)
		return kafkaClient
	} else {
		logger.Info("Kafka Producer %s config: %v", config.Name, tools.ToJson(config))
	}

	configMap := &kafka.ConfigMap{}
	if err := configMap.SetKey("bootstrap.servers", strings.Join(config.BootstrapServers, ",")); err != nil {
		logger.Warn("set bootstrap.servers error", err.Error())
	}
	if err := configMap.SetKey("group.id", config.GroupID); err != nil {
		logger.Warn("set group.id error", err.Error())
	}

	producer, err := kafka.NewProducer(configMap)
	if err != nil {
		logger.Error("create kafka consumer error", err.Error())
		return kafkaClient
	}

	// Delivery report handler for produced messages
	go func() {
		for e := range producer.Events() {
			switch ev := e.(type) {
			case *kafka.Message:
				if ev.TopicPartition.Error != nil {
					logger.Error("Delivery failed. error: %v. Message %v, topic %v", ev.TopicPartition.Error, ev.Value, ev.TopicPartition.Topic)
				} else {
					logger.Info("Delivered message %v to %v", ev.Value, ev.TopicPartition)
				}
			}
		}
	}()

	kafkaClient.Producer = producer
	return kafkaClient
}

func (c *KafkaClient) Send(msg interface{}) error {
	var err error
	if c.Producer != nil {
		event := make(chan kafka.Event, len(c.config.Topics))
		for _, topic := range c.config.Topics {
			message := &kafka.Message{TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny}, Value: []byte(tools.ToJson(msg))}
			err := c.Producer.Produce(message, event)
			if err != nil {
				return err
			}
		}

		for i := 0; i < len(c.config.Topics); i++ {
			e := <-event
			switch ev := e.(type) {
			case *kafka.Message:
				if ev.TopicPartition.Error != nil {
					logger.Error("Delivery failed. error: %v. Message %v, topic %v", ev.TopicPartition.Error, ev.Value, ev.TopicPartition.Topic)
					err = ev.TopicPartition.Error
				} else {
					logger.Info("Delivered message %v to %v", ev.Value, ev.TopicPartition)
				}
			}
		}
	} else {
		logger.Warn("Kafka Producer %s is not init, message: %v", c.config.Name, tools.ToJson(msg))
	}

	return err
}

func (c *KafkaClient) AsyncSend(msg interface{}) error {
	if c.Producer != nil {
		event := make(chan kafka.Event, len(c.config.Topics))
		for _, topic := range c.config.Topics {
			message := &kafka.Message{TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny}, Value: []byte(tools.ToJson(msg))}
			err := c.Producer.Produce(message, event)
			if err != nil {
				return err
			}
		}
	} else {
		logger.Warn("Kafka Producer %s is not init, message: %v", c.config.Name, tools.ToJson(msg))
	}

	return nil
}

func (c *KafkaClient) GetProducer() *kafka.Producer {
	return c.Producer
}
