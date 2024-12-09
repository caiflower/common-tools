package v1

import (
	"sync"

	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
)

type Config struct {
	Name             string
	Enable           bool
	BootstrapServers []string
	GroupID          string
	Topics           []string
}

type KafkaClient struct {
	lock   sync.Locker
	config Config

	Consumer *kafka.Consumer
	Producer *kafka.Producer
}

func (c *KafkaClient) Close() {
	if c.Consumer != nil {
		err := c.Consumer.Close()
		if err != nil {
			logger.Warn("close kafka consumer error", err.Error())
		}
		c.Consumer = nil
	}
	if c.Producer != nil {
		c.Producer.Close()
		c.Producer = nil
	}
}
