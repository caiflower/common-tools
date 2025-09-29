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

 package v1

import (
	"strings"

	"github.com/caiflower/common-tools/global/env"
	xkafka "github.com/caiflower/common-tools/kafka"
	"github.com/prometheus/client_golang/prometheus"
)

var consumerCount *prometheus.CounterVec    //消费次数
var producerCount *prometheus.CounterVec    //生产次数
var consumerErrCount *prometheus.CounterVec //失败次数
var producerErrCount *prometheus.CounterVec //失败次数

func init() {
	constLabels := prometheus.Labels{"ip": env.GetLocalHostIP(), "version": "v1"}
	consumerCount = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "kafka_consumer_count", Help: "kafka consumer count", ConstLabels: constLabels}, []string{"name", "url", "topic"})
	producerCount = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "kafka_producer_count", Help: "kafka producer count", ConstLabels: constLabels}, []string{"name", "url", "topic"})
	consumerErrCount = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "kafka_consumer_err_count", Help: "kafka consumer err count", ConstLabels: constLabels}, []string{"name", "url", "topic", "type"})
	producerErrCount = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "kafka_producer_err_count", Help: "kafka producer err count", ConstLabels: constLabels}, []string{"name", "url", "topic", "type"})
	_ = prometheus.Register(consumerErrCount)
	_ = prometheus.Register(producerErrCount)
}

func addConsumerError(cfg *xkafka.Config, typ string) {
	consumerErrCount.WithLabelValues(cfg.Name, strings.Join(cfg.BootstrapServers, ","), strings.Join(cfg.Topics, ","), typ).Inc()
}

func addProducerErrCount(cfg *xkafka.Config, topic string, typ string) {
	producerErrCount.WithLabelValues(cfg.Name, strings.Join(cfg.BootstrapServers, ","), topic, typ).Inc()
}

func countConsumer(cfg *xkafka.Config) {
	consumerCount.WithLabelValues(cfg.Name, strings.Join(cfg.BootstrapServers, ","), strings.Join(cfg.Topics, ",")).Inc()
}

func countProducer(cfg *xkafka.Config, topic string) {
	producerCount.WithLabelValues(cfg.Name, strings.Join(cfg.BootstrapServers, ","), topic).Inc()
}
