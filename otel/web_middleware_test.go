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
	"net/http/httptest"
	"testing"

	"github.com/caiflower/common-tools/web"
	"github.com/caiflower/common-tools/web/app"
	"github.com/caiflower/common-tools/web/app/server/config"
	"github.com/stretchr/testify/assert"
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
