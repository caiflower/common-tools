//go:build integration

package v2

import (
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/IBM/sarama/mocks"
	xkafka "github.com/caiflower/common-tools/kafka"
	"github.com/stretchr/testify/assert"
)

const testTopic = "test-topic"

func newMockBrokerWithHandlers(t *testing.T, handlerMapFunc func(brokerAddr string, brokerID int32) map[string]sarama.MockResponse) *sarama.MockBroker {
	t.Helper()
	broker := sarama.NewMockBroker(t, 1)
	handlerMap := handlerMapFunc(broker.Addr(), broker.BrokerID())
	broker.SetHandlerByMap(handlerMap)
	return broker
}

func producerHandlerMap(brokerAddr string, brokerID int32) map[string]sarama.MockResponse {
	return map[string]sarama.MockResponse{
		"ApiVersionsRequest": sarama.NewMockApiVersionsResponse(nil),
		"MetadataRequest": sarama.NewMockMetadataResponse(nil).
			SetBroker(brokerAddr, brokerID).
			SetLeader(testTopic, 0, brokerID),
		"ProduceRequest": sarama.NewMockProduceResponse(nil).
			SetError(testTopic, 0, sarama.ErrNoError),
	}
}

func idempotentProducerHandlerMap(brokerAddr string, brokerID int32) map[string]sarama.MockResponse {
	return map[string]sarama.MockResponse{
		"ApiVersionsRequest": sarama.NewMockApiVersionsResponse(nil),
		"MetadataRequest": sarama.NewMockMetadataResponse(nil).
			SetBroker(brokerAddr, brokerID).
			SetLeader(testTopic, 0, brokerID),
		"InitProducerIDRequest": sarama.NewMockInitProducerIDResponse(nil),
		"ProduceRequest": sarama.NewMockProduceResponse(nil).
			SetError(testTopic, 0, sarama.ErrNoError),
	}
}

func errorProducerHandlerMap(brokerAddr string, brokerID int32) map[string]sarama.MockResponse {
	return map[string]sarama.MockResponse{
		"ApiVersionsRequest": sarama.NewMockApiVersionsResponse(nil),
		"MetadataRequest": sarama.NewMockMetadataResponse(nil).
			SetBroker(brokerAddr, brokerID).
			SetLeader(testTopic, 0, brokerID),
		"ProduceRequest": sarama.NewMockProduceResponse(nil).
			SetError(testTopic, 0, sarama.ErrNotLeaderForPartition),
	}
}

func newTestProducerConfig(brokerAddr string) xkafka.Config {
	return xkafka.Config{
		Name:                   "itest-producer",
		Enable:                 "true",
		BootstrapServers:       []string{brokerAddr},
		ProducerAcks:           -1,
		ProducerCompressType:   "none",
		ProducerRequestTimeout: 10 * time.Second,
		ProducerVersion:        "2.6.0",
		ProducerIdempotence:    false,
	}
}

func newTestIdempotentProducerConfig(brokerAddr string) xkafka.Config {
	return xkafka.Config{
		Name:                   "itest-idempotent-producer",
		Enable:                 "true",
		BootstrapServers:       []string{brokerAddr},
		ProducerAcks:           -1,
		ProducerCompressType:   "none",
		ProducerRequestTimeout: 10 * time.Second,
		ProducerVersion:        "2.6.0",
		ProducerIdempotence:    true,
	}
}

func TestV2SyncProducer_SendViaMockBroker(t *testing.T) {
	broker := newMockBrokerWithHandlers(t, producerHandlerMap)
	defer broker.Close()

	cfg := newTestProducerConfig(broker.Addr())
	client := NewProducerClient(cfg)
	defer client.Close()

	err := client.Send(testTopic, "key1", "hello sync")
	assert.NoError(t, err)
}

func TestV2AsyncProducer_SendViaMockBroker(t *testing.T) {
	broker := newMockBrokerWithHandlers(t, producerHandlerMap)
	defer broker.Close()

	cfg := newTestProducerConfig(broker.Addr())
	client := NewProducerClient(cfg)
	defer client.Close()

	err := client.AsyncSend(testTopic, "key1", "hello async")
	assert.NoError(t, err)
}

