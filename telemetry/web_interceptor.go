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
	"errors"
	"net/http"

	"github.com/caiflower/common-tools/web/common/e"
	"github.com/caiflower/common-tools/web/common/webctx"
	"go.opentelemetry.io/otel/trace"

	golocalv1 "github.com/caiflower/common-tools/pkg/golocal/v1"
	"github.com/caiflower/common-tools/pkg/tools"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.10.0"
)

const (
	uptraceSpan    = "uptraceDone"
	uptraceContent = "uptraceContent"
)

type WebInterceptor struct {
}

func NewWebInterceptor() *WebInterceptor {
	return &WebInterceptor{}
}

func (wi *WebInterceptor) Before(ctx *webctx.Context) e.ApiError {
	content := new(Content)
	attrs := make([]attribute.KeyValue, 6, 10)
	attrs[0] = attribute.String("http.request.action", ctx.GetAction())
	attrs[1] = semconv.HTTPMethodKey.String(ctx.GetMethod())
	_, r := ctx.GetResponseWriterAndRequest()
	attrs[2] = semconv.HTTPClientIPKey.String(r.RemoteAddr)
	attrs[3] = semconv.HTTPRouteKey.String(ctx.GetPath())
	content.Attrs = attrs
	traceId := golocalv1.GetTraceID()

	span := DefaultClient.Start(traceId, "github.com/caiflower/common-tools/web/v1", ctx.GetAction(), trace.SpanKindServer)
	ctx.Put(uptraceSpan, span)
	ctx.Put(uptraceContent, content)

	return nil
}

func (wi *WebInterceptor) After(ctx *webctx.Context, err e.ApiError) e.ApiError {
	span := ctx.Get(uptraceSpan).(trace.Span)
	content := ctx.Get(uptraceContent).(*Content)
	defer DefaultClient.End(span, content)

	if err != nil {
		content.Attrs[4] = semconv.HTTPStatusCodeKey.Int(err.GetCode())
		content.Attrs[5] = attribute.String("http.response.error.message", err.GetMessage())
		content.Failed = err.GetCause()
	} else {
		content.Attrs[4] = semconv.HTTPStatusCodeKey.Int(http.StatusOK)
		content.Attrs[5] = attribute.String("http.response.data", tools.ToJson(ctx.GetData()))
	}

	return nil
}

func (wi *WebInterceptor) OnPanic(ctx *webctx.Context, recover interface{}) e.ApiError {
	span := ctx.Get(uptraceSpan).(trace.Span)
	content := ctx.Get(uptraceContent).(*Content)
	defer DefaultClient.End(span, content)

	content.Attrs[4] = semconv.HTTPStatusCodeKey.Int(http.StatusInternalServerError)
	content.Failed = errors.New(tools.ToJson(recover))

	return nil
}
