package v1

import (
	"fmt"
	"testing"
	"time"

	"github.com/caiflower/common-tools/pkg/basic"
	"github.com/caiflower/common-tools/pkg/tools"
)

type Message struct {
	Name string
	Age  int
	Time basic.Time
}

func TestMock(t *testing.T) {
	productConfig := Config{
		Name:             "consumer",
		Enable:           true,
		BootstrapServers: []string{"10.226.132.131:9092"},
		GroupID:          "testGroup",
		Topics:           []string{"testTopic"},
		//SecurityProtocol: "SASL_PLAINTEXT",
		//SaslMechanism:    "SCRAM-SHA-256",
		//SaslUsername:     "user1",
		//SaslPassword:     "IFh3YqTg1J",
	}
	pClient := NewProducerClient(productConfig)
	if err := pClient.Send(&Message{
		Name: "syncSend",
		Age:  10,
		Time: basic.Time(time.Now()),
	}); err != nil {
		panic(err)
	}

	if err := pClient.AsyncSend(&Message{
		Name: "asyncSend",
		Age:  10,
		Time: basic.Time(time.Now()),
	}); err != nil {
		panic(err)
	}
	pClient.Close()

	consumerConfig := Config{
		Name:             "consumer",
		Enable:           true,
		BootstrapServers: []string{"10.226.132.131:9092"},
		GroupID:          "testGroup",
		Topics:           []string{"testTopic"},
		//SecurityProtocol: "SASL_PLAINTEXT",
		//SaslMechanism:    "SCRAM-SHA-256",
		//SaslUsername:     "user1",
		//SaslPassword:     "IFh3YqTg1J",
	}
	cClient := NewConsumerClient(consumerConfig)
	cClient.Listen(func(message interface{}) {
		m := Message{}
		if err := tools.Unmarshal(message.([]byte), &m); err != nil {
			fmt.Println(err)
		}
		fmt.Println(tools.ToJson(m))
	})

	time.Sleep(20 * time.Second)
	cClient.Close()

	time.Sleep(2 * time.Second)
}