// TestV2SyncProducer_IdempotentViaMockBroker verifies that a SyncProducer with
// ProducerIdempotence=true can successfully send messages via MockBroker.
// Note: MockBroker does not validate idempotence semantics (e.g. duplicate
// deduplication); it only confirms the InitProducerID + Produce protocol flow
// completes without error.
func TestV2SyncProducer_IdempotentViaMockBroker(t *testing.T) {
	broker := newMockBrokerWithHandlers(t, idempotentProducerHandlerMap)
	defer broker.Close()

	cfg := newTestIdempotentProducerConfig(broker.Addr())
	client := NewProducerClient(cfg)
	defer client.Close()

	err := client.Send(testTopic, "key1", "idempotent-msg")
	assert.NoError(t, err)
}

// TestV2AsyncProducer_IdempotentViaMockBroker verifies that an AsyncProducer
// with ProducerIdempotence=true can successfully send messages via MockBroker.
// Note: MockBroker does not validate idempotence semantics; see
// TestV2SyncProducer_IdempotentViaMockBroker for details.
func TestV2AsyncProducer_IdempotentViaMockBroker(t *testing.T) {
	broker := newMockBrokerWithHandlers(t, idempotentProducerHandlerMap)
	defer broker.Close()

	cfg := newTestIdempotentProducerConfig(broker.Addr())
	client := NewProducerClient(cfg)
	defer client.Close()

	err := client.AsyncSend(testTopic, "key1", "idempotent-async-msg")
	assert.NoError(t, err)
}

func TestV2SyncProducer_IdempotentWithMocks(t *testing.T) {
	config := sarama.NewConfig()
	config.Producer.Idempotent = true
	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Return.Successes = true
	config.Producer.Return.Errors = true
	config.Net.MaxOpenRequests = 1

	mockProducer := mocks.NewSyncProducer(t, config)
	defer mockProducer.Close()

	mockProducer.ExpectSendMessageAndSucceed()

	msg := &sarama.ProducerMessage{
		Topic: testTopic,
		Value: sarama.StringEncoder("idempotent-msg"),
	}
	_, _, err := mockProducer.SendMessage(msg)
	assert.NoError(t, err)
}

func TestV2AsyncProducer_IdempotentWithMocks(t *testing.T) {
	config := sarama.NewConfig()
	config.Producer.Idempotent = true
	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Return.Successes = true
	config.Producer.Return.Errors = true
	config.Net.MaxOpenRequests = 1

	mockProducer := mocks.NewAsyncProducer(t, config)
	defer mockProducer.Close()

	mockProducer.ExpectInputAndSucceed()

	msg := &sarama.ProducerMessage{
		Topic: testTopic,
		Value: sarama.StringEncoder("idempotent-async-msg"),
	}
	mockProducer.Input() <- msg

	select {
	case success := <-mockProducer.Successes():
		assert.Equal(t, testTopic, success.Topic)
	case err := <-mockProducer.Errors():
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestV2SyncProducer_IdempotenceConfigOverride verifies that when
// ProducerIdempotence=true, the internal sarama config is correctly set
// regardless of the ProducerAcks value in xkafka.Config.
func TestV2SyncProducer_IdempotenceConfigOverride(t *testing.T) {
	broker := newMockBrokerWithHandlers(t, idempotentProducerHandlerMap)
	defer broker.Close()

	cfg := xkafka.Config{
		Name:                   "itest-idempotent-override",
		Enable:                 "true",
		BootstrapServers:       []string{broker.Addr()},
		ProducerAcks:           1,
		ProducerIdempotence:    true,
		ProducerVersion:        "2.6.0",
		ProducerRequestTimeout: 10 * time.Second,
	}

	client := NewProducerClient(cfg)
	defer client.Close()

	assert.True(t, client.saramaConfig.Producer.Idempotent)
	assert.Equal(t, sarama.WaitForAll, client.saramaConfig.Producer.RequiredAcks)
	assert.Equal(t, 1, client.saramaConfig.Net.MaxOpenRequests)
}

// TestV2SyncProducer_ProduceErrorViaMockBroker verifies that a SyncProducer
// returns an error when the MockBroker responds with a ProduceError.
func TestV2SyncProducer_ProduceErrorViaMockBroker(t *testing.T) {
	broker := newMockBrokerWithHandlers(t, errorProducerHandlerMap)
	defer broker.Close()

	cfg := newTestProducerConfig(broker.Addr())
	client := NewProducerClient(cfg)
	defer client.Close()

	err := client.Send(testTopic, "key1", "will-fail")
	assert.Error(t, err)
}
