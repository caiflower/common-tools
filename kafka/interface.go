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

package xkafka

import (
	"crypto/tls"
	"crypto/x509"
	"os"
	"time"
)

type Config struct {
	Name                      string        `yaml:"name"`
	Enable                    string        `yaml:"enable" default:"true"` // 默认开启 / Enabled by default
	BootstrapServers          []string      `yaml:"bootstrapServers"`
	GroupID                   string        `yaml:"groupId"`
	Topics                    []string      `yaml:"topics"`
	ProducerAcks              int           `yaml:"producerAcks" default:"-1"`
	ProducerCompressType      string        `yaml:"producerCompressType" default:"none"`    // none, gzip, snappy, lz4, zstd
	ProducerMessageTimeout    int           `yaml:"producerMessageTimeout" default:"15000"` // 默认15秒, 仅在v1生效 / Default 15s, only effective in v1
	ProducerRequestTimeout    int           `yaml:"producerRequestTimeout" default:"10000"` // 默认10秒 / Default 10s
	ProducerVersion           string        `yaml:"producerVersion"`                        // kafka版本，仅在v2生效 / Kafka version, only effective in v2
	ConsumerWorkerNum         int           `yaml:"consumerWorkerNum" default:"2"`
	ConsumerHeartBeatInterval time.Duration `yaml:"consumerHeartBeatInterval" default:"6s"`
	ConsumerSessionTimeout    time.Duration `yaml:"consumerSessionTimeout" default:"20s"`
	ConsumerAutoOffsetReset   string        `yaml:"consumerAutoOffsetReset" default:"latest"` // https://www.cnblogs.com/convict/p/16701154.html
	ConsumerQueueSize         int           `yaml:"consumerQueueSize" default:"500"`          // 消费者每个缓存队列大小，默认1000 / Consumer buffer queue size per partition
	ConsumerCommitInterval    time.Duration `yaml:"consumerCommitInterval" default:"1s"`      // 提交offset间隔 / Offset commit interval
	ConsumerFetchMaxBytes     int           `yaml:"consumerFetchMaxBytes" default:"52428800"` // 一次抓取消息的最大大小，要确保这个大小要大于一条消息的最大大小，默认50MB，如果内存占用过高，可以适当调小 / Max bytes per fetch, ensure this is larger than max message size, default 50MB
	ConsumerRetryCount        int           `yaml:"consumerRetryCount" default:"3"`           // 消费失败最大重试次数，默认3 / Max retry count for failed messages, default 3
	SecurityProtocol          string        `yaml:"securityProtocol"`
	SaslMechanism             string        `yaml:"saslMechanism"`
	SaslUsername              string        `yaml:"saslUsername"`
	SaslPassword              string        `yaml:"saslPassword"`
	SSLCaFile                 string        `yaml:"sslCaFile"`
	SSLCertFile               string        `yaml:"sslCertFile"`
	SSLKeyFile                string        `yaml:"sslKeyFile"`
}

// DeadLetterHandler is called when a message fails after all retries are exhausted.
// The handler receives the original message and the last error from the callback.
//
// DeadLetterHandler 在消息重试次数耗尽后被调用，
// 接收原始消息和最后一次回调返回的错误。
type DeadLetterHandler func(message interface{}, err error)

type Consumer interface {
	// Listen starts consuming messages. If the callback returns a non-nil error,
	// the message will be re-enqueued for retry up to ConsumerRetryCount times.
	// When all retries are exhausted, the optional deadLetterHandler is called,
	// then the message offset is committed to prevent rebalance.
	//
	// Listen 启动消息消费。如果回调返回非 nil 的 error，
	// 消息将重新入队重试，最多重试 ConsumerRetryCount 次。
	// 重试次数耗尽后，调用可选的死信回调 deadLetterHandler，
	// 然后提交 offset 以避免 rebalance。
	Listen(fn func(message interface{}) error, deadLetterHandler ...DeadLetterHandler)
	Close()
}

type Producer interface {
	Send(topic string, key string, values ...interface{}) error
	AsyncSend(topic string, key string, values ...interface{}) error
	Close()
}

func NewTLSConfig(caFile, certFile, keyFile string) (*tls.Config, error) {
	tlsConfig := &tls.Config{}

	if caFile != "" {
		caCert, err := os.ReadFile(caFile)
		if err != nil {
			return nil, err
		}
		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(caCert)
		tlsConfig.RootCAs = caCertPool
	}

	if certFile != "" && keyFile != "" {
		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			return nil, err
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	return tlsConfig, nil
}
