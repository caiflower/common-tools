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
	"strings"

	"github.com/caiflower/common-tools/global/env"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	ConnectErr   = "connect"
	ConsumeErr   = "consume"
	RebalanceErr = "rebalance"
	AsyncErr     = "async"
	SyncErr      = "sync"
)

var consumerCount *prometheus.CounterVec           //消费次数
var producerCount *prometheus.CounterVec           //生产次数
var consumerErrCount *prometheus.CounterVec        //失败次数
var producerErrCount *prometheus.CounterVec        //失败次数
var consumerQueueSize *prometheus.GaugeVec         //队列大小
var consumerConsumedHistogram prometheus.Histogram //消费者消费直方图

func init() {
	constLabels := prometheus.Labels{"ip": env.GetLocalHostIP(), "version": "v2"}
	consumerCount = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "kafka_consumer_count", Help: "Number of messages consumed by kafka consumer", ConstLabels: constLabels}, []string{"name", "url", "topic"})
	producerCount = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "kafka_producer_count", Help: "Number of messages produced by kafka producer", ConstLabels: constLabels}, []string{"name", "url", "topic"})
	consumerErrCount = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "kafka_consumer_err_count", Help: "Number of errors encountered by kafka consumer", ConstLabels: constLabels}, []string{"name", "url", "topic", "type"})
	producerErrCount = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "kafka_producer_err_count", Help: "Number of errors encountered by kafka producer", ConstLabels: constLabels}, []string{"name", "url", "topic", "type"})
	consumerQueueSize = prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "kafka_consumer_queue_size", Help: "Size of kafka consumer cache queue", ConstLabels: constLabels}, []string{"name", "url", "key"})
	consumerConsumedHistogram = prometheus.NewHistogram(prometheus.HistogramOpts{Name: "kafka_consumer_consume_duration_ms", Help: "Histogram of kafka consumer message consumption durations in milliseconds.", Buckets: []float64{20, 50, 100, 200, 500, 1000, 2000, 5000, 10000}, ConstLabels: constLabels})
	_ = prometheus.Register(consumerErrCount)
	_ = prometheus.Register(producerErrCount)
	_ = prometheus.Register(consumerQueueSize)
	_ = prometheus.Register(consumerConsumedHistogram)
}

func AddConsumerError(cfg *Config, typ string) {
	consumerErrCount.WithLabelValues(cfg.Name, strings.Join(cfg.BootstrapServers, ","), strings.Join(cfg.Topics, ","), typ).Inc()
}

func AddProducerErrCount(cfg *Config, topic string, typ string) {
	producerErrCount.WithLabelValues(cfg.Name, strings.Join(cfg.BootstrapServers, ","), topic, typ).Inc()
}

func CountConsumer(cfg *Config) {
	consumerCount.WithLabelValues(cfg.Name, strings.Join(cfg.BootstrapServers, ","), strings.Join(cfg.Topics, ",")).Inc()
}

func CountProducer(cfg *Config, topic string) {
	producerCount.WithLabelValues(cfg.Name, strings.Join(cfg.BootstrapServers, ","), topic).Inc()
}

func SetQueueSize(cfg *Config, key string, value float64) {
	consumerQueueSize.WithLabelValues(cfg.Name, strings.Join(cfg.BootstrapServers, ","), key).Set(value)
}

func RecordConsumedDuration(duration int64) {
	consumerConsumedHistogram.Observe(float64(duration))
}
