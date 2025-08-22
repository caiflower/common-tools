package v1

import (
	"reflect"
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
	if err := tools.DoTagFunc(&config, nil, []func(reflect.StructField, reflect.Value, interface{}) error{tools.SetDefaultValueIfNil}); err != nil {
		logger.Warn("Kafka consumer %s set default config failed. err: %s", config.Name, err.Error())
	}

	kafkaClient := &KafkaClient{config: config, lock: syncx.NewSpinLock()}
	if !config.Enable {
		logger.Warn("kafka producer %s is disable", config.Name)
		return kafkaClient
	} else {
		logger.Info("kafka producer %s config: %s", config.Name, tools.ToJson(config))
	}

	configMap := &kafka.ConfigMap{}
	_ = configMap.SetKey("bootstrap.servers", strings.Join(config.BootstrapServers, ","))
	_ = configMap.SetKey("request.required.acks", config.ProducerAcks)
	_ = configMap.SetKey("compression.type", config.ProducerCompressType)
	_ = configMap.SetKey("message.timeout.ms", config.ProducerMessageTimeout)

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
		logger.Error("[kafka] create kafka producer failed. Error: %s", err.Error())
		return kafkaClient
	}

	// Delivery report handler for produced messages
	go func() {
		for e := range producer.Events() {
			switch ev := e.(type) {
			case *kafka.Message:
				if ev.TopicPartition.Error != nil {
					logger.Error("[kafka] producer delivery failed. Error: %v. Message %v, topic %v", ev.TopicPartition.Error, ev.Value, ev.TopicPartition.Topic)
				} else {
					logger.Debug("[kafka] producer delivery message %v to %v", ev.Value, ev.TopicPartition)
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
			err = c.Producer.Produce(message, event)
			if err != nil {
				return err
			}
		}

		for i := 0; i < len(c.config.Topics); i++ {
			e := <-event
			switch ev := e.(type) {
			case *kafka.Message:
				if ev.TopicPartition.Error != nil {
					logger.Error("[kafka]  producer delivery failed. Error: %v. Message %v, topic %v", ev.TopicPartition.Error, tools.ToJson(msg), ev.TopicPartition.Topic)
					err = ev.TopicPartition.Error
				} else {
					logger.Debug("[kafka] producer delivery message %v to %v success.", tools.ToJson(msg), ev.TopicPartition)
				}
			}
		}
	} else {
		logger.Warn("[kafka] producer %s is not init, message: %v", c.config.Name, tools.ToJson(msg))
	}

	return err
}

func (c *KafkaClient) AsyncSend(msg interface{}) error {
	if c.Producer != nil {
		for _, topic := range c.config.Topics {
			message := &kafka.Message{TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny}, Value: []byte(tools.ToJson(msg))}
			err := c.Producer.Produce(message, nil)
			if err != nil {
				return err
			}
		}
	} else {
		logger.Warn("[kafka] Producer %s is not init, message: %v", c.config.Name, tools.ToJson(msg))
	}

	return nil
}

func (c *KafkaClient) GetProducer() *kafka.Producer {
	return c.Producer
}
