package v1

import (
	"context"
	"sync"

	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
)

type Config struct {
	Name              string   `json:"name"`
	Enable            bool     `json:"enable"`
	BootstrapServers  []string `json:"bootstrap_servers"`
	GroupID           string   `json:"group_id"`
	Topics            []string `json:"topics"`
	ConsumerWorkerNum int      `json:"consumer_worker_num" default:"2"`
}

type KafkaClient struct {
	lock   sync.Locker
	config Config
	ctx    context.Context
	cancel context.CancelFunc

	Consumer         *kafka.Consumer
	consumerFuncList []func(message interface{})

	Producer *kafka.Producer
}

func (c *KafkaClient) Close() {
	c.lock.Lock()
	defer c.lock.Unlock()
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
	if c.cancel != nil {
		c.cancel()
		c.cancel = nil
	}
}
