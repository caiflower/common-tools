package v2

import (
	"testing"

	"github.com/IBM/sarama"
	xkafka "github.com/caiflower/common-tools/kafka"
)

func TestHandleAsyncProducerErrorIgnoresNilError(t *testing.T) {
	handleAsyncProducerError(&xkafka.Config{}, nil)
}

func TestHandleAsyncProducerErrorIgnoresNilMessage(t *testing.T) {
	handleAsyncProducerError(&xkafka.Config{}, &sarama.ProducerError{})
}
