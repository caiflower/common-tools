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

 package v2

import (
	"fmt"
	"testing"
	"time"

	xkafka "github.com/caiflower/common-tools/kafka"
	"github.com/caiflower/common-tools/pkg/basic"
	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/caiflower/common-tools/pkg/tools"
)

type Message struct {
	Name string
	Age  int
	Time basic.Time
}

func TestMock(t *testing.T) {
	logger.InitLogger(&logger.Config{
		Level: logger.DebugLevel,
	})

	consumerConfig := xkafka.Config{
		Name:             "consumer",
		Enable:           "true",
		BootstrapServers: []string{"kafaka-kafka.app.svc.cluster.local:9092"},
		GroupID:          "test",
		Topics:           []string{"testTopic"},
		SecurityProtocol: "SASL_PLAINTEXT",
		SaslMechanism:    "SCRAM-SHA-256",
		SaslUsername:     "user1",
		SaslPassword:     "3JVZWh98fe",
	}
	cClient := NewConsumerClient(consumerConfig)
	cClient.Listen(func(message interface{}) {
		if v, ok := message.(*KafkaMessage); ok {
			m := Message{}
			if err := tools.Unmarshal(v.Value, &m); err != nil {
				fmt.Println(err)
			}
			fmt.Println(tools.ToJson(m))
		} else {
			fmt.Println("unknown message")
		}

	})

	defer cClient.Close()

	time.Sleep(10 * time.Second)

	productConfig := xkafka.Config{
		Name:                 "product",
		Enable:               "true",
		BootstrapServers:     []string{"kafaka-kafka.app.svc.cluster.local:9092"},
		GroupID:              "testGroup",
		Topics:               []string{"testTopic"},
		SecurityProtocol:     "SASL_PLAINTEXT",
		SaslMechanism:        "SCRAM-SHA-256",
		SaslUsername:         "user1",
		SaslPassword:         "3JVZWh98fe",
		ProducerCompressType: "gzip",
	}
	pClient := NewProducerClient(productConfig)
	defer pClient.Close()

	if err := pClient.Send(productConfig.Topics[0], "", Message{
		Name: "syncSend",
		Age:  10,
		Time: basic.Time(time.Now()),
	}, Message{
		Name: "syncSend",
		Age:  11,
		Time: basic.Time(time.Now()),
	}); err != nil {
		panic(err)
	}

	for i := 0; i < 10; i++ {
		go func(age int) {
			if err := pClient.AsyncSend(productConfig.Topics[0], "", &Message{
				Name: "asyncSend",
				Age:  age,
				Time: basic.Time(time.Now()),
			}); err != nil {
				panic(err)
			}
		}(i)
	}

	time.Sleep(10 * time.Second)
}

func TestMock1(t *testing.T) {
	logger.InitLogger(&logger.Config{
		Level: logger.DebugLevel,
	})

	consumerConfig := xkafka.Config{
		Name:                    "consumer",
		Enable:                  "true",
		BootstrapServers:        []string{"kafaka-kafka.app.svc.cluster.local:9092"},
		GroupID:                 "test100",
		Topics:                  []string{"testTopic"},
		SecurityProtocol:        "SASL_PLAINTEXT",
		SaslMechanism:           "SCRAM-SHA-256",
		SaslUsername:            "user1",
		SaslPassword:            "3JVZWh98fe",
		ConsumerAutoOffsetReset: "earliest",
	}
	cClient := NewConsumerClient(consumerConfig)
	cClient.Listen(func(message interface{}) {
		if v, ok := message.(*KafkaMessage); ok {
			m := Message{}
			if err := tools.Unmarshal(v.Value, &m); err != nil {
				fmt.Println(err)
			}
			fmt.Println(tools.ToJson(m))
		} else {
			fmt.Println("unknown message")
		}

	})

	defer cClient.Close()

	time.Sleep(10 * time.Second)
}
