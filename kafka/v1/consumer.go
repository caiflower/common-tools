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
		logger.Warn("[kafka-consumer] consumer %s set default config failed. err: %s", config.Name, err.Error())
	}

	ctx, cancelFunc := context.WithCancel(context.Background())
	kafkaClient := &KafkaClient{config: config, lock: syncx.NewSpinLock(), ctx: ctx, cancel: cancelFunc}
	if !config.Enable {
		logger.Warn("[kafka-consumer] consumer %s is disable", config.Name)
		return kafkaClient
	} else {
		logger.Info("[kafka-consumer] consumer %s config: %s", config.Name, tools.ToJson(config))
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
	err = consumer.SubscribeTopics(config.Topics, nil)
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
