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
	"strings"
	"time"

	"github.com/caiflower/common-tools/global"
	xkafka "github.com/caiflower/common-tools/kafka"
	"github.com/caiflower/common-tools/pkg/basic"
	"github.com/caiflower/common-tools/pkg/crontab"
	"github.com/caiflower/common-tools/pkg/e"
	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/caiflower/common-tools/pkg/syncx"
	"github.com/caiflower/common-tools/pkg/tools"
	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
)

type Consumer interface {
	xkafka.Consumer
	GetConsumer() *kafka.Consumer
}

type KafkaMessage = kafka.Message

func NewConsumerClient(config xkafka.Config) *KafkaClient {
	_ = tools.DoTagFunc(&config, []tools.FnObj{{Fn: tools.SetDefaultValueIfNil}})

	if config.GroupID == "" {
		panic("[kafka-consumer] consumer group id should not be empty")
	}

	kafkaClient := &KafkaClient{
		config: &config,
		lock:   syncx.NewSpinLock(),
	}
	if strings.ToUpper(config.Enable) != "TRUE" {
		logger.Warn("[kafka-consumer] consumer '%s' is disable", config.Name)
		return kafkaClient
	} else {
		logger.Info("[kafka-consumer] consumer '%s' config: %s", config.Name, tools.ToJson(config))
	}

	configMap := &kafka.ConfigMap{}
	_ = configMap.SetKey("bootstrap.servers", strings.Join(config.BootstrapServers, ","))
	_ = configMap.SetKey("group.id", config.GroupID)
	_ = configMap.SetKey("enable.auto.commit", false)
	_ = configMap.SetKey("heartbeat.interval.ms", int(config.ConsumerHeartBeatInterval.Milliseconds()))
	_ = configMap.SetKey("session.timeout.ms", int(config.ConsumerSessionTimeout.Milliseconds()))
	_ = configMap.SetKey("auto.offset.reset", config.ConsumerAutoOffsetReset)
	_ = configMap.SetKey("fetch.max.bytes", config.ConsumerFetchMaxBytes)

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

	consumer, err := kafka.NewConsumer(configMap)
	if err != nil {
		logger.Error("[kafka-consumer]  create kafka consumer Error: %s", err.Error())
		return kafkaClient
	}
	err = consumer.SubscribeTopics(config.Topics, kafkaClient.rebalanceCallback)
	if err != nil {
		logger.Error("[kafka-consumer] subscribe topics error, %s", err.Error())
		return kafkaClient
	}
	kafkaClient.Consumer = consumer

	global.DefaultResourceManger.Add(kafkaClient)

	return kafkaClient
}

func (c *KafkaClient) GetConsumer() *kafka.Consumer {
	return c.Consumer
}

func (c *KafkaClient) Listen(fn func(message interface{})) {
	c.lock.Lock()
	defer c.lock.Unlock()
	if c.running || strings.ToUpper(c.config.Enable) != "TRUE" {
		return
	}
	c.running = true

	c.ctx, c.cancel = context.WithCancel(context.Background())
	// 1 个 reader goroutine + ConsumerWorkerNum 个 worker goroutine
	c.closeChan = make(chan struct{}, 1+c.config.ConsumerWorkerNum)
	c.msgChan = make(chan *msgItem, c.config.ConsumerQueueSize)

	c.doListen(fn)
	c.monitorOffset()
}

// monitorOffset 定期扫描每个 partition 的有序队列，将队头连续已完成的消息提交 offset。
// 只有队头的消息 done=true 才推进，保证不跳过未完成的消息。
func (c *KafkaClient) monitorOffset() {
	fn := func() {
		e.OnError("kafka consumer monitorOffset")
		var commitOffsets []kafka.TopicPartition
		c.msgQueue.Range(func(key, value interface{}) bool {
			queue := value.(*basic.SafeRingQueue)
			var lastDoneMsg *kafka.Message
			for {
				head, err := queue.Peek()
				if err != nil {
					break
				}
				item := head.(*msgItem)
				if !item.done {
					break
				}
				lastDoneMsg = item.msg
				queue.Dequeue()
			}
			if lastDoneMsg != nil {
				tp := lastDoneMsg.TopicPartition
				tp.Offset++
				logger.Info("%s Commit offset [key=%s] [offset=%d]", c.config.Name, key.(string), lastDoneMsg.TopicPartition.Offset)
				commitOffsets = append(commitOffsets, tp)
			}
			return true
		})
		if len(commitOffsets) > 0 {
			_, err := c.Consumer.CommitOffsets(commitOffsets)
			if err != nil {
				logger.Error("[kafka-consumer] commit offsets failed. Error: %s", err.Error())
			}
		}
	}
	c.commitOffsetFunc = fn
	c.monitorOffsetJob = crontab.NewRegularJob("MonitorOffset", fn, crontab.WithInterval(c.config.ConsumerCommitInterval), crontab.WithIgnorePanic(), crontab.WithImmediately())
	c.monitorOffsetJob.Run()
}

