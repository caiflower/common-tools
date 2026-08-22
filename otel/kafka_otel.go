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

package otel

import (
	"context"

	xkafka "github.com/caiflower/common-tools/kafka"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.10.0"
	"go.opentelemetry.io/otel/trace"
)

const kafkaProducerTracerName = "github.com/caiflower/common-tools/kafka"

type kafkaProducerHook struct{}

var _ xkafka.ProducerHook = (*kafkaProducerHook)(nil)
var _ xkafka.ProducerContext = (*KafkaProducerSpan)(nil)

// KafkaProducerSpan represents one producer send operation. Complete ends the
// span as soon as Send or AsyncSend returns; per-message delivery results are
// intentionally not part of the send span.
type KafkaProducerSpan struct {
	span   trace.Span
	failed error
}

// NewKafkaProducerHook returns a ProducerHook that enables OpenTelemetry
// tracing for kafka producers. Callers opt in by registering it with
// producer.AddHook(otel.NewKafkaProducerHook()).
func NewKafkaProducerHook() xkafka.ProducerHook {
	return &kafkaProducerHook{}
}

func (h *kafkaProducerHook) BeforeSend(topic, key string, values []interface{}) xkafka.ProducerContext {
	return StartKafkaProducerSpan(topic, key, len(values))
}

func StartKafkaProducerSpan(topic, key string, messageCount int) *KafkaProducerSpan {
	if !IsEnabled() || messageCount <= 0 {
		return nil
	}

	span := DefaultClient.Start(getTraceID(context.Background()), kafkaProducerTracerName, "send "+topic, trace.SpanKindProducer)
	if span == nil || !span.IsRecording() {
		return nil
	}

	attrs := []attribute.KeyValue{
		semconv.MessagingSystemKey.String("kafka"),
		semconv.MessagingOperationKey.String("send"),
		semconv.MessagingDestinationKey.String(topic),
		semconv.MessagingDestinationKindTopic,
		attribute.Int("messaging.batch.message_count", messageCount),
	}
	if key != "" {
		attrs = append(attrs, semconv.MessagingKafkaMessageKeyKey.String(key))
	}
	span.SetAttributes(attrs...)

	return &KafkaProducerSpan{
		span: span,
	}
}

func (span *KafkaProducerSpan) Complete(err error) {
	if span == nil {
		return
	}

	if err != nil && span.failed == nil {
		span.failed = err
	}
	failed := span.failed

	DefaultClient.End(span.span, &Content{Failed: failed})
}

// MessageDone is a no-op because the span is finished by Complete and does not
// wait for per-message delivery results.
func (span *KafkaProducerSpan) MessageDone(err error) {}
