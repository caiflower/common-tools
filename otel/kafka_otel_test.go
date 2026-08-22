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
	"errors"
	"testing"

	xkafka "github.com/caiflower/common-tools/kafka"
	golocalv1 "github.com/caiflower/common-tools/pkg/golocal/v1"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/codes"
)

func TestKafkaProducerSpanBatch(t *testing.T) {
	recorder := setupRecordingWebClient(t)
	golocalv1.PutTraceID("0123456789abcdef0123456789abcdef")
	defer golocalv1.Clean()

	span := StartKafkaProducerSpan("orders", "key-1", 2)
	assert.NotNil(t, span)

	span.Complete(nil)
	assert.Len(t, recorder.Ended(), 1)

	span.MessageDone(nil)
	span.MessageDone(errors.New("delivery failed"))

	spans := recorder.Ended()
	assert.Len(t, spans, 1)
	if len(spans) != 1 {
		return
	}

	attrs := make(map[string]string)
	for _, attr := range spans[0].Attributes() {
		attrs[string(attr.Key)] = attr.Value.Emit()
	}
	assert.Equal(t, "kafka", attrs["messaging.system"])
	assert.Equal(t, "send", attrs["messaging.operation"])
	assert.Equal(t, "orders", attrs["messaging.destination"])
	assert.Equal(t, "key-1", attrs["messaging.kafka.message_key"])
	assert.Equal(t, "2", attrs["messaging.batch.message_count"])
}

func TestKafkaProducerSpanMessageDoneDoesNotAffectComplete(t *testing.T) {
	recorder := setupRecordingWebClient(t)
	golocalv1.PutTraceID("0123456789abcdef0123456789abcdef")
	defer golocalv1.Clean()

	span := StartKafkaProducerSpan("orders", "", 2)
	span.MessageDone(errors.New("delivery failed"))
	span.Complete(nil)

	spans := recorder.Ended()
	assert.Len(t, spans, 1)
	if len(spans) != 1 {
		return
	}
	assert.Equal(t, codes.Unset, spans[0].Status().Code)
}

func TestKafkaProducerSpanCompleteIsIdempotent(t *testing.T) {
	recorder := setupRecordingWebClient(t)
	golocalv1.PutTraceID("0123456789abcdef0123456789abcdef")
	defer golocalv1.Clean()

	span := StartKafkaProducerSpan("orders", "", 1)
	span.MessageDone(nil)
	span.Complete(nil)
	span.Complete(errors.New("ignored"))

	assert.Len(t, recorder.Ended(), 1)
}

func TestKafkaProducerSpanCompleteErrorEndsImmediately(t *testing.T) {
	recorder := setupRecordingWebClient(t)
	golocalv1.PutTraceID("0123456789abcdef0123456789abcdef")
	defer golocalv1.Clean()

	span := StartKafkaProducerSpan("orders", "", 3)
	span.Complete(errors.New("enqueue failed"))

	assert.Len(t, recorder.Ended(), 1)
}

func TestKafkaProducerSpanDisabled(t *testing.T) {
	oldClient := DefaultClient
	DefaultClient = &client{}
	defer func() { DefaultClient = oldClient }()

	assert.Nil(t, StartKafkaProducerSpan("orders", "", 1))
	var span *KafkaProducerSpan
	assert.NotPanics(t, func() {
		span.Complete(nil)
		span.MessageDone(nil)
	})
}

func TestNewKafkaProducerHookDisabled(t *testing.T) {
	oldClient := DefaultClient
	DefaultClient = &client{}
	defer func() { DefaultClient = oldClient }()

	hook := NewKafkaProducerHook()
	assert.NotNil(t, hook)
	assert.Nil(t, hook.BeforeSend("orders", "", []interface{}{"value"}))

	var _ xkafka.ProducerHook = hook
}
