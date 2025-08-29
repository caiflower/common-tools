package v1

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/caiflower/common-tools/global"
	xkafka "github.com/caiflower/common-tools/kafka"
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
	if err := tools.DoTagFunc(&config, nil, []func(reflect.StructField, reflect.Value, interface{}) error{tools.SetDefaultValueIfNil}); err != nil {
		logger.Warn("[kafka-consumer] consumer '%s' set default config failed. err: %s", config.Name, err.Error())
	}

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
	c.closeChan = make(chan struct{}, c.config.ConsumerWorkerNum)
	c.fn = fn

	// 开始消费
	c.doListen()
	c.monitorOffset()
}

func (c *KafkaClient) monitorOffset() {
	fn := func() {
		e.OnError("kafka consumer monitorOffset")
		var commitOffsets []kafka.TopicPartition
		c.offsets.Range(func(k, v interface{}) bool {
			tp := v.(kafka.TopicPartition)
			logger.Info("%s Commit offset [key=%s] [offset=%d]", c.config.Name, k.(string), tp.Offset)
			tp.Offset += 1
			commitOffsets = append(commitOffsets, tp)
			return true
		})
		if len(commitOffsets) > 0 {
			_, err := c.Consumer.CommitOffsets(commitOffsets)
			if err != nil {
				logger.Error("[kafka-consumer] commit offsets failed. Error: %s", err.Error())
			} else {
				c.offsets.Range(func(k, v interface{}) bool {
					c.offsets.Delete(k)
					return true
				})
			}
		}
	}
	c.commitOffsetFunc = fn
	c.monitorOffsetJob = crontab.NewRegularJob("MonitorOffset", fn, crontab.WithInterval(c.config.ConsumerCommitInterval), crontab.WithIgnorePanic(), crontab.WithImmediately())
	c.monitorOffsetJob.Run()
}

func (c *KafkaClient) doListen() {
	runThread := func(tid int) {
		defer e.OnError("")
		logger.Info("[kafka-consumer] consumer [%s-%d] started.", c.config.Name, tid)

		for {
			select {
			case <-c.ctx.Done():
				logger.Info("[kafka-consumer] consumer [%s-%d] stopped.", c.config.Name, tid)
				c.closeChan <- struct{}{}
				return
			default:
				msg, err := c.Consumer.ReadMessage(100 * time.Millisecond)
				if err == nil {
					func() {
						defer e.OnError(fmt.Sprintf("[kafka-consumer] [%s-%d] consumer listen", c.config.Name, tid))
						c.fn(msg)
					}()

					c.offsets.Store(getTopicPartitionKey(&msg.TopicPartition), msg.TopicPartition)
				} else if !err.(kafka.Error).IsTimeout() {
					logger.Error("[kafka-consumer] [%s-%d] failed. Error: %v", c.config.Name, tid, err)
					addConsumerError(c.config, "consume")
				}
			}
		}
	}

	for i := 1; i <= c.config.ConsumerWorkerNum; i++ {
		go runThread(i)
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
