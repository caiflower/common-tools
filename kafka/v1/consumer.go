package v1

import (
	"context"
	"reflect"
	"strings"

	"github.com/caiflower/common-tools/pkg/e"
	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/caiflower/common-tools/pkg/syncx"
	"github.com/caiflower/common-tools/pkg/tools"
	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
)

type Consumer interface {
	Listen(fn func(message interface{}))
	GetConsumer() *kafka.Consumer
	Close()
}

func NewConsumerClient(config Config) Consumer {
	if err := tools.DoTagFunc(&config, nil, []func(reflect.StructField, reflect.Value, interface{}) error{tools.SetDefaultValueIfNil}); err != nil {
		logger.Warn("[kafka-consumer] consumer '%s' set default config failed. err: %s", config.Name, err.Error())
	}

	ctx, cancelFunc := context.WithCancel(context.Background())
	kafkaClient := &KafkaClient{config: config, lock: syncx.NewSpinLock(), ctx: ctx, cancel: cancelFunc}
	if !config.Enable {
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
	err = consumer.SubscribeTopics(config.Topics, rebalanceCallback)
	if err != nil {
		logger.Error("[kafka-consumer] subscribe topics error, %s", err.Error())
		return kafkaClient
	}
	kafkaClient.Consumer = consumer

	// 开始消费
	kafkaClient.doListen()

	return kafkaClient
}

func (c *KafkaClient) GetConsumer() *kafka.Consumer {
	return c.Consumer
}

func (c *KafkaClient) Listen(fn func(message interface{})) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.consumerFuncList = append(c.consumerFuncList, fn)
}

func (c *KafkaClient) doListen() {
	for i := 0; i < c.config.ConsumerWorkerNum; i++ {
		go func() {
			for {
				select {
				case <-c.ctx.Done():
					return
				default:
					msg, err := c.Consumer.ReadMessage(-1)
					if err == nil {
						func() {
							defer e.OnError("kafka consumer listen")
							for _, fn := range c.consumerFuncList {
								fn(msg.Value)
							}

							if _, err = c.Consumer.CommitMessage(msg); err != nil {
								logger.Error("[kafka-consumer] consumer commit message failed. Error: %s. TopicPartition: %s", err.Error(), tools.ToJson(msg))
							}
						}()
					} else if !err.(kafka.Error).IsTimeout() {
						logger.Error("[kafka-consumer] consumer failed. Error: %v (%v)\n", err, msg)
					}
				}
			}
		}()
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
func rebalanceCallback(c *kafka.Consumer, event kafka.Event) error {
	switch ev := event.(type) {
	case kafka.AssignedPartitions:
		logger.Info("%s rebalance: %d new partition(s) assigned: %v",
			c.GetRebalanceProtocol(), len(ev.Partitions), ev.Partitions)

		// The application may update the start .Offset of each assigned
		// partition and then call Assign(). It is optional to call Assign
		// in case the application is not modifying any start .Offsets. In
		// that case we don't, the library takes care of it.
		// It is called here despite not modifying any .Offsets for illustrative
		// purposes.
		err := c.Assign(ev.Partitions)
		if err != nil {
			return err
		}

	case kafka.RevokedPartitions:
		logger.Info("%s rebalance: %d partition(s) revoked: %v",
			c.GetRebalanceProtocol(), len(ev.Partitions), ev.Partitions)

		// Usually, the rebalance callback for `RevokedPartitions` is called
		// just before the partitions are revoked. We can be certain that a
		// partition being revoked is not yet owned by any other consumer.
		// This way, logic like storing any pending offsets or committing
		// offsets can be handled.
		// However, there can be cases where the assignment is lost
		// involuntarily. In this case, the partition might already be owned
		// by another consumer, and operations including committing
		// offsets may not work.
		if c.AssignmentLost() {
			// Our consumer has been kicked out of the group and the
			// entire assignment is thus lost.
			logger.Warn("Assignment lost involuntarily, commit may fail")
		}

		// Since enable.auto.commit is unset, we need to commit offsets manually
		// before the partition is revoked.
		//commitedOffsets, err := c.Commit()

		//if err != nil && err.(kafka.Error).Code() != kafka.ErrNoOffset {
		//	logger.Error("Failed to commit offsets: %s", err)
		//	return err
		//}
		//logger.Info("Commited offsets to kafka: %v", commitedOffsets)

		// Similar to Assign, client automatically calls Unassign() unless the
		// callback has already called that method. Here, we don't call it.

	default:
		logger.Warn("Unexpected event type: %v", event)
	}

	return nil
}