func (c *KafkaClient) doListen(fn func(message interface{})) {
	// 单 goroutine 读消息，分发到 msgChan
	go func() {
		defer func() {
			c.closeChan <- struct{}{}
			logger.Info("[kafka-consumer] reader [%s] stopped.", c.config.Name)
		}()
		logger.Info("[kafka-consumer] reader [%s] started.", c.config.Name)
		for {
			select {
			case <-c.ctx.Done():
				return
			default:
				msg, err := c.Consumer.ReadMessage(100 * time.Millisecond)
				if err != nil {
					if !err.(kafka.Error).IsTimeout() {
						logger.Error("[kafka-consumer] [%s] read failed. Error: %v", c.config.Name, err)
						xkafka.AddConsumerError(c.config, xkafka.ConsumeErr)
					}
					continue
				}

				item := &msgItem{msg: msg, done: false}

				// 按 partition 维护有序队列，用于 offset 追踪
				key := getTopicPartitionKey(&msg.TopicPartition)
				queue, ok := c.msgQueue.Load(key)
				if !ok {
					queue = basic.NewSafeRingQueue(c.config.ConsumerQueueSize)
					c.msgQueue.Store(key, queue)
				}
				queue.(*basic.SafeRingQueue).BlockEnqueue(item)

				// 分发给 worker pool
				select {
				case <-c.ctx.Done():
					return
				case c.msgChan <- item:
				}
			}
		}
	}()

	// worker pool：并发处理消息，处理完标记 done=true
	runWorker := func(tid int) {
		defer func() {
			c.closeChan <- struct{}{}
			logger.Info("[kafka-consumer] worker [%s-%d] stopped.", c.config.Name, tid)
		}()
		logger.Info("[kafka-consumer] worker [%s-%d] started.", c.config.Name, tid)
		for item := range c.msgChan {
			func() {
				defer e.OnError(fmt.Sprintf("[kafka-consumer] [%s-%d] consumer listen", c.config.Name, tid))
				startTime := time.Now()
				fn(item.msg)
				xkafka.RecordConsumedDuration(time.Now().Sub(startTime).Milliseconds())
				xkafka.CountConsumer(c.config)
			}()
			item.done = true
		}
	}

	for i := 1; i <= c.config.ConsumerWorkerNum; i++ {
		go runWorker(i)
	}
}

// rebalanceCallback is called on each group rebalance to assign additional
// partitions, or remove existing partitions, from the consumer's current
// assignment.
//
// A rebalance occurs when a consumer joins or leaves a consumer group, if it
// changes the topic(s) it's subscribed to, or if there's a change in one of
// the topics it's subscribed to, for example, the total number of partitions
// increases.
//
// The application may use this optional callback to inspect the assignment,
// alter the initial start offset (the .Offset field of each assigned partition),
// and read/write offsets to commit to an alternative store outside of Kafka.
func (c *KafkaClient) rebalanceCallback(consumer *kafka.Consumer, event kafka.Event) error {
	switch ev := event.(type) {
	case kafka.AssignedPartitions:
		logger.Info("%s rebalance: %d new partition(s) assigned: %v",
			consumer.GetRebalanceProtocol(), len(ev.Partitions), ev.Partitions)

		// The application may update the start .Offset of each assigned
		// partition and then call Assign(). It is optional to call Assign
		// in case the application is not modifying any start .Offsets. In
		// that case we don't, the library takes care of it.
		// It is called here despite not modifying any .Offsets for illustrative
		// purposes.
		err := consumer.Assign(ev.Partitions)
		if err != nil {
			xkafka.AddConsumerError(c.config, xkafka.RebalanceErr)
			return err
		}

	case kafka.RevokedPartitions:
		logger.Info("%s rebalance: %d partition(s) revoked: %v",
			consumer.GetRebalanceProtocol(), len(ev.Partitions), ev.Partitions)

		// Usually, the rebalance callback for `RevokedPartitions` is called
		// just before the partitions are revoked. We can be certain that a
		// partition being revoked is not yet owned by any other consumer.
		// This way, logic like storing any pending offsets or committing
		// offsets can be handled.
		// However, there can be cases where the assignment is lost
		// involuntarily. In this case, the partition might already be owned
		// by another consumer, and operations including committing
		// offsets may not work.
		if consumer.AssignmentLost() {
			// Our consumer has been kicked out of the group and the
			// entire assignment is thus lost.
			logger.Warn("Assignment lost involuntarily, commit may fail")
		}

		// commit current offset before rebalance
		logger.Info("[rebalanceCallback] commit offset before rebalance, name='%s'", c.config.Name)
		c.commitOffsetFunc()
	default:
		logger.Warn("Unexpected event type: %v", event)
	}

	return nil
}
