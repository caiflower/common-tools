package xkafka

import "time"

type Config struct {
	Name                      string        `yaml:"name"`
	Enable                    string        `yaml:"enable" default:"true"` // 默认开启
	BootstrapServers          []string      `yaml:"bootstrapServers"`
	GroupID                   string        `yaml:"groupId"`
	Topics                    []string      `yaml:"topics"`
	ProducerAcks              int           `yaml:"producerAcks" default:"-1"`
	ProducerCompressType      string        `yaml:"producerCompressType" default:"none"`    // none, gzip, snappy, lz4, zstd
	ProducerMessageTimeout    int           `yaml:"producerMessageTimeout" default:"15000"` // 默认15秒, 仅在v1生效
	ProducerRequestTimeout    int           `yaml:"producerRequestTimeout" default:"10000"` // 默认10秒
	ProducerVersion           string        `yaml:"producerVersion"`                        // kafka版本，仅在v2生效
	ConsumerWorkerNum         int           `yaml:"consumerWorkerNum" default:"2"`
	ConsumerHeartBeatInterval time.Duration `yaml:"consumerHeartBeatInterval" default:"6s"`
	ConsumerSessionTimeout    time.Duration `yaml:"consumerSessionTimeout" default:"20s"`
	ConsumerAutoOffsetReset   string        `yaml:"consumerAutoOffsetReset" default:"latest"` // https://www.cnblogs.com/convict/p/16701154.html
	ConsumerQueueSize         int           `yaml:"consumerQueueSize" default:"500"`          // 消费者每个缓存队列大小，默认1000，仅v2生效
	ConsumerCommitInterval    time.Duration `yaml:"consumerCommitInterval" default:"1s"`      // 提交offset间隔
	ConsumerFetchMaxBytes     int           `yaml:"consumerFetchMaxBytes" default:"52428800"` // 一次抓取消息的最大大小，要确保这个大小要大于一条消息的最大大小，默认50MB，如果内存占用过高，可以适当调小
	SecurityProtocol          string        `yaml:"securityProtocol"`
	SaslMechanism             string        `yaml:"saslMechanism"`
	SaslUsername              string        `yaml:"saslUsername"`
	SaslPassword              string        `yaml:"saslPassword"`
}

type Consumer interface {
	Listen(fn func(message interface{}))
	Close()
}

type Producer interface {
	Send(topic string, key string, values ...interface{}) error
	AsyncSend(topic string, key string, values ...interface{}) error
	Close()
}
