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
	"fmt"
	"net/http"

	"github.com/caiflower/common-tools/global/env"
	"github.com/caiflower/common-tools/pkg/logger"
	webclient "github.com/caiflower/common-tools/web/app/client"
	"github.com/caiflower/common-tools/web/protocol"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.10.0"
	"go.opentelemetry.io/otel/trace"
)

var webClientTracer = otel.Tracer("github.com/caiflower/common-tools/web/client")

// NewWebClientMiddleware returns tracing middleware for web/app/client.
// Register it with client.Use():
//
//	c.Use(otel.NewWebClientMiddleware())
func NewWebClientMiddleware() webclient.Middleware {
	return func(next webclient.Endpoint) webclient.Endpoint {
		return func(ctx context.Context, req *protocol.Request, resp *protocol.Response) (err error) {
			_traceID := getTraceID(ctx)
			traceID, traceErr := trace.TraceIDFromHex(_traceID)
			if traceErr != nil {
				logger.Error("trace.TraceIDFromHex failed. Error: %v", traceErr)
			}

			path := string(req.URI().Path())
			if path == "" {
				path = string(req.Host())
			}

			ctx, span := webClientTracer.Start(
				trace.ContextWithSpanContext(context.Background(), trace.NewSpanContext(trace.SpanContextConfig{
					TraceID: traceID,
				})),
				path,
				trace.WithSpanKind(trace.SpanKindClient),
			)

			defer func() {
				if recovered := recover(); recovered != nil {
					span.RecordError(fmt.Errorf("%v", recovered))
					span.SetStatus(codes.Error, fmt.Sprintf("%v", recovered))
					span.End()
					panic(recovered)
				}
				span.End()
			}()

			if span.IsRecording() {
				span.SetAttributes(
					semconv.HTTPMethodKey.String(string(req.Method())),
					semconv.HTTPHostKey.String(string(req.Host())),
					semconv.HTTPRouteKey.String(path),
					semconv.HTTPClientIPKey.String(env.GetLocalHostIP()),
				)
			}

			err = next(ctx, req, resp)
			if err != nil && span.IsRecording() {
				span.SetAttributes(semconv.HTTPStatusCodeKey.Int(http.StatusInternalServerError))
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
			} else if resp != nil && span.IsRecording() {
				span.SetAttributes(semconv.HTTPStatusCodeKey.Int(resp.StatusCode()))
			}

			logger.Trace("otel trace: %s", span.SpanContext().TraceID())
			return err
		}
	}
}
