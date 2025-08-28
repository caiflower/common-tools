package v2

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/Shopify/sarama"
	"github.com/caiflower/common-tools/global"
	xkafka "github.com/caiflower/common-tools/kafka"
	"github.com/caiflower/common-tools/pkg/basic"
	"github.com/caiflower/common-tools/pkg/crontab"
	"github.com/caiflower/common-tools/pkg/e"
	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/caiflower/common-tools/pkg/tools"
)

type KafkaMessage = sarama.ConsumerMessage

func NewConsumerClient(cfg xkafka.Config) *KafkaClient {
	_ = tools.DoTagFunc(&cfg, nil, []func(reflect.StructField, reflect.Value, interface{}) error{tools.SetDefaultValueIfNil})

	if strings.ToUpper(cfg.Enable) != "TRUE" {
		logger.Warn("[kafka-consumer] consumer '%s' is disable", cfg.Name)
		return &KafkaClient{}
	}

	if cfg.GroupID == "" {
		panic("[kafka-consumer] consumer group id should not be empty")
	}

	config := sarama.NewConfig()
	config.Consumer.Group.Session.Timeout = cfg.ConsumerSessionTimeout
	config.Consumer.Group.Heartbeat.Interval = cfg.ConsumerHeartBeatInterval
	config.Consumer.MaxProcessingTime = 500 * time.Millisecond
	config.Consumer.Return.Errors = true
	config.Consumer.Offsets.AutoCommit.Enable = false
	config.Consumer.Fetch.Max = int32(cfg.ConsumerFetchMaxBytes)
	if cfg.ConsumerAutoOffsetReset == "latest" {
		config.Consumer.Offsets.Initial = sarama.OffsetNewest
	} else {
		config.Consumer.Offsets.Initial = sarama.OffsetOldest
	}

	if cfg.SaslPassword != "" {
		config.Net.SASL.Enable = true
		config.Net.SASL.Mechanism = sarama.SASLMechanism(cfg.SaslMechanism)
		config.Net.SASL.User = cfg.SaslUsername
		config.Net.SASL.Password = cfg.SaslPassword
		config.Net.SASL.AuthIdentity = sarama.SASLTypePlaintext
		switch config.Net.SASL.Mechanism {
		case sarama.SASLTypeSCRAMSHA256:
			config.Net.SASL.SCRAMClientGeneratorFunc = func() sarama.SCRAMClient { return &XDGSCRAMClient{HashGeneratorFcn: SHA256} }
		case sarama.SASLTypeSCRAMSHA512:
			config.Net.SASL.SCRAMClientGeneratorFunc = func() sarama.SCRAMClient { return &XDGSCRAMClient{HashGeneratorFcn: SHA512} }
		}
	}

	kafkaClient := &KafkaClient{
		cfg:          &cfg,
		saramaConfig: config,
	}

	logger.Info("[kafka-consumer] consumer '%s' config: %s", cfg.Name, tools.ToJson(cfg))

	global.DefaultResourceManger.Add(kafkaClient)
	return kafkaClient
}

type consumerGroupHandler struct {
	lastCommitTime time.Time
	*KafkaClient
}

type msgItem struct {
	msg  *sarama.ConsumerMessage
	done bool
}

func (h *consumerGroupHandler) Setup(session sarama.ConsumerGroupSession) error {
	logger.Info("%s", tools.ToJson(session.Claims()))
	return nil
}

func (h *consumerGroupHandler) Cleanup(session sarama.ConsumerGroupSession) error {
	return nil
}

// ConsumeClaim sarama 调度时，对应每一个 partition，会启动一个 ConsumeClaim 协程，参数 claim 就代表一个分区
func (h *consumerGroupHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	h.consumerSession = session
	for {
		select {
		case msg, ok := <-claim.Messages():
			if !ok {
				h.consumerSession = nil
				return nil
			}
			logger.Trace("Message receive event : [name=%s] [group=%s] [topic=%s] [partition=%d] [offset=%d] [msg=%s]", h.cfg.Name, h.cfg.GroupID, msg.Topic, msg.Partition, msg.Offset, string(msg.Value))

			item := &msgItem{msg: msg, done: false}

			key := fmt.Sprintf("%s-%d", msg.Topic, msg.Partition)
			select {
			case <-h.ctx.Done():
				logger.Info("[ConsumeClaim] consumer is closed. [key:%s]", key)
				return nil
			case h.msgChan <- item:
			}

			queue, ok := h.msgQueue.Load(key)
			if !ok {
				queue = basic.NewSafeRingQueue(h.cfg.ConsumerQueueSize)
				h.msgQueue.Store(key, queue)
			}
			queue.(*basic.SafeRingQueue).BlockEnqueue(item)
		case <-session.Context().Done(): //表示内部会话已关闭，这里一定要退出去，否则会导致 rebalance 超时
			h.consumerSession = nil
			return nil
		}
	}
}

