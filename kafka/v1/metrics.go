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
