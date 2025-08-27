package v1

import (
	"context"
	"sync"

	xkafka "github.com/caiflower/common-tools/kafka"
	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
)

/*
 * 基于github.com/confluentinc/confluent-kafka-go实现的kafka client
 * 1、生产者支持异步和同步发送
 * 2、消费者支持设置consumer_worker_num, 消费者数量
 */

type KafkaClient struct {
	lock   sync.Locker
	config *xkafka.Config
	ctx    context.Context
	cancel context.CancelFunc

	Consumer *kafka.Consumer
	fn       func(message interface{})

	Producer *kafka.Producer

	running bool
}

func (c *KafkaClient) Close() {
	c.lock.Lock()
	defer c.lock.Unlock()
	if c.Consumer != nil {
		err := c.Consumer.Close()
		if err != nil {
			logger.Warn("[kafka-consumer] close kafka failed. Error: %s", err.Error())
		}
		c.Consumer = nil
	}
	if c.Producer != nil {
		for c.Producer.Flush(10000) > 0 {
			logger.Info("[kafka-product] waiting flush message")
		}
		c.Producer.Close()
		c.Producer = nil
	}
	if c.cancel != nil {
		c.cancel()
		c.cancel = nil
	}
	c.running = false
}
