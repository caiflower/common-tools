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
	"sync"

	"github.com/caiflower/common-tools/global/env"
	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	ConnectErr   = "connect"
	ConsumeErr   = "consume"
	RebalanceErr = "rebalance"
	AsyncErr     = "async"
	SyncErr      = "sync"
)

var (
	consumerCount             *prometheus.CounterVec
	producerCount             *prometheus.CounterVec
	consumerErrCount          *prometheus.CounterVec
	producerErrCount          *prometheus.CounterVec
	consumerQueueSize         *prometheus.GaugeVec
	consumerConsumedHistogram prometheus.Histogram
	metricsOnce               sync.Once
)

func registerMetric(collector prometheus.Collector) {
	if err := prometheus.Register(collector); err != nil {
		if _, ok := err.(prometheus.AlreadyRegisteredError); ok {
			logger.Warn("prometheus metric already registered: %v", err)
		} else {
			logger.Error("failed to register prometheus metric: %v", err)
		}
	}
}

func ensureMetricsRegistered() {
	metricsOnce.Do(func() {
		constLabels := prometheus.Labels{"ip": env.GetLocalHostIP(), "version": "v2"}
		consumerCount = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "kafka_consumer_count", Help: "Number of messages consumed by kafka consumer", ConstLabels: constLabels}, []string{"name", "url", "topic"})
		producerCount = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "kafka_producer_count", Help: "Number of messages produced by kafka producer", ConstLabels: constLabels}, []string{"name", "url", "topic"})
		consumerErrCount = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "kafka_consumer_err_count", Help: "Number of errors encountered by kafka consumer", ConstLabels: constLabels}, []string{"name", "url", "topic", "type"})
		producerErrCount = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "kafka_producer_err_count", Help: "Number of errors encountered by kafka producer", ConstLabels: constLabels}, []string{"name", "url", "topic", "type"})
		consumerQueueSize = prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "kafka_consumer_queue_size", Help: "Size of kafka consumer cache queue", ConstLabels: constLabels}, []string{"name", "url", "key"})
		consumerConsumedHistogram = prometheus.NewHistogram(prometheus.HistogramOpts{Name: "kafka_consumer_consume_duration_ms", Help: "Histogram of kafka consumer message consumption durations in milliseconds.", Buckets: []float64{20, 50, 100, 200, 500, 1000, 2000, 5000, 10000}, ConstLabels: constLabels})
		registerMetric(consumerCount)
		registerMetric(producerCount)
		registerMetric(consumerErrCount)
		registerMetric(producerErrCount)
		registerMetric(consumerQueueSize)
		registerMetric(consumerConsumedHistogram)
	})
}

func AddConsumerError(cfg *Config, typ string) {
	ensureMetricsRegistered()
	consumerErrCount.WithLabelValues(cfg.Name, strings.Join(cfg.BootstrapServers, ","), strings.Join(cfg.Topics, ","), typ).Inc()
}

func AddProducerErrCount(cfg *Config, topic string, typ string) {
	ensureMetricsRegistered()
	producerErrCount.WithLabelValues(cfg.Name, strings.Join(cfg.BootstrapServers, ","), topic, typ).Inc()
}

func CountConsumer(cfg *Config) {
	ensureMetricsRegistered()
	consumerCount.WithLabelValues(cfg.Name, strings.Join(cfg.BootstrapServers, ","), strings.Join(cfg.Topics, ",")).Inc()
}

func CountProducer(cfg *Config, topic string) {
	ensureMetricsRegistered()
	producerCount.WithLabelValues(cfg.Name, strings.Join(cfg.BootstrapServers, ","), topic).Inc()
}

func SetQueueSize(cfg *Config, key string, value float64) {
	ensureMetricsRegistered()
	consumerQueueSize.WithLabelValues(cfg.Name, strings.Join(cfg.BootstrapServers, ","), key).Set(value)
}

func RecordConsumedDuration(duration int64) {
	ensureMetricsRegistered()
	consumerConsumedHistogram.Observe(float64(duration))
}
