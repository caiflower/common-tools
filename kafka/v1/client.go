package v1

import (
	"context"
	"sync"

	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
)

/*
 * 基于github.com/confluentinc/confluent-kafka-go实现的kafka client
 * 1、生产者支持异步和同步发送
 * 2、消费者支持设置consumer_worker_num, 消费者数量
 */

type Config struct {
	Name              string   `yaml:"name"`
	Enable            bool     `yaml:"enable"`
	BootstrapServers  []string `yaml:"bootstrap_servers"`
	GroupID           string   `yaml:"group_id"`
	Topics            []string `yaml:"topics"`
	ConsumerWorkerNum int      `yaml:"consumer_worker_num" default:"2"`
	SecurityProtocol  string   `yaml:"security_protocol"`
	SaslMechanism     string   `yaml:"sasl_mechanism"`
	SaslUsername      string   `yaml:"sasl_username"`
	SaslPassword      string   `yaml:"sasl_password"`
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
			logger.Warn("close kafka consumer error: %s", err.Error())
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
