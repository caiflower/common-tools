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
	"testing"

	golocalv1 "github.com/caiflower/common-tools/pkg/golocal/v1"
	"github.com/stretchr/testify/assert"
)

func TestGetTraceIDPreferGolocal(t *testing.T) {
	golocalv1.Clean()
	golocalv1.PutTraceID("golocal-trace")
	defer golocalv1.Clean()

	ctx := context.WithValue(context.Background(), traceIDKey, "ctx-trace")
	assert.Equal(t, "golocal-trace", getTraceID(ctx))
}

func TestGetTraceIDFallbackToContext(t *testing.T) {
	golocalv1.Clean()
	defer golocalv1.Clean()

	ctx := context.WithValue(context.Background(), traceIDKey, "ctx-trace")
	assert.Equal(t, "ctx-trace", getTraceID(ctx))
}

func TestGetTraceIDEmptyWhenUnavailable(t *testing.T) {
	golocalv1.Clean()
	defer golocalv1.Clean()

	ctx := context.WithValue(context.Background(), traceIDKey, 123)
	assert.Equal(t, "", getTraceID(ctx))
}
