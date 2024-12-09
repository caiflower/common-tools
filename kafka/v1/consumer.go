package v1

import (
	"strings"

	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/caiflower/common-tools/pkg/syncx"
	"github.com/caiflower/common-tools/pkg/tools"
	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
)

type Consumer interface {
	Listen()
	GetConsumer() *kafka.Consumer
	Close()
}

func NewConsumerClient(config Config) Consumer {
	kafkaClient := &KafkaClient{config: config, lock: syncx.NewSpinLock()}
	if !config.Enable {
		logger.Warn("kafka Consumer %s is disable", config.Name)
		return kafkaClient
	} else {
		logger.Info("Kafka Consumer config: %v", config.Name, tools.ToJson(config))
	}

	configMap := &kafka.ConfigMap{}
	if err := configMap.SetKey("bootstrap.servers", strings.Join(config.BootstrapServers, ",")); err != nil {
		logger.Warn("set bootstrap.servers error", err.Error())
	}
	if err := configMap.SetKey("group.id", config.GroupID); err != nil {
		logger.Warn("set group.id error", err.Error())
	}
	if err := configMap.SetKey("enable.auto.commit", false); err != nil {
		logger.Warn("set enable.auto.commit error", err.Error())
	}

	consumer, err := kafka.NewConsumer(configMap)
	if err != nil {
		logger.Error("create kafka consumer error", err.Error())
		return &KafkaClient{config: config}
	}
	err = consumer.SubscribeTopics(config.Topics, nil)
	if err != nil {
		logger.Error("subscribe topics error", err.Error())
		return &KafkaClient{config: config}
	}
	kafkaClient.Consumer = consumer

	return kafkaClient
}

func (c *KafkaClient) GetConsumer() *kafka.Consumer {
	return c.Consumer
}

func (c *KafkaClient) Listen() {

}
