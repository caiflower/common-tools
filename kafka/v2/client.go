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
	"crypto/sha256"
	"crypto/sha512"
	"hash"
	"sync"

	"github.com/IBM/sarama"
	xkafka "github.com/caiflower/common-tools/kafka"
	"github.com/caiflower/common-tools/pkg/crontab"
	"github.com/caiflower/common-tools/pkg/e"
	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/xdg-go/scram"
)

type KafkaClient struct {
	cfg          *xkafka.Config
	saramaConfig *sarama.Config
	lock         sync.Mutex
	running      bool

	consumerGroup       sarama.ConsumerGroup
	msgChan             chan *msgItem
	msgQueue            sync.Map
	consumerSession     sarama.ConsumerGroupSession
	monitorOffsetJob    crontab.RegularJob
	monitorQueueSizeJob crontab.RegularJob
	commitOffsetFunc    func()
	closeChan           chan struct{}
	ctx                 context.Context
	cancelFunc          context.CancelFunc

	// 同步发送
	syncProducer sarama.SyncProducer
	// 异步发送
	asyncProducer sarama.AsyncProducer
	retryVersions map[string]interface{}
}

func (c *KafkaClient) Close() {
	c.lock.Lock()
	defer c.lock.Unlock()
	logger.Info("[Kafka client close] name='%s'", c.cfg.Name)
	defer e.OnError("")

	if c.running == false {
		return
	}
	c.running = false

	if c.msgChan != nil {
		c.cancelFunc()

		close(c.msgChan)

		for i := 1; i <= c.cfg.ConsumerWorkerNum; i++ {
			<-c.closeChan
		}

		if c.commitOffsetFunc != nil {
			// 手动再提交一次offset
			logger.Info("[Kafka client close] commit offset before close, name='%s'", c.cfg.Name)
			c.commitOffsetFunc()
		}

		c.msgChan = nil
		c.closeChan = nil
	}

	if c.consumerGroup != nil {
		_ = c.consumerGroup.Close()
		c.consumerGroup = nil
	}

	if c.monitorOffsetJob != nil {
		c.monitorOffsetJob.Stop()
		c.monitorOffsetJob = nil
	}
	if c.monitorQueueSizeJob != nil {
		c.monitorQueueSizeJob.Stop()
		c.monitorQueueSizeJob = nil
	}

	if c.syncProducer != nil {
		_ = c.syncProducer.Close()
		c.syncProducer = nil
	}
	if c.asyncProducer != nil {
		_ = c.asyncProducer.Close()
		c.asyncProducer = nil
	}
}

var SHA256 scram.HashGeneratorFcn = func() hash.Hash { return sha256.New() }
var SHA512 scram.HashGeneratorFcn = func() hash.Hash { return sha512.New() }

type XDGSCRAMClient struct {
	*scram.Client
	*scram.ClientConversation
	scram.HashGeneratorFcn
}

func (x *XDGSCRAMClient) Begin(userName, password, authzID string) (err error) {
	x.Client, err = x.HashGeneratorFcn.NewClient(userName, password, authzID)
	if err != nil {
		return err
	}
	x.ClientConversation = x.Client.NewConversation()
	return nil
}

func (x *XDGSCRAMClient) Step(challenge string) (response string, err error) {
	response, err = x.ClientConversation.Step(challenge)
	return
}

func (x *XDGSCRAMClient) Done() bool {
	return x.ClientConversation.Done()
}