func (c *KafkaClient) openConsume() {
	// 创建消费组
	var consumerGroup sarama.ConsumerGroup
	var err error
label:
	for {
		if c.running == false {
			return
		}
		c.resetRetryVersion()
		for i := 1; i <= len(retryVersions); i++ {
			c.saramaConfig.Version = c.getRetryVersion()
			if consumerGroup, err = sarama.NewConsumerGroup(c.cfg.BootstrapServers, c.cfg.GroupID, c.saramaConfig); err == nil {
				break label
			}
		}
		if err != nil {
			addConsumerError(c.cfg, "connect")
			logger.Error("%s [ConsumerGroup] open failed. Error: %v", c.cfg.Name, err)
			time.Sleep(time.Second)
		}
	}

	// 监控错误
	go func() {
		for err := range consumerGroup.Errors() {
			addConsumerError(c.cfg, "consume")
			logger.Error("%s %v", c.cfg.Name, err)
		}
	}()

	// 开始消费
	// 当发生 rebalance 时，Consume 方法会重复调用，关闭老会话，创建新会话。
	c.consumerGroup = consumerGroup
	ctx := context.Background()
	for {
		if c.running == false {
			logger.Info("[ConsumerGroup] consumer is closed, name=%s", c.cfg.Name)
			break
		}
		handler := &consumerGroupHandler{KafkaClient: c, lastCommitTime: time.Now()}
		err = consumerGroup.Consume(ctx, c.cfg.Topics, handler)
		if err != nil {
			addConsumerError(c.cfg, "balance")
			logger.Error("%s [ConsumerGroup] return error. Error: %v", c.cfg.Name, err)
			time.Sleep(time.Second)
		}
		logger.Warn("%s [ConsumerGroup] reconsume. may be cause of rebalance.", c.cfg.Name)
	}
}

func (c *KafkaClient) consume(fn func(message interface{})) {
	runThread := func(tid int) {
		logger.Debug("[kafka-consumer] [%s-%d] started.", c.cfg.Name, tid)
		for item := range c.msgChan {
			if c.running == false {
				// 不再消费直接退出
				return
			}

			func() {
				defer e.OnError(fmt.Sprintf("kafka [%s-%d] consumer listen", c.cfg.Name, tid))
				fn(item.msg)
			}()
			item.done = true
		}
		logger.Debug("[kafka-consumer] [%s-%d] Exited.", c.cfg.Name, tid)
		c.closeChan <- struct{}{}
	}
	for i := 1; i <= c.cfg.ConsumerWorkerNum; i++ {
		go runThread(i)
	}
}

func (c *KafkaClient) monitorOffset() {
	fn := func() {
		c.msgQueue.Range(func(key, value interface{}) bool {
			if msgQueue := value.(*basic.SafeRingQueue); msgQueue.Size() >= 0 {
				var lastDoneMsg *sarama.ConsumerMessage = nil

				for {
					if msg, err := msgQueue.Peek(); err == nil {
						if msg.(*msgItem).done {
							lastDoneMsg = msg.(*msgItem).msg
							_, _ = msgQueue.Dequeue()
						}
					} else {
						break
					}
				}

				// 提交过的offset的消息
				if lastDoneMsg != nil {
					logger.Info("%s Commit offset [key=%s] [offset=%d]", c.cfg.Name, key, lastDoneMsg.Offset)
					if c.consumerSession != nil {
						c.consumerSession.MarkMessage(lastDoneMsg, "")
						c.consumerSession.Commit()
					}
				}
			}
			return true
		})
	}
	c.commitOffsetFunc = fn
	c.monitorOffsetJob = crontab.NewRegularJob("MonitorOffset", fn, crontab.WithInterval(c.cfg.ConsumerCommitInterval), crontab.WithIgnorePanic(), crontab.WithImmediately())
	c.monitorOffsetJob.Run()
}

func (c *KafkaClient) monitorMsgQueueSize() {
	fn := func() {
		c.msgQueue.Range(func(key, value interface{}) bool {
			msgQueue := value.(*basic.SafeRingQueue)
			logger.Info("[kafka-consumer] [key: %s] msgQueue size: %d", key, msgQueue.Size())
			setQueueSize(c.cfg, key.(string), float64(msgQueue.Size()))
			return true
		})
	}
	// 每分钟打印一次消息缓存队列大小
	c.monitorQueueSizeJob = crontab.NewRegularJob("MonitorMsgQueueSize", fn, crontab.WithInterval(1*time.Minute), crontab.WithIgnorePanic(), crontab.WithImmediately())
	c.monitorQueueSizeJob.Run()
}

func (c *KafkaClient) Listen(fn func(message interface{})) {
	c.lock.Lock()
	defer c.lock.Unlock()
	if c.running {
		return
	}
	c.running = true

	c.msgChan = make(chan *msgItem, c.cfg.ConsumerQueueSize)
	c.closeChan = make(chan struct{}, c.cfg.ConsumerWorkerNum)
	c.msgQueue = sync.Map{}
	ctx, cancelFunc := context.WithCancel(context.Background())
	c.cancelFunc = cancelFunc
	c.ctx = ctx

	go c.openConsume()
	go c.consume(fn)
	c.monitorOffset()
	c.monitorMsgQueueSize()
}
