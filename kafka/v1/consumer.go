package v1

import (
	"context"
	"reflect"
	"strings"
	"time"

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
		logger.Warn("Kafka consumer %s set default config failed. err: %s", config.Name, err.Error())
	}

	ctx, cancelFunc := context.WithCancel(context.Background())
	kafkaClient := &KafkaClient{config: config, lock: syncx.NewSpinLock(), ctx: ctx, cancel: cancelFunc}
	if !config.Enable {
		logger.Warn("kafka consumer %s is disable", config.Name)
		return kafkaClient
	} else {
		logger.Info("Kafka consumer %s config: %s", config.Name, tools.ToJson(config))
	}

	configMap := &kafka.ConfigMap{}
	if err := configMap.SetKey("bootstrap.servers", strings.Join(config.BootstrapServers, ",")); err != nil {
		logger.Warn("set bootstrap.servers error: %s", err.Error())
	}
	if err := configMap.SetKey("group.id", config.GroupID); err != nil {
		logger.Warn("set group.id error: %s", err.Error())
	}
	if err := configMap.SetKey("enable.auto.commit", false); err != nil {
		logger.Warn("set enable.auto.commit error: %s", err.Error())
	}
	if config.SecurityProtocol != "" {
		if err := configMap.SetKey("security.protocol", config.SecurityProtocol); err != nil {
			logger.Warn("set security.protocol error: %s", err.Error())
		}
	}
	if config.SaslMechanism != "" {
		if err := configMap.SetKey("sasl.mechanism", config.SaslMechanism); err != nil {
			logger.Warn("set sasl.mechanism error: %s", err.Error())
		}
	}
	if config.SaslUsername != "" {
		if err := configMap.SetKey("sasl.username", config.SaslUsername); err != nil {
			logger.Warn("set sasl.username error: %s", err.Error())
		}
	}
	if config.SaslPassword != "" {
		if err := configMap.SetKey("sasl.password", config.SaslPassword); err != nil {
			logger.Warn("set sasl.password error: %s", err.Error())
		}
	}

	consumer, err := kafka.NewConsumer(configMap)
	if err != nil {
		logger.Error("create kafka consumer error: %s", err.Error())
		return &KafkaClient{config: config}
	}
	err = consumer.SubscribeTopics(config.Topics, nil)
	if err != nil {
		logger.Error("subscribe topics error, %s", err.Error())
		return &KafkaClient{config: config}
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
					ev := c.Consumer.Poll(100)
					switch event := ev.(type) {
					case *kafka.Message:
						e.OnError("Kafka consumer")
						for _, fn := range c.consumerFuncList {
							fn(event)
						}
						err := c.Consumer.Assign([]kafka.TopicPartition{event.TopicPartition})
						if err != nil {
							logger.Warn("Kafka consumer assign kafka message error: %s", err.Error())
						}
					case kafka.PartitionEOF:
						logger.Warn("Kafka consumer reached EOF. error: %v", event)
					case kafka.Error:
						logger.Warn("Kafka consumer failed. error: %v", event)
					default:
						logger.Warn("Kafka consumer ignored. ev: %v", event)
					}
				}
			}
		}()
	}

	ticker := time.NewTicker(time.Second * 1)
	go func() {
		for {
			select {
			case <-c.ctx.Done():
				return
			case <-ticker.C:
				if p, err := c.Consumer.Commit(); err != nil {
					logger.Error("Kafka consumer commit error: %s. TopicPartition: %s", err.Error(), tools.ToJson(p))
				}
			default:

			}
		}
	}()
}
