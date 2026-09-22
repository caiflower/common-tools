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

package v2

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/IBM/sarama"
	"github.com/caiflower/common-tools/global"
	xkafka "github.com/caiflower/common-tools/kafka"
	"github.com/caiflower/common-tools/pkg/basic"
	"github.com/caiflower/common-tools/pkg/crontab"
	"github.com/caiflower/common-tools/pkg/e"
	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/caiflower/common-tools/pkg/tools"
)

var _ xkafka.BatchConsumer = (*KafkaClient)(nil)

type KafkaMessage = sarama.ConsumerMessage

func NewConsumerClient(cfg xkafka.Config) *KafkaClient {
	_ = tools.DoTagFunc(&cfg, []tools.FnObj{{Fn: tools.SetDefaultValueIfNil}})

	if strings.ToUpper(cfg.Enable) != "TRUE" {
		logger.Warn("[kafka-consumer] consumer '%s' is disable", cfg.Name)
		return &KafkaClient{
			cfg: &cfg,
		}
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

	if strings.ToUpper(cfg.SecurityProtocol) == "SSL" {
		config.Net.TLS.Enable = true
		config.Net.TLS.Config, _ = xkafka.NewTLSConfig(cfg.SSLCaFile, cfg.SSLCertFile, cfg.SSLKeyFile)
	}

	kafkaClient := &KafkaClient{
		cfg:                   &cfg,
		saramaConfig:          config,
		consumerReplayOffsets: buildConsumerReplayOffsets(cfg.ConsumerReplayOffsets),
	}

	logger.Info("[kafka-consumer] consumer '%s' config: %s", cfg.Name, tools.ToJson(cfg))

	global.DefaultResourceManger.Add(kafkaClient)
	return kafkaClient
}

type consumerGroupHandler struct {
	lastCommitTime time.Time
	*KafkaClient
}

type consumeState struct {
	done          atomic.Bool
	retryCount    int
	lastRetryTime time.Time
	retryDelay    time.Duration
}

func (s *consumeState) markDone() {
	s.done.Store(true)
}

type msgItem struct {
	consumeState
	msg *sarama.ConsumerMessage
}

type batchItem struct {
	consumeState
	messages []*sarama.ConsumerMessage
	values   []interface{}
}

type queuedItem interface {
	isDone() bool
	lastMessage() *sarama.ConsumerMessage
}

func (i *msgItem) isDone() bool {
	return i.done.Load()
}

func (i *msgItem) lastMessage() *sarama.ConsumerMessage {
	return i.msg
}

func (i *batchItem) isDone() bool {
	return i.done.Load()
}

func (i *batchItem) lastMessage() *sarama.ConsumerMessage {
	if len(i.messages) == 0 {
		return nil
	}
	return i.messages[len(i.messages)-1]
}

func (h *consumerGroupHandler) Setup(session sarama.ConsumerGroupSession) error {
	logger.Info("%s", tools.ToJson(session.Claims()))
	resetOffset := false
	for topic, partitions := range session.Claims() {
		for _, partition := range partitions {
			key := topicPartitionKey(topic, partition)
			offset, ok := h.consumerReplayOffsets[key]
			if !ok {
				continue
			}
			if _, loaded := h.replayedOffsets.LoadOrStore(key, struct{}{}); loaded {
				continue
			}

			session.ResetOffset(topic, partition, offset, "")
			session.MarkOffset(topic, partition, offset, "")
			logger.Info("[kafka-consumer] replay offset [name=%s] [topic=%s] [partition=%d] [offset=%d]", h.cfg.Name, topic, partition, offset)
			resetOffset = true
		}
	}
	if resetOffset {
		session.Commit()
	}
	return nil
}

func buildConsumerReplayOffsets(offsets []xkafka.ConsumerReplayOffset) map[string]int64 {
	if len(offsets) == 0 {
		return nil
	}
	result := make(map[string]int64, len(offsets))
	for _, offset := range offsets {
		if offset.Topic == "" {
			continue
		}
		result[topicPartitionKey(offset.Topic, offset.Partition)] = offset.Offset
	}
	return result
}

func (h *consumerGroupHandler) Cleanup(session sarama.ConsumerGroupSession) error {
	h.sessionMu.Lock()
	h.consumerSession = nil
	h.sessionMu.Unlock()
	return nil
}

// ConsumeClaim sarama 调度时，对应每一个 partition，会启动一个 ConsumeClaim 协程，参数 claim 就代表一个分区
func (h *consumerGroupHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	h.sessionMu.Lock()
	h.consumerSession = session
	h.sessionMu.Unlock()
	if h.batchMode {
		return h.consumeClaimBatch(session, claim)
	}
	for {
		select {
		case msg, ok := <-claim.Messages():
			if !ok {
				return nil
			}
			logger.Trace("Message receive event : [name=%s] [group=%s] [topic=%s] [partition=%d] [offset=%d] [msg=%s]", h.cfg.Name, h.cfg.GroupID, msg.Topic, msg.Partition, msg.Offset, string(msg.Value))

			item := &msgItem{msg: msg}

			key := topicPartitionKey(msg.Topic, msg.Partition)
			select {
			case <-h.ctx.Done():
				logger.Info("[ConsumeClaim] consumer is closed. [key:%s]", key)
				return nil
			case h.msgChan <- item:
			}

			h.getQueue(key, h.cfg.ConsumerQueueSize).BlockEnqueue(item)
		case <-session.Context().Done(): //表示内部会话已关闭，这里一定要退出去，否则会导致 rebalance 超时
			return nil
		}
	}
}

func (h *consumerGroupHandler) consumeClaimBatch(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for {
		messages, ok := h.collectBatch(session, claim)
		if !ok {
			return nil
		}

		item := newBatchItem(messages)
		key := topicPartitionKey(claim.Topic(), claim.Partition())
		queue := h.getQueue(key, batchQueueCapacity(h.cfg.ConsumerQueueSize, h.cfg.ConsumerBatchSize))
		queue.BlockEnqueue(item)

		batchChan, ok := h.ensureBatchWorker(key)
		if !ok {
			return nil
		}
		select {
		case <-h.ctx.Done():
			logger.Info("[ConsumeClaim] consumer is closed. [key:%s]", key)
			return nil
		case batchChan <- item:
		}
	}
}

func (h *consumerGroupHandler) getQueue(key string, capacity int) *basic.SafeRingQueue {
	queue, ok := h.msgQueue.Load(key)
	if !ok {
		queue = basic.NewSafeRingQueue(capacity)
		h.msgQueue.Store(key, queue)
	}
	return queue.(*basic.SafeRingQueue)
}

func topicPartitionKey(topic string, partition int32) string {
	return fmt.Sprintf("%s-%d", topic, partition)
}

func (h *consumerGroupHandler) ensureBatchWorker(key string) (chan *batchItem, bool) {
	if value, ok := h.batchQueues.Load(key); ok {
		return value.(chan *batchItem), true
	}

	h.batchWorkerMu.Lock()
	defer h.batchWorkerMu.Unlock()
	if !h.running.Load() {
		return nil, false
	}
	if value, ok := h.batchQueues.Load(key); ok {
		return value.(chan *batchItem), true
	}

	capacity := batchQueueCapacity(h.cfg.ConsumerQueueSize, h.cfg.ConsumerBatchSize)
	batchChan := make(chan *batchItem, capacity)
	h.batchQueues.Store(key, batchChan)
	h.batchWG.Add(1)
	go h.consumeBatch(batchChan)
	return batchChan, true
}

func (c *KafkaClient) consumeBatch(batchChan <-chan *batchItem) {
	defer c.batchWG.Done()
	for {
		select {
		case <-c.ctx.Done():
			return
		case item, ok := <-batchChan:
			if !ok {
				return
			}
			if !c.running.Load() {
				return
			}
			select {
			case c.batchSem <- struct{}{}:
			case <-c.ctx.Done():
				return
			}
			c.processBatch(item, c.batchHandler, c.batchDeadLetterFunc)
			<-c.batchSem
		}
	}
}

func batchQueueCapacity(queueSize, batchSize int) int {
	if queueSize <= 0 || batchSize <= 0 {
		return 1
	}
	capacity := (queueSize + batchSize - 1) / batchSize
	if capacity < 1 {
		return 1
	}
	return capacity
}

func retryDelay(retryCount int) time.Duration {
	backoffSeconds := 1 << (retryCount - 1)
	if backoffSeconds > 30 {
		backoffSeconds = 30
	}
	return time.Duration(backoffSeconds) * time.Second
}

func newBatchItem(messages []*sarama.ConsumerMessage) *batchItem {
	values := make([]interface{}, len(messages))
	for i, msg := range messages {
		values[i] = msg
	}
	return &batchItem{
		messages: messages,
		values:   values,
	}
}

func completedQueueItem(value interface{}) (*sarama.ConsumerMessage, bool) {
	item, ok := value.(queuedItem)
	if !ok || !item.isDone() {
		return nil, false
	}
	msg := item.lastMessage()
	return msg, msg != nil
}

func (h *consumerGroupHandler) collectBatch(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) ([]*sarama.ConsumerMessage, bool) {
	select {
	case <-h.ctx.Done():
		return nil, false
	case <-session.Context().Done():
		return nil, false
	case msg, ok := <-claim.Messages():
		if !ok {
			return nil, false
		}
		batch := make([]*sarama.ConsumerMessage, 0, h.cfg.ConsumerBatchSize)
		batch = append(batch, msg)
		if len(batch) >= h.cfg.ConsumerBatchSize {
			return batch, true
		}
		if h.cfg.ConsumerBatchWait <= 0 {
			return batch, true
		}

		// Flush partial batches after the max wait so low-throughput partitions
		// still reach the batch worker.
		timer := time.NewTimer(h.cfg.ConsumerBatchWait)
		defer timer.Stop()
		for len(batch) < h.cfg.ConsumerBatchSize {
			select {
			case <-h.ctx.Done():
				return nil, false
			case <-session.Context().Done():
				return nil, false
			case <-timer.C:
				return batch, true
			case next, ok := <-claim.Messages():
				if !ok {
					return batch, true
				}
				batch = append(batch, next)
			}
		}
		return batch, true
	}
}

func (c *KafkaClient) openConsume() {
	// 创建消费组
	var consumerGroup sarama.ConsumerGroup
	var err error
label:
	for {
		if !c.running.Load() {
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
			xkafka.AddConsumerError(c.cfg, xkafka.ConnectErr)
			logger.Error("%s [ConsumerGroup] open failed. Error: %v", c.cfg.Name, err)
			time.Sleep(time.Second)
		}
	}

	// 监控错误
	go func() {
		for err := range consumerGroup.Errors() {
			xkafka.AddConsumerError(c.cfg, xkafka.ConsumeErr)
			logger.Error("%s %v", c.cfg.Name, err)
		}
	}()

	// 开始消费
	// 当发生 rebalance 时，Consume 方法会重复调用，关闭老会话，创建新会话。
	c.consumerGroup = consumerGroup
	ctx := context.Background()
	for {
		if !c.running.Load() {
			logger.Info("[ConsumerGroup] consumer is closed, name=%s", c.cfg.Name)
			break
		}
		handler := &consumerGroupHandler{KafkaClient: c, lastCommitTime: time.Now()}
		err = consumerGroup.Consume(ctx, c.cfg.Topics, handler)
		if err != nil {
			xkafka.AddConsumerError(c.cfg, xkafka.RebalanceErr)
			logger.Error("%s [ConsumerGroup] return error. Error: %v", c.cfg.Name, err)
			time.Sleep(time.Second)
		}
		logger.Warn("%s [ConsumerGroup] reconsume. may be cause of rebalance.", c.cfg.Name)
	}
}

func (c *KafkaClient) consume(fn func(message interface{}) error, deadLetterHandler ...xkafka.DeadLetterHandler) {
	var dlHandler xkafka.DeadLetterHandler
	if len(deadLetterHandler) > 0 {
		dlHandler = deadLetterHandler[0]
	}

	runThread := func(tid int) {
		logger.Debug("[kafka-consumer] [%s-%d] started.", c.cfg.Name, tid)
		defer func() {
			c.closeChan <- struct{}{}
			logger.Debug("[kafka-consumer] [%s-%d] Exited.", c.cfg.Name, tid)
		}()

		for item := range c.msgChan {
			// if close， return immediately
			if !c.running.Load() {
				return
			}

			completed := func() bool {
				defer e.OnError(fmt.Sprintf("kafka [%s-%d] consumer listen", c.cfg.Name, tid))
				startTime := time.Now()
				if err := fn(item.msg); err != nil {
					item.retryCount++
					if item.retryCount < c.cfg.ConsumerRetryCount {
						// Check if consumer is still running before retry, avoid writing to closed channel / 消费者关闭时不再重试，避免向已关闭 channel 写入
						if !c.running.Load() {
							return false
						}
						item.retryDelay = retryDelay(item.retryCount)
						item.lastRetryTime = time.Now()

						// Retry: re-enqueue the message for another attempt / 重试：将消息重新入队
						logger.Warn("[kafka-consumer] [%s-%d] consume message failed [topic=%s] [partition=%d] [offset=%d] (retry %d/%d), will retry after %v. Error: %v", c.cfg.Name, tid, item.msg.Topic, item.msg.Partition, item.msg.Offset, item.retryCount, c.cfg.ConsumerRetryCount, item.retryDelay, err)
						// Use goroutine to delay re-enqueue without blocking the worker
						// 使用 goroutine 延迟入队，不阻塞当前 worker 协程
						go func(msg *msgItem) {
							time.Sleep(msg.retryDelay)
							select {
							case <-c.ctx.Done():
								return
							case c.msgChan <- msg:
							}
						}(item)
						return false
					}
					// All retries exhausted / 重试次数耗尽
					logger.Error("[kafka-consumer] [%s-%d] consume message failed [topic=%s] [partition=%d] [offset=%d] after %d retries. Error: %v", c.cfg.Name, tid, item.msg.Topic, item.msg.Partition, item.msg.Offset, c.cfg.ConsumerRetryCount, err)
					if dlHandler != nil {
						// Call dead letter handler / 调用死信回调
						dlHandler(item.msg, err)
					}
					// Mark done to allow offset commit, preventing rebalance / 标记完成以允许 offset 提交，避免 rebalance
				}
				xkafka.RecordConsumedDuration(time.Since(startTime).Milliseconds())
				xkafka.CountConsumer(c.cfg)
				return true
			}()
			if completed {
				item.markDone()
			}
		}
	}
	for i := 1; i <= c.cfg.ConsumerWorkerNum; i++ {
		go runThread(i)
	}
}

func (c *KafkaClient) processBatch(item *batchItem, fn xkafka.BatchHandler, dlHandler xkafka.BatchDeadLetterHandler) {
	defer e.OnError(fmt.Sprintf("kafka [%s] batch consumer listen", c.cfg.Name))
	for {
		startTime := time.Now()
		err := fn(item.values)
		xkafka.RecordConsumedDuration(time.Since(startTime).Milliseconds())
		if err == nil {
			for range item.messages {
				xkafka.CountConsumer(c.cfg)
			}
			item.markDone()
			return
		}

		item.retryCount++
		if item.retryCount < c.cfg.ConsumerRetryCount {
			item.retryDelay = retryDelay(item.retryCount)
			item.lastRetryTime = time.Now()
			logger.Warn("[kafka-consumer] batch consume failed [topic=%s] [partition=%d] [firstOffset=%d] [messages=%d] (retry %d/%d), will retry after %v. Error: %v",
				item.messages[0].Topic, item.messages[0].Partition, item.messages[0].Offset, len(item.messages), item.retryCount, c.cfg.ConsumerRetryCount, item.retryDelay, err)

			timer := time.NewTimer(item.retryDelay)
			select {
			case <-c.ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
			continue
		}

		logger.Error("[kafka-consumer] batch consume failed [topic=%s] [partition=%d] [firstOffset=%d] [messages=%d] after %d retries. Error: %v",
			item.messages[0].Topic, item.messages[0].Partition, item.messages[0].Offset, len(item.messages), c.cfg.ConsumerRetryCount, err)
		if dlHandler != nil {
			dlHandler(item.values, err)
		}
		for range item.messages {
			xkafka.CountConsumer(c.cfg)
		}
		item.markDone()
		return
	}
}

func (c *KafkaClient) monitorOffset() {
	fn := func() {
		if c.monitorOffsetRunning.Load() {
			return
		}
		c.monitorOffsetRunning.Store(true)
		defer c.monitorOffsetRunning.Store(false)

		c.commitCompletedOffsets()
	}
	c.commitOffsetFunc = fn
	c.monitorOffsetJob = crontab.NewRegularJob("MonitorOffset", fn, crontab.WithInterval(c.cfg.ConsumerCommitInterval), crontab.WithIgnorePanic(), crontab.WithImmediately())
	c.monitorOffsetJob.Run()
}

func (c *KafkaClient) commitCompletedOffsets() {
	c.commitCycleCount++
	c.msgQueue.Range(func(key, value interface{}) bool {
		msgQueue, ok := value.(*basic.SafeRingQueue)
		if !ok {
			return true
		}

		var lastDoneMsg *sarama.ConsumerMessage
		for {
			item, err := msgQueue.Peek()
			if err != nil {
				break
			}
			lastMessage, done := completedQueueItem(item)
			if !done {
				break
			}
			lastDoneMsg = lastMessage
			_, _ = msgQueue.Dequeue()
		}

		if lastDoneMsg != nil {
			if c.commitCycleCount%10 == 0 {
				logger.Info("%s Commit offset [key=%s] [offset=%d]", c.cfg.Name, key, lastDoneMsg.Offset)
			}
			c.sessionMu.RLock()
			session := c.consumerSession
			c.sessionMu.RUnlock()
			if session != nil {
				session.MarkMessage(lastDoneMsg, "")
				session.Commit()
			}
		}
		return true
	})

	if c.commitCycleCount%10 == 0 {
		c.commitCycleCount = 0
	}
}

func (c *KafkaClient) monitorMsgQueueSize() {
	fn := func() {
		c.msgQueue.Range(func(key, value interface{}) bool {
			msgQueue := value.(*basic.SafeRingQueue)
			logger.Info("[kafka-consumer] [key: %s] msgQueue size: %d", key, msgQueue.Size())
			xkafka.SetQueueSize(c.cfg, key.(string), float64(msgQueue.Size()))
			return true
		})
	}
	// 每分钟打印一次消息缓存队列大小
	c.monitorQueueSizeJob = crontab.NewRegularJob("MonitorMsgQueueSize", fn, crontab.WithInterval(1*time.Minute), crontab.WithIgnorePanic(), crontab.WithImmediately())
	c.monitorQueueSizeJob.Run()
}

func (c *KafkaClient) Listen(fn func(message interface{}) error, deadLetterHandler ...xkafka.DeadLetterHandler) {
	c.lock.Lock()
	defer c.lock.Unlock()
	if c.running.Load() || strings.ToUpper(c.cfg.Enable) != "TRUE" {
		return
	}
	c.batchMode = false

	c.msgChan = make(chan *msgItem, c.cfg.ConsumerQueueSize)
	c.closeChan = make(chan struct{}, c.cfg.ConsumerWorkerNum)
	c.startConsumer(func() {
		go c.consume(fn, deadLetterHandler...)
	})
}

func (c *KafkaClient) ListenBatch(fn xkafka.BatchHandler, deadLetterHandler ...xkafka.BatchDeadLetterHandler) {
	c.lock.Lock()
	defer c.lock.Unlock()
	if c.running.Load() || strings.ToUpper(c.cfg.Enable) != "TRUE" {
		return
	}
	if c.cfg.ConsumerBatchSize <= 0 {
		c.cfg.ConsumerBatchSize = 1
	}
	if c.cfg.ConsumerBatchWait < 0 {
		c.cfg.ConsumerBatchWait = 0
	}
	workerNum := c.cfg.ConsumerWorkerNum
	if workerNum <= 0 {
		workerNum = 1
	}

	var dlHandler xkafka.BatchDeadLetterHandler
	if len(deadLetterHandler) > 0 {
		dlHandler = deadLetterHandler[0]
	}

	c.batchMode = true
	c.batchHandler = fn
	c.batchDeadLetterFunc = dlHandler
	c.batchQueues = sync.Map{}
	c.batchWG = sync.WaitGroup{}
	c.batchSem = make(chan struct{}, workerNum)
	c.startConsumer(nil)
}

func (c *KafkaClient) startConsumer(startWorkers func()) {
	c.msgQueue = sync.Map{}
	c.replayedOffsets = sync.Map{}
	ctx, cancelFunc := context.WithCancel(context.Background())
	c.cancelFunc = cancelFunc
	c.ctx = ctx
	c.running.Store(true)

	go c.openConsume()
	if startWorkers != nil {
		startWorkers()
	}
	c.monitorOffset()
	c.monitorMsgQueueSize()
}
