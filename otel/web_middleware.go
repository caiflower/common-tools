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
	"errors"
	"net"
	"net/http"

	"github.com/caiflower/common-tools/pkg/tools"
	"github.com/caiflower/common-tools/web/app"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.10.0"
	"go.opentelemetry.io/otel/trace"
)

// NewWebMiddleware returns tracing middleware that can be registered with
// RouterGroup.Use() or Engine.Use(). Code before reqCtx.Next() is the "before"
// logic, code after it is the "after" logic, and recover() handles panics.
func NewWebMiddleware() app.HandlerFunc {
	return func(ctx context.Context, reqCtx *app.RequestContext) {
		content := new(Content)
		attrs := make([]attribute.KeyValue, 6, 10)
		attrs[0] = attribute.String("http.request.action", reqCtx.GetAction())
		attrs[1] = semconv.HTTPMethodKey.String(reqCtx.GetMethod())
		attrs[2] = semconv.HTTPClientIPKey.String(clientIP(reqCtx))
		attrs[3] = semconv.HTTPRouteKey.String(reqCtx.GetPath())
		content.Attrs = attrs

		span := DefaultClient.Start(getTraceID(ctx), "github.com/caiflower/common-tools/web/v1", reqCtx.GetAction(), trace.SpanKindServer)

		defer func() {
			if recovered := recover(); recovered != nil {
				attrs[4] = semconv.HTTPStatusCodeKey.Int(http.StatusInternalServerError)
				attrs[5] = attribute.String("http.response.error.message", tools.ToJson(recovered))
				content.Failed = errors.New(tools.ToJson(recovered))
				DefaultClient.End(span, content)
				// Re-panic so the framework's top-level recover writes the 500 response.
				panic(recovered)
			}
			DefaultClient.End(span, content)
		}()

		reqCtx.Next(ctx)

		if err := reqCtx.GetError(); err != nil {
			attrs[4] = semconv.HTTPStatusCodeKey.Int(err.GetCode())
			attrs[5] = attribute.String("http.response.error.message", err.GetMessage())
			content.Failed = err.GetCause()
		} else {
			attrs[4] = semconv.HTTPStatusCodeKey.Int(http.StatusOK)
			attrs[5] = attribute.String("http.response.data", tools.ToJson(reqCtx.GetData()))
		}
	}
}

func clientIP(reqCtx *app.RequestContext) string {
	_, r := reqCtx.GetResponseWriterAndRequest()
	if r == nil {
		return reqCtx.ClientIP()
	}
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}
