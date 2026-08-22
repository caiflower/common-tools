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
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"

	"github.com/caiflower/common-tools/pkg/tools"
	"github.com/caiflower/common-tools/web/app"
	"github.com/caiflower/common-tools/web/common/e"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.10.0"
	"go.opentelemetry.io/otel/trace"
)

const defaultWebBodySize = 4096

type WebOption func(*webTraceOptions)

type webTraceOptions struct {
	requestBodyEnabled  bool
	responseBodyEnabled bool
	maxBodySize         int
	allowedHeaders      map[string]struct{}
	deniedHeaders       map[string]struct{}
}

// NewWebMiddleware returns tracing middleware that can be registered with
// RouterGroup.Use() or Engine.Use(). Request and response bodies are disabled by
// default because they frequently contain PII or credentials.
func NewWebMiddleware(options ...WebOption) app.HandlerFunc {
	traceOptions := defaultWebTraceOptions()
	for _, option := range options {
		option(&traceOptions)
	}

	return func(ctx context.Context, reqCtx *app.RequestContext) {
		if !IsEnabled() {
			reqCtx.Next(ctx)
			return
		}

		responseWriter, httpRequest := reqCtx.GetResponseWriterAndRequest()
		var requestBuffer *boundedBuffer
		if traceOptions.requestBodyEnabled {
			requestBuffer = newBoundedBuffer(traceOptions.maxBodySize)
			if !reqCtx.IsNetpoll() {
				if httpRequest != nil && httpRequest.Body != nil {
					httpRequest.Body = io.NopCloser(io.TeeReader(httpRequest.Body, requestBuffer))
				} else {
					requestBuffer = nil
				}
			} else {
				requestBuffer.Write(reqCtx.Request.Body())
			}
		}
		if !reqCtx.IsNetpoll() && responseWriter != nil {
			responseWriter = &tracingResponseWriter{
				ResponseWriter: responseWriter,
				body:           newBoundedBuffer(traceOptions.maxBodySize),
				captureBody:    traceOptions.responseBodyEnabled,
			}
			reqCtx.SetHttpWriterAndRequest(responseWriter, httpRequest)
		}

		route := reqCtx.GetRouteTemplate()
		if route == "" {
			route = reqCtx.GetPath()
		}
		action := reqCtx.GetAction()
		attrs := make([]attribute.KeyValue, 0, 16)
		attrs = append(attrs,
			attribute.String("http.request.action", action),
			semconv.HTTPMethodKey.String(reqCtx.GetMethod()),
			attribute.String("client.address", reqCtx.ClientIP()),
			semconv.HTTPRouteKey.String(route),
		)
		attrs = append(attrs, requestAttributes(reqCtx, httpRequest, traceOptions)...)

		spanName := action
		if spanName == "" {
			spanName = reqCtx.GetMethod() + " " + route
		}
		span := DefaultClient.Start(getTraceID(ctx), "github.com/caiflower/common-tools/web/v1", spanName, trace.SpanKindServer)

		var result webResult
		defer func() {
			result.requestBody = boundedBufferValue(requestBuffer)
			result.responseHeaders = responseHeaderAttributes(reqCtx, responseWriter, traceOptions)
			if recovered := recover(); recovered != nil {
				message := fmt.Sprint(recovered)
				result = webResult{
					statusCode: http.StatusInternalServerError,
					errorType:  "Panic",
					message:    message,
					failed:     errors.New(message),
				}
				DefaultClient.End(span, result.content(append(attrs, result.attributes()...)))
				// Re-panic so the framework's top-level recover writes the 500 response.
				panic(recovered)
			}

			DefaultClient.End(span, result.content(append(attrs, result.attributes()...)))
		}()

		reqCtx.Next(ctx)

		apiErr := reqCtx.GetError()
		result.statusCode = responseStatusCode(reqCtx, responseWriter, apiErr)
		result.failed = failureForAPIError(apiErr)
		if apiErr != nil {
			result.errorType = apiErr.GetType()
			result.message = apiErr.GetMessage()
		} else {
			result.responseBody = responseBody(reqCtx, responseWriter, traceOptions)
		}
	}
}

func defaultWebTraceOptions() webTraceOptions {
	return webTraceOptions{
		maxBodySize: defaultWebBodySize,
		deniedHeaders: map[string]struct{}{
			http.CanonicalHeaderKey("Authorization"):       {},
			http.CanonicalHeaderKey("Cookie"):              {},
			http.CanonicalHeaderKey("Proxy-Authorization"): {},
			http.CanonicalHeaderKey("Set-Cookie"):          {},
		},
		allowedHeaders: make(map[string]struct{}),
	}
}

