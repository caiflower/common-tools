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
	"github.com/caiflower/common-tools/global/env"
	golocalv1 "github.com/caiflower/common-tools/pkg/golocal/v1"
	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/uptrace/uptrace-go/uptrace"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.10.0"
	"go.opentelemetry.io/otel/trace"
	"net/http"
)

var httpclientTracer = otel.Tracer("github.com/caiflower/common-tools/http")

type HttpClientHook struct {
}

func NewHttpClientHook() *HttpClientHook {
	return &HttpClientHook{}
}

func (h *HttpClientHook) BeforeRequest(ctx context.Context, request *http.Request) (context.Context, error) {
	traceId := golocalv1.GetTraceID()
	if traceId == "" {
		traceId = ctx.Value("traceId").(string)
	}
	traceID, err := trace.TraceIDFromHex(traceId)
	if err != nil {
		logger.Error("trace.TraceIDFromHex failed. Error: %v", err)
	}

	path := request.URL.Path
	if path == "" {
		path = request.URL.Host
	}

	ctx, span := httpclientTracer.Start(trace.ContextWithSpanContext(context.Background(), trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: traceID,
	})), path, trace.WithSpanKind(trace.SpanKindClient))

	if !span.IsRecording() {
		return ctx, nil
	}

	attrs := make([]attribute.KeyValue, 0, 3)
	attrs = append(attrs, semconv.HTTPMethodKey.String(request.Method))
	attrs = append(attrs, semconv.HTTPClientIPKey.String(env.GetLocalHostIP()))
	attrs = append(attrs, semconv.HTTPRouteKey.String(path))
	span.SetAttributes(attrs...)

	return ctx, nil
}

func (h *HttpClientHook) AfterRequest(ctx context.Context, _ *http.Request, response *http.Response, err error) error {
	span := trace.SpanFromContext(ctx)
	defer span.End()

	if !span.IsRecording() {
		return nil
	}

	attrs := make([]attribute.KeyValue, 0, 2)
	if err != nil {
		attrs = append(attrs, semconv.HTTPStatusCodeKey.Int(http.StatusInternalServerError))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	} else {
		attrs = append(attrs, semconv.HTTPStatusCodeKey.Int(response.StatusCode))
		attrs = append(attrs, semconv.HTTPHostKey.String(response.Request.Host))
	}
	span.SetAttributes(attrs...)

	logger.Trace("uptrace: %s\n", uptrace.TraceURL(span))
	return nil
}
