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

 package telemetry

import (
	"context"
	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/uptrace/uptrace-go/uptrace"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/go-redis/redis/extra/rediscmd/v8"
	"github.com/go-redis/redis/v8"
)

var tracer = otel.Tracer("github.com/go-redis/redis")

type TracingHook struct{}

var _ redis.Hook = (*TracingHook)(nil)

func NewTracingHook() *TracingHook {
	return new(TracingHook)
}

func spanFromContext(ctx context.Context, name string) (context.Context, trace.Span) {
	traceId := ctx.Value("traceId").(string)
	traceID, err := trace.TraceIDFromHex(traceId)
	if err != nil {
		logger.Error("trace.TraceIDFromHex failed. Error: %v", err)
	}
	ctx, span := tracer.Start(trace.ContextWithSpanContext(context.Background(), trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: traceID,
	})), name, trace.WithSpanKind(trace.SpanKindClient))
	return ctx, span
}

func (TracingHook) BeforeProcess(ctx context.Context, cmd redis.Cmder) (context.Context, error) {
	ctx, span := spanFromContext(ctx, cmd.FullName())
	if !span.IsRecording() {
		return ctx, nil
	}

	span.SetAttributes(
		attribute.String("db.system", "redis"),
		attribute.String("db.statement", rediscmd.CmdString(cmd)),
	)

	return ctx, nil
}

func (TracingHook) AfterProcess(ctx context.Context, cmd redis.Cmder) error {
	span := trace.SpanFromContext(ctx)
	defer span.End()
	if !span.IsRecording() {
		return nil
	}

	if err := cmd.Err(); err != nil {
		recordError(ctx, span, err)
	}

	logger.Trace("uptrace: %s\n", uptrace.TraceURL(span))
	return nil
}

func (TracingHook) BeforeProcessPipeline(ctx context.Context, cmds []redis.Cmder) (context.Context, error) {
	summary, cmdsString := rediscmd.CmdsString(cmds)

	ctx, span := spanFromContext(ctx, "pipeline "+summary)
	if !span.IsRecording() {
		return ctx, nil
	}

	span.SetAttributes(
		attribute.String("db.system", "redis"),
		attribute.Int("db.redis.num_cmd", len(cmds)),
		attribute.String("db.statement", cmdsString),
	)

	return ctx, nil
}

func (TracingHook) AfterProcessPipeline(ctx context.Context, cmds []redis.Cmder) error {
	span := trace.SpanFromContext(ctx)
	defer span.End()
	if !span.IsRecording() {
		return nil
	}

	if err := cmds[0].Err(); err != nil {
		recordError(ctx, span, err)
	}

	logger.Trace("uptrace: %s\n", uptrace.TraceURL(span))
	return nil
}

func recordError(ctx context.Context, span trace.Span, err error) {
	if err != redis.Nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
}