// WithRequestBody controls capturing the first maxSize bytes of a request body.
// A non-positive maxSize uses the default limit.
func WithRequestBody(enabled bool, maxSize int) WebOption {
	return func(options *webTraceOptions) {
		options.requestBodyEnabled = enabled
		if maxSize > 0 {
			options.maxBodySize = maxSize
		}
	}
}

// WithResponseBody controls capturing response data. A non-positive maxSize
// uses the default limit.
func WithResponseBody(enabled bool, maxSize int) WebOption {
	return func(options *webTraceOptions) {
		options.responseBodyEnabled = enabled
		if maxSize > 0 {
			options.maxBodySize = maxSize
		}
	}
}

// WithAllowedHeaders restricts captured request and response headers. By
// default no headers are captured.
func WithAllowedHeaders(headers ...string) WebOption {
	return func(options *webTraceOptions) {
		for _, header := range headers {
			header = http.CanonicalHeaderKey(strings.TrimSpace(header))
			if header != "" {
				options.allowedHeaders[header] = struct{}{}
			}
		}
	}
}

// WithDeniedHeaders adds request and response headers to the built-in sensitive
// header list.
func WithDeniedHeaders(headers ...string) WebOption {
	return func(options *webTraceOptions) {
		for _, header := range headers {
			header = http.CanonicalHeaderKey(strings.TrimSpace(header))
			if header != "" {
				options.deniedHeaders[header] = struct{}{}
			}
		}
	}
}

type webResult struct {
	statusCode      int
	requestBody     string
	errorType       string
	message         string
	responseBody    string
	responseHeaders []attribute.KeyValue
	failed          error
}

func (result webResult) attributes() []attribute.KeyValue {
	attrs := make([]attribute.KeyValue, 0, 6)
	if result.requestBody != "" {
		attrs = append(attrs, attribute.String("http.request.body", result.requestBody))
	}
	attrs = append(attrs, semconv.HTTPStatusCodeKey.Int(result.statusCode))
	if result.errorType != "" {
		attrs = append(attrs, attribute.String("error.type", result.errorType))
	}
	if result.message != "" {
		attrs = append(attrs, attribute.String("error.message", result.message))
	}
	if result.responseBody != "" {
		attrs = append(attrs, attribute.String("http.response.body", result.responseBody))
	}
	return append(attrs, result.responseHeaders...)
}

func (result webResult) content(attrs []attribute.KeyValue) *Content {
	content := &Content{Attrs: attrs}
	if result.failed != nil {
		content.Failed = result.failed
	} else if result.statusCode >= 400 && result.message != "" {
		content.Failed = errors.New(result.message)
	}
	return content
}

func failureForAPIError(apiErr e.ApiError) error {
	if apiErr == nil {
		return nil
	}
	if cause := apiErr.GetCause(); cause != nil {
		return cause
	}
	return errors.New(apiErr.Error())
}

func responseStatusCode(reqCtx *app.RequestContext, responseWriter http.ResponseWriter, apiErr e.ApiError) int {
	if writer, ok := responseWriter.(*tracingResponseWriter); ok && writer.status != 0 {
		return writer.status
	}
	if reqCtx.IsNetpoll() {
		if status := reqCtx.GetStatusCode(); status != 0 {
			return status
		}
	}
	if apiErr != nil && apiErr.GetCode() != 0 {
		return apiErr.GetCode()
	}
	return http.StatusOK
}

func responseBody(reqCtx *app.RequestContext, responseWriter http.ResponseWriter, options webTraceOptions) string {
	if !options.responseBodyEnabled {
		return ""
	}
	if !reqCtx.IsNetpoll() {
		if writer, ok := responseWriter.(*tracingResponseWriter); ok && writer.body.Len() > 0 {
			return writer.body.String()
		}
	} else if body := reqCtx.Response.Body(); len(body) > 0 {
		return truncateString(string(body), options.maxBodySize)
	}

	if reqCtx.GetData() == nil {
		return ""
	}
	return truncateString(tools.ToJson(reqCtx.GetData()), options.maxBodySize)
}

func requestAttributes(reqCtx *app.RequestContext, httpRequest *http.Request, options webTraceOptions) []attribute.KeyValue {
	var attrs []attribute.KeyValue

	if !reqCtx.IsNetpoll() && httpRequest != nil {
		scheme := "http"
		if httpRequest.TLS != nil {
			scheme = "https"
		}
		attrs = append(attrs,
			attribute.String("url.path", httpRequest.URL.Path),
			attribute.String("url.query", httpRequest.URL.RawQuery),
			attribute.String("server.address", hostOnly(httpRequest.Host)),
			attribute.String("url.scheme", scheme),
			attribute.Int64("http.request.content_length", httpRequest.ContentLength),
		)
	} else {
		uri := reqCtx.URI()
		attrs = append(attrs,
			attribute.String("url.path", reqCtx.GetPath()),
			attribute.String("url.query", string(uri.QueryString())),
			attribute.String("server.address", hostOnly(string(uri.Host()))),
			attribute.String("url.scheme", string(uri.Scheme())),
			attribute.Int64("http.request.content_length", reqCtx.GetContentLength()),
		)
	}

	return append(attrs, requestHeaderAttributes(reqCtx, options)...)
}

