package v2

import (
	"errors"
	"reflect"
	"strings"
	"time"

	"github.com/Shopify/sarama"
	"github.com/caiflower/common-tools/global"
	xkafka "github.com/caiflower/common-tools/kafka"
	"github.com/caiflower/common-tools/pkg/e"
	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/caiflower/common-tools/pkg/tools"
)

func NewProducerClient(cfg xkafka.Config) *KafkaClient {
	_ = tools.DoTagFunc(&cfg, nil, []func(reflect.StructField, reflect.Value, interface{}) error{tools.SetDefaultValueIfNil})

	if strings.ToUpper(cfg.Enable) != "TRUE" {
		logger.Warn("[kafka-product] producer '%s' is disable", cfg.Name)
		return &KafkaClient{}
	}

	config := sarama.NewConfig()
	config.ClientID = tools.MD5(cfg.Name + "_p")
	config.Producer.Return.Successes = true
	config.Producer.Return.Errors = true

	switch cfg.ProducerAcks {
	case -1:
		config.Producer.RequiredAcks = sarama.WaitForAll
	case 1:
		config.Producer.RequiredAcks = sarama.WaitForLocal
	case 0:
		config.Producer.RequiredAcks = sarama.NoResponse
	default:
		config.Producer.RequiredAcks = sarama.WaitForAll
	}

	second := time.Duration(cfg.ProducerRequestTimeout / 1000)
	config.Producer.Timeout = time.Second * second
	switch cfg.ProducerCompressType {
	case "none":

	case "gzip":
		config.Producer.Compression = sarama.CompressionGZIP
	case "snappy":
		config.Producer.Compression = sarama.CompressionSnappy
	case "lz4":
		config.Producer.Compression = sarama.CompressionLZ4
	case "zstd":
		config.Producer.Compression = sarama.CompressionZSTD
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
		running:      true,
	}
	logger.Info("[kafka-product] producer '%s' config: %s", cfg.Name, tools.ToJson(cfg))

	kafkaClient.openSyncProducer()
	kafkaClient.openASyncProducer()

	global.DefaultResourceManger.Add(kafkaClient)
	return kafkaClient
}

func (c *KafkaClient) openSyncProducer() {
	config := c.saramaConfig
	var producer sarama.SyncProducer
	var err error
label:
	for {
		if c.running == false {
			return
		}
		c.resetRetryVersion()
		for i := 1; i <= len(retryVersions); i++ {
			config.Version = c.getRetryVersion()
			if producer, err = sarama.NewSyncProducer(c.cfg.BootstrapServers, config); err == nil {
				break label
			}
		}
		if err != nil {
			addProducerErrCount(c.cfg, "", "connect")
			logger.Error("Open sync producer failed. name=%s. err=%v", c.cfg.Name, err)
			time.Sleep(time.Second)
		}
	}
	c.syncProducer = producer
}

func (c *KafkaClient) openASyncProducer() {
	config := c.saramaConfig
	var producer sarama.AsyncProducer

label:
	for {
		var err error
		if c.running == false {
			return
		}
		c.resetRetryVersion()
		for i := 1; i <= len(retryVersions); i++ {
			config.Version = c.getRetryVersion()
			if producer, err = sarama.NewAsyncProducer(c.cfg.BootstrapServers, config); err == nil {
				break label
			}
		}
		if err != nil {
			addProducerErrCount(c.cfg, "", "connect")
			logger.Error("Open async producer failed. name=%s. err=%v", c.cfg.Name, err)
			time.Sleep(time.Second)
		}
	}

	c.asyncProducer = producer
	go func(p sarama.AsyncProducer) {
		defer e.OnError("")
		errorChan := p.Errors()
		success := p.Successes()
	fe:
		for {
			select {
			case err, ok := <-errorChan:
				if !ok {
					logger.Info("KafkaAsyncProducer %s chan errors is closed. exit.", c.cfg.Name)
					break fe
				}
				if err != nil {
					addProducerErrCount(c.cfg, err.Msg.Topic, "async")
					logger.Error("KafkaAsyncProducer %s got error. %s", c.cfg.Name, err)
				}
			case <-success:
			}
		}
	}(producer)
}

func (c *KafkaClient) Send(topic, key string, values ...interface{}) error {
	if strings.ToUpper(c.cfg.Enable) != "TRUE" || len(values) == 0 {
		return nil
	}

	if topic == "" {
		return errors.New("sync producer cannot send message without topic")
	}
	if c.syncProducer == nil {
		return errors.New("sync producer no connected yet")
	}

	var msgs []*sarama.ProducerMessage
	for _, v := range values {
		bytes, _ := tools.ToByte(v)

		msg := &sarama.ProducerMessage{
			Topic: topic,
			Value: sarama.ByteEncoder(bytes),
		}
		if key != "" {
			msg.Key = sarama.ByteEncoder(key)
		}
		countProducer(c.cfg, topic)

		msgs = append(msgs, msg)
	}

	for i := 1; i <= 3; i++ {
		err := c.syncProducer.SendMessages(msgs)
		if err == nil {
			break
		} else if i != 3 {
			addProducerErrCount(c.cfg, topic, "sync")
			continue
		} else {
			addProducerErrCount(c.cfg, topic, "sync")
			return err
		}
	}

	return nil
}

func (c *KafkaClient) AsyncSend(topic, key string, values ...interface{}) error {
	if strings.ToUpper(c.cfg.Enable) != "TRUE" || len(values) == 0 {
		return nil
	}

	if topic == "" {
		return errors.New("async producer cannot send message without topic")
	}
	if c.asyncProducer == nil {
		return errors.New("async producer no connected yet")
	}

	for _, v := range values {

		bytes, _ := tools.ToByte(v)
		msg := &sarama.ProducerMessage{
			Topic: topic,
			Value: sarama.ByteEncoder(bytes),
		}
		if key != "" {
			msg.Key = sarama.ByteEncoder(key)
		}

		countProducer(c.cfg, topic)
		c.asyncProducer.Input() <- msg
	}
	return nil
}

func (c *KafkaClient) resetRetryVersion() {
	c.retryVersions = map[string]interface{}{}
}

// 获得版本
func (c *KafkaClient) getRetryVersion() sarama.KafkaVersion {
	// 明确指定了版本，直接使用
	if c.cfg.ProducerVersion != "" {
		version, vErr := sarama.ParseKafkaVersion(c.cfg.ProducerVersion)
		if vErr != nil {
			panic(vErr)
		}
		return version
	}
	if c.retryVersions == nil {
		c.resetRetryVersion()
	}

	// 使用预置的版本进行偿试
	for v, _ := range retryVersions {
		if _, ok := c.retryVersions[v]; !ok {
			c.retryVersions[v] = struct{}{}
			version, vErr := sarama.ParseKafkaVersion(v)
			if vErr != nil {
				panic(vErr)
			}
			return version
		}
	}
	panic(errors.New("all version retry failed"))
}

// 没有指定版本时，尝试使用以下版本
var retryVersions = map[string]interface{}{
	"2.6.0":    struct{}{},
	"0.10.2.1": struct{}{},
}
