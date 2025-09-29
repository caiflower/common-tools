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
	"context"
	"fmt"
	"sync"

	xkafka "github.com/caiflower/common-tools/kafka"
	"github.com/caiflower/common-tools/pkg/crontab"
	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
)

/*
 * 基于github.com/confluentinc/confluent-kafka-go实现的kafka client
 * 1、生产者支持异步和同步发送
 * 2、消费者支持设置consumer_worker_num, 消费者数量
 */

type KafkaClient struct {
	lock    sync.Locker
	config  *xkafka.Config
	ctx     context.Context
	cancel  context.CancelFunc
	running bool

	Consumer         *kafka.Consumer
	fn               func(message interface{})
	offsets          sync.Map
	closeChan        chan struct{}
	commitOffsetFunc func()
	monitorOffsetJob crontab.RegularJob

	Producer *kafka.Producer
}

func getTopicPartitionKey(topicPartition *kafka.TopicPartition) string {
	var topic string
	if topicPartition.Topic != nil {
		topic = *topicPartition.Topic
	}
	return fmt.Sprintf("%s-%d", topic, topicPartition.Partition)
}

func (c *KafkaClient) Close() {
	c.lock.Lock()
	defer c.lock.Unlock()

	logger.Info("[kafka-consumer-close] name='%s'", c.config.Name)

	if c.cancel != nil {
		c.cancel()
		c.cancel = nil
		for i := 1; i <= c.config.ConsumerWorkerNum; i++ {
			<-c.closeChan
		}
		c.closeChan = nil
	}

	if c.monitorOffsetJob != nil {
		c.monitorOffsetJob.Stop()
		c.monitorOffsetJob = nil
	}

	if c.Consumer != nil {
		err := c.Consumer.Close()
		if err != nil {
			logger.Warn("[kafka-consumer-close] close kafka failed. Error: %s", err.Error())
		}
		c.Consumer = nil
	}
	if c.Producer != nil {
		for c.Producer.Flush(10000) > 0 {
			logger.Info("[kafka-consumer-close] waiting flush message")
		}
		c.Producer.Close()
		c.Producer = nil
	}
	c.running = false
}