func requestHeaderAttributes(reqCtx *app.RequestContext, options webTraceOptions) []attribute.KeyValue {
	attrs := make([]attribute.KeyValue, 0, len(options.allowedHeaders))
	for header := range options.allowedHeaders {
		if _, denied := options.deniedHeaders[header]; denied {
			continue
		}
		if value := reqCtx.HeaderGet(header); value != "" {
			attrs = append(attrs, attribute.String("http.request.header."+strings.ToLower(header), value))
		}
	}
	return attrs
}

func responseHeaderAttributes(reqCtx *app.RequestContext, responseWriter http.ResponseWriter, options webTraceOptions) []attribute.KeyValue {
	attrs := make([]attribute.KeyValue, 0, len(options.allowedHeaders))
	for header := range options.allowedHeaders {
		if _, denied := options.deniedHeaders[header]; denied {
			continue
		}

		var value string
		if !reqCtx.IsNetpoll() && responseWriter != nil {
			value = responseWriter.Header().Get(header)
		} else {
			value = reqCtx.Response.Header.Get(header)
		}
		if value != "" {
			attrs = append(attrs, attribute.String("http.response.header."+strings.ToLower(header), value))
		}
	}
	return attrs
}

func hostOnly(host string) string {
	if hostname, _, err := net.SplitHostPort(host); err == nil {
		return hostname
	}
	return host
}

func truncateString(value string, maxSize int) string {
	if maxSize <= 0 || len(value) <= maxSize {
		return value
	}
	const suffix = "...[truncated]"
	if maxSize <= len(suffix) {
		return value[:maxSize]
	}
	return value[:maxSize-len(suffix)] + suffix
}

func boundedBufferValue(buffer *boundedBuffer) string {
	if buffer == nil || buffer.Len() == 0 {
		return ""
	}
	return buffer.String()
}

type boundedBuffer struct {
	buffer    bytes.Buffer
	limit     int
	truncated bool
}

func newBoundedBuffer(limit int) *boundedBuffer {
	if limit <= 0 {
		limit = defaultWebBodySize
	}
	return &boundedBuffer{limit: limit}
}

func (buffer *boundedBuffer) Write(data []byte) (int, error) {
	remaining := buffer.limit - buffer.buffer.Len()
	switch {
	case remaining > 0:
		if len(data) > remaining {
			buffer.buffer.Write(data[:remaining])
			buffer.truncated = true
		} else {
			buffer.buffer.Write(data)
		}
	case len(data) > 0:
		buffer.truncated = true
	}
	return len(data), nil
}

func (buffer *boundedBuffer) String() string {
	value := buffer.buffer.String()
	if buffer.truncated {
		value += "...[truncated]"
	}
	return value
}

func (buffer *boundedBuffer) Len() int {
	return buffer.buffer.Len()
}

// tracingResponseWriter records the status chosen inside the handler chain while
// forwarding optional writer behavior as safely as possible.
type tracingResponseWriter struct {
	http.ResponseWriter
	status      int
	body        *boundedBuffer
	captureBody bool
}

func (writer *tracingResponseWriter) WriteHeader(statusCode int) {
	if writer.status == 0 {
		writer.status = statusCode
	}
	writer.ResponseWriter.WriteHeader(statusCode)
}

func (writer *tracingResponseWriter) Write(data []byte) (int, error) {
	if writer.status == 0 {
		writer.status = http.StatusOK
	}
	if writer.captureBody && writer.body != nil {
		writer.body.Write(data)
	}
	return writer.ResponseWriter.Write(data)
}

func (writer *tracingResponseWriter) Flush() {
	if flusher, ok := writer.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (writer *tracingResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hijacker, ok := writer.ResponseWriter.(http.Hijacker); ok {
		return hijacker.Hijack()
	}
	return nil, nil, errors.New("the response writer does not support hijacking")
}

func (writer *tracingResponseWriter) CloseNotify() <-chan bool {
	if notifier, ok := writer.ResponseWriter.(http.CloseNotifier); ok {
		return notifier.CloseNotify()
	}
	closed := make(chan bool, 1)
	close(closed)
	return closed
}
