package xkafka

import "time"

type Config struct {
	Name                      string        `yaml:"name"`
	Enable                    string        `yaml:"enable" default:"true"` // 默认开启
	BootstrapServers          []string      `yaml:"bootstrap_servers"`
	GroupID                   string        `yaml:"group_id"`
	Topics                    []string      `yaml:"topics"`
	ProducerAcks              int           `yaml:"producer_acks" default:"-1"`
	ProducerCompressType      string        `yaml:"producer_compress_type" default:"none"`    // none, gzip, snappy, lz4, zstd
	ProducerMessageTimeout    int           `yaml:"producer_message_timeout" default:"15000"` // 默认15秒, 仅在v1生效
	ProducerRequestTimeout    int           `yaml:"producer_request_timeout" default:"10000"` // 默认10秒
	ProducerVersion           string        `yaml:"producer_version"`                         // kafka版本，仅在v2生效
	ConsumerWorkerNum         int           `yaml:"consumer_worker_num" default:"2"`
	ConsumerHeartBeatInterval time.Duration `yaml:"consumer_heart_beat_interval" default:"6s"`
	ConsumerSessionTimeout    time.Duration `yaml:"consumer_session_timeout" default:"20s"`
	ConsumerAutoOffsetReset   string        `yaml:"consumer_auto_offset_reset" default:"latest"` // https://www.cnblogs.com/convict/p/16701154.html
	ConsumerQueueSize         int           `yaml:"consumer_queue_size" default:"500"`           // 消费者每个缓存队列大小，默认1000，仅v2生效
	ConsumerCommitInterval    time.Duration `yaml:"consumer_commit_interval" default:"1s"`       // 提交offset间隔，仅v2生效
	ConsumerFetchMaxBytes     int           `yaml:"consumer_fetch_max_bytes" default:"52428800"` // 一次抓取消息的最大大小，要确保这个大小要大于一条消息的最大大小，默认50MB，如果内存占用过高，可以适当调小
	SecurityProtocol          string        `yaml:"security_protocol"`
	SaslMechanism             string        `yaml:"sasl_mechanism"`
	SaslUsername              string        `yaml:"sasl_username"`
	SaslPassword              string        `yaml:"sasl_password"`
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
