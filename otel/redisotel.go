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
	"net"
	"strings"

	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/redis/go-redis/v9"
	"github.com/uptrace/uptrace-go/uptrace"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

var tracer = otel.Tracer("github.com/redis/go-redis/v9")

type TracingHook struct{}

var _ redis.Hook = (*TracingHook)(nil)

func NewTracingHook() *TracingHook {
	return new(TracingHook)
}

func spanFromContext(ctx context.Context, name string) (context.Context, trace.Span) {
	_traceID := getTraceID(ctx)
	traceID, err := trace.TraceIDFromHex(_traceID)
	if err != nil {
		logger.Error("trace.TraceIDFromHex failed. Error: %v", err)
	}
	ctx, span := tracer.Start(trace.ContextWithSpanContext(context.Background(), trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: traceID,
	})), name, trace.WithSpanKind(trace.SpanKindClient))
	return ctx, span
}

// DialHook implements the v9 redis.Hook interface. It passes through to the next dialer.
func (TracingHook) DialHook(next redis.DialHook) redis.DialHook {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		return next(ctx, network, addr)
	}
}

// ProcessHook implements the v9 redis.Hook interface using the middleware/wrapper pattern.
// It creates a tracing span before the command executes and ends it after.
func (TracingHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		ctx, span := spanFromContext(ctx, cmd.FullName())
		defer span.End()

		if span.IsRecording() {
			span.SetAttributes(
				attribute.String("db.system", "redis"),
				attribute.String("db.statement", cmd.String()),
			)
		}

		err := next(ctx, cmd)

		if span.IsRecording() {
			if cmdErr := cmd.Err(); cmdErr != nil && cmdErr != redis.Nil {
				recordError(span, cmdErr)
			}
			logger.Trace("uptrace: %s\n", uptrace.TraceURL(span))
		}

		return err
	}
}

// ProcessPipelineHook implements the v9 redis.Hook interface for pipeline commands.
func (TracingHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []redis.Cmder) error {
		summary := pipelineSummary(cmds)
		ctx, span := spanFromContext(ctx, "pipeline "+summary)
		defer span.End()

		if span.IsRecording() {
			span.SetAttributes(
				attribute.String("db.system", "redis"),
				attribute.Int("db.redis.num_cmd", len(cmds)),
				attribute.String("db.statement", pipelineCmdsString(cmds)),
			)
		}

		err := next(ctx, cmds)

		if span.IsRecording() {
			if len(cmds) > 0 {
				if cmdErr := cmds[0].Err(); cmdErr != nil && cmdErr != redis.Nil {
					recordError(span, cmdErr)
				}
			}
			logger.Trace("uptrace: %s\n", uptrace.TraceURL(span))
		}

		return err
	}
}

func recordError(span trace.Span, err error) {
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
}

// pipelineSummary returns a brief summary of the pipeline commands (first cmd name + count).
func pipelineSummary(cmds []redis.Cmder) string {
	if len(cmds) == 0 {
		return "empty"
	}
	return cmds[0].FullName()
}

// pipelineCmdsString formats all pipeline commands into a single string.
func pipelineCmdsString(cmds []redis.Cmder) string {
	var sb strings.Builder
	for i, cmd := range cmds {
		if i > 0 {
			sb.WriteByte('\n')
		}
		sb.WriteString(cmd.String())
	}
	return sb.String()
}
