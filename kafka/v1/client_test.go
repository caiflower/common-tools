package v1

import (
	"fmt"
	"testing"
	"time"
)

type Message struct {
	Name string
	Age  int
}

func TestMock(t *testing.T) {
	productConfig := Config{
		Name:             "product",
		Enable:           true,
		BootstrapServers: []string{"app.caiflower.cn:9094"},
		GroupID:          "testGroup",
		Topics:           []string{"testTopic"},
		SecurityProtocol: "SASL_PLAINTEXT",
		SaslMechanism:    "SCRAM-SHA-256",
		SaslUsername:     "user1",
		SaslPassword:     "IFh3YqTg1J",
	}
	pClient := NewProducerClient(productConfig)
	if err := pClient.Send(&Message{
		Name: "testName",
		Age:  10,
	}); err != nil {
		panic(err)
	}

	consumerConfig := Config{
		Name:             "consumer",
		Enable:           true,
		BootstrapServers: []string{"app.caiflower.cn:9094"},
		GroupID:          "testGroup",
		Topics:           []string{"testTopic"},
		SecurityProtocol: "SASL_PLAINTEXT",
		SaslMechanism:    "SCRAM-SHA-256",
		SaslUsername:     "user1",
		SaslPassword:     "IFh3YqTg1J",
	}
	cClient := NewConsumerClient(consumerConfig)
	cClient.Listen(func(message interface{}) {
		m := message.(*Message)
		fmt.Println(m)
	})

	time.Sleep(20 * time.Second)
}
