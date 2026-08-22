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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/caiflower/common-tools/web"
	"github.com/caiflower/common-tools/web/app"
	"github.com/caiflower/common-tools/web/app/server/config"
	"github.com/caiflower/common-tools/web/common/e"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestWebMiddleware(t *testing.T) {
	oldClient := DefaultClient
	DefaultClient = &client{}
	defer func() { DefaultClient = oldClient }()

	engine := web.Default(
		config.WithAddr(":0"),
		config.WithName("test-otel-web-middleware"),
		config.WithRootPath(""),
		config.WithControllerRootPkgName("webtest"),
	)
	engine.Use(NewWebMiddleware())
	engine.GET("/test", func(ctx context.Context, reqCtx *app.RequestContext) {
		reqCtx.JSON(200, map[string]string{"message": "ok"})
	})

	handler := engine.Handler()
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
}

func TestWebMiddlewarePanic(t *testing.T) {
	oldClient := DefaultClient
	DefaultClient = &client{}
	defer func() { DefaultClient = oldClient }()

	engine := web.Default(
		config.WithAddr(":0"),
		config.WithName("test-otel-web-middleware-panic"),
		config.WithRootPath(""),
		config.WithControllerRootPkgName("webtest"),
	)
	engine.Use(NewWebMiddleware())
	engine.GET("/panic", func(ctx context.Context, reqCtx *app.RequestContext) {
		panic("boom")
	})

	handler := engine.Handler()
	req := httptest.NewRequest("GET", "/panic", nil)
	w := httptest.NewRecorder()
	assert.NotPanics(t, func() {
		handler.ServeHTTP(w, req)
	})
	assert.Equal(t, 500, w.Code)
}

func setupRecordingWebClient(t *testing.T) *tracetest.SpanRecorder {
	t.Helper()

	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	otel.SetTracerProvider(provider)

	oldClient := DefaultClient
	t.Cleanup(func() { DefaultClient = oldClient })
	DefaultClient = &client{
		config:         Config{Enabled: true},
		tracerProvider: provider,
	}
	return recorder
}

func spanAttributes(t *testing.T, recorder *tracetest.SpanRecorder) map[string]string {
	t.Helper()

	spans := recorder.Ended()
	assert.Len(t, spans, 1)
	if len(spans) != 1 {
		return nil
	}

	attrs := make(map[string]string)
	for _, attr := range spans[0].Attributes() {
		attrs[string(attr.Key)] = attr.Value.Emit()
	}
	return attrs
}

func TestWebMiddlewareMetadataAndRouteTemplate(t *testing.T) {
	recorder := setupRecordingWebClient(t)

	engine := web.Default(
		config.WithAddr(":0"),
		config.WithName("test-otel-web-metadata"),
		config.WithRootPath(""),
		config.WithControllerRootPkgName("webtest"),
	)
	engine.Use(NewWebMiddleware())
	engine.GET("/users/:id", func(ctx context.Context, reqCtx *app.RequestContext) {
		reqCtx.JSON(http.StatusAccepted, map[string]string{"message": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/users/42?q=active", nil)
	w := httptest.NewRecorder()
	engine.Handler().ServeHTTP(w, req)

	attrs := spanAttributes(t, recorder)
	assert.Equal(t, "/users/:id", attrs["http.route"])
	assert.Equal(t, "/users/42", attrs["url.path"])
	assert.Equal(t, "q=active", attrs["url.query"])
	assert.Equal(t, "GET", attrs["http.method"])
	assert.Equal(t, "202", attrs["http.status_code"])
	assert.NotContains(t, attrs, "http.response.body")
}

func TestWebMiddlewareBusinessErrorIsFailedSpan(t *testing.T) {
	recorder := setupRecordingWebClient(t)

	engine := web.Default(
		config.WithAddr(":0"),
		config.WithName("test-otel-web-error"),
		config.WithRootPath(""),
		config.WithControllerRootPkgName("webtest"),
	)
	engine.Use(NewWebMiddleware())
	engine.GET("/conflict", func(ctx context.Context, reqCtx *app.RequestContext) {
		reqCtx.SetError(e.NewApiError(e.Conflict, "state conflict", nil))
	})

	req := httptest.NewRequest(http.MethodGet, "/conflict", nil)
	w := httptest.NewRecorder()
	engine.Handler().ServeHTTP(w, req)

	spans := recorder.Ended()
	assert.Len(t, spans, 1)
	if len(spans) != 1 {
		return
	}

	attrs := spanAttributes(t, recorder)
	assert.Equal(t, "400", attrs["http.status_code"])
	assert.Equal(t, "Conflict", attrs["error.type"])
	assert.Equal(t, "state conflict", attrs["error.message"])
	assert.Equal(t, codes.Error, spans[0].Status().Code)
}

func TestWebMiddlewareOptionalCaptureAndHeaderFiltering(t *testing.T) {
	recorder := setupRecordingWebClient(t)

	engine := web.Default(
		config.WithAddr(":0"),
		config.WithName("test-otel-web-capture"),
		config.WithRootPath(""),
		config.WithControllerRootPkgName("webtest"),
	)
	engine.Use(NewWebMiddleware(
		WithRequestBody(true, 32),
		WithResponseBody(true, 64),
		WithAllowedHeaders("Authorization", "X-Api-Key", "X-Secret", "X-Response-Id", "Set-Cookie"),
		WithDeniedHeaders("X-Secret"),
	))
	engine.POST("/echo", func(ctx context.Context, reqCtx *app.RequestContext) {
		reqCtx.SetHeader("X-Response-Id", "response-42")
		reqCtx.SetHeader("Set-Cookie", "session=hidden")
		reqCtx.JSON(http.StatusOK, map[string]string{"request": string(reqCtx.GetBody())})
	})

	requestBody := strings.Repeat("x", 80)
	req := httptest.NewRequest(http.MethodPost, "/echo", strings.NewReader(requestBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "secret-token")
	req.Header.Set("X-Api-Key", "public-key")
	req.Header.Set("X-Secret", "hidden-value")
	w := httptest.NewRecorder()
	engine.Handler().ServeHTTP(w, req)

	attrs := spanAttributes(t, recorder)
	assert.Contains(t, attrs["http.request.body"], "...[truncated]")
	assert.Contains(t, attrs["http.response.body"], "...[truncated]")
	assert.Contains(t, w.Body.String(), requestBody)
	assert.Equal(t, "public-key", attrs["http.request.header.x-api-key"])
	assert.Equal(t, "response-42", attrs["http.response.header.x-response-id"])
	assert.NotContains(t, attrs, "http.request.header.authorization")
	assert.NotContains(t, attrs, "http.request.header.x-secret")
	assert.NotContains(t, attrs, "http.response.header.set-cookie")
}
