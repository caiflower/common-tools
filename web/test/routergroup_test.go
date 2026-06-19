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

package webtest

import (
	"bytes"
	"context"
	"net/http/httptest"
	"testing"

	"github.com/caiflower/common-tools/web"
	"github.com/caiflower/common-tools/web/app"
	"github.com/caiflower/common-tools/web/app/server/config"
	"github.com/caiflower/common-tools/web/common/json"
	"github.com/caiflower/common-tools/web/common/resp"
	"github.com/caiflower/common-tools/web/router"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
)

// TestRouterGroupBasic tests basic RouterGroup functionality
func TestRouterGroupBasic(t *testing.T) {
	engine := web.Default(
		config.WithAddr(":0"),
		config.WithName("test-router-group"),
		config.WithRootPath(""),
		config.WithControllerRootPkgName("webtest"),
	)

	// Register a simple HandlerFunc route (like Hertz)
	engine.GET("/ping", func(ctx context.Context, reqCtx *app.RequestContext) {
		reqCtx.JSON(200, map[string]string{"message": "pong"})
	})

	handler := engine.Handler()

	req := httptest.NewRequest("GET", "/ping", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)

	var resp map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "pong", resp["message"])
}

// TestRouterGroupWithPrefix tests RouterGroup with path prefix
func TestRouterGroupWithPrefix(t *testing.T) {
	engine := web.Default(
		config.WithAddr(":0"),
		config.WithName("test-router-group-prefix"),
		config.WithRootPath(""),
		config.WithControllerRootPkgName("webtest"),
	)

	// Create a group with prefix
	api := engine.Group("/api")
	api.GET("/users", func(ctx context.Context, reqCtx *app.RequestContext) {
		reqCtx.JSON(200, map[string]string{"handler": "list-users"})
	})

	handler := engine.Handler()

	req := httptest.NewRequest("GET", "/api/users", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)

	var resp map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "list-users", resp["handler"])
}

// TestRouterGroupNested tests nested RouterGroups
func TestRouterGroupNested(t *testing.T) {
	engine := web.Default(
		config.WithAddr(":0"),
		config.WithName("test-router-group-nested"),
		config.WithRootPath(""),
		config.WithControllerRootPkgName("webtest"),
	)

	v1 := engine.Group("/api/v1")
	v1.GET("/status", func(ctx context.Context, reqCtx *app.RequestContext) {
		reqCtx.JSON(200, map[string]string{"version": "v1"})
	})

	v2 := engine.Group("/api/v2")
	v2.GET("/status", func(ctx context.Context, reqCtx *app.RequestContext) {
		reqCtx.JSON(200, map[string]string{"version": "v2"})
	})

	handler := engine.Handler()

	// Test v1
	req := httptest.NewRequest("GET", "/api/v1/status", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
	var resp1 map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp1)
	assert.Equal(t, "v1", resp1["version"])

	// Test v2
	req2 := httptest.NewRequest("GET", "/api/v2/status", nil)
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)
	assert.Equal(t, 200, w2.Code)
	var resp2 map[string]string
	json.Unmarshal(w2.Body.Bytes(), &resp2)
	assert.Equal(t, "v2", resp2["version"])
}

// TestRouterGroupMiddleware tests middleware execution in RouterGroup
func TestRouterGroupMiddleware(t *testing.T) {
	engine := web.Default(
		config.WithAddr(":0"),
		config.WithName("test-router-group-middleware"),
		config.WithRootPath(""),
		config.WithControllerRootPkgName("webtest"),
	)

	var middlewareCalled bool

	api := engine.Group("/api")
	api.Use(func(ctx context.Context, reqCtx *app.RequestContext) {
		middlewareCalled = true
		reqCtx.SetHeader("X-Middleware", "true")
		reqCtx.Next(ctx)
	})
	api.GET("/test", func(ctx context.Context, reqCtx *app.RequestContext) {
		reqCtx.JSON(200, map[string]bool{"ok": true})
	})

	handler := engine.Handler()

	req := httptest.NewRequest("GET", "/api/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	assert.True(t, middlewareCalled, "middleware should have been called")
}

// TestRouterGroupMultipleMethods tests registering multiple HTTP methods
func TestRouterGroupMultipleMethods(t *testing.T) {
	engine := web.Default(
		config.WithAddr(":0"),
		config.WithName("test-router-group-methods"),
		config.WithRootPath(""),
		config.WithControllerRootPkgName("webtest"),
	)

	engine.GET("/resource", func(ctx context.Context, reqCtx *app.RequestContext) {
		reqCtx.JSON(200, map[string]string{"method": "GET"})
	})
	engine.POST("/resource", func(ctx context.Context, reqCtx *app.RequestContext) {
		reqCtx.JSON(200, map[string]string{"method": "POST"})
	})
	engine.PUT("/resource", func(ctx context.Context, reqCtx *app.RequestContext) {
		reqCtx.JSON(200, map[string]string{"method": "PUT"})
	})
	engine.DELETE("/resource", func(ctx context.Context, reqCtx *app.RequestContext) {
		reqCtx.JSON(200, map[string]string{"method": "DELETE"})
	})

	handler := engine.Handler()

	methods := []string{"GET", "POST", "PUT", "DELETE"}
	for _, m := range methods {
		req := httptest.NewRequest(m, "/resource", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		assert.Equal(t, 200, w.Code)
		var resp map[string]string
		json.Unmarshal(w.Body.Bytes(), &resp)
		assert.Equal(t, m, resp["method"])
	}
}

// TestRouterGroupGRPC tests gRPC route registration via RouterGroup
func TestRouterGroupGRPC(t *testing.T) {
	engine := web.Default(
		config.WithAddr(":0"),
		config.WithName("test-router-group-grpc"),
		config.WithRootPath(""),
		config.WithControllerRootPkgName("webtest"),
	)

	// Register gRPC route using the GRPC method
	engine.GRPC("POST", "/grpc/search", _IService_Search_Handler, &HelloImpl{})

	handler := engine.Handler()

	reqBody := `{"query":"test"}`
	req := httptest.NewRequest("POST", "/grpc/search", bytes.NewReader([]byte(reqBody)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// Should not be 404 (route should be found)
	assert.NotEqual(t, 404, w.Code, "gRPC route should be registered and found")
}

// TestRouterGroupAutoDetectHandler tests auto-detection of handler types
func TestRouterGroupAutoDetectHandler(t *testing.T) {
	engine := web.Default(
		config.WithAddr(":0"),
		config.WithName("test-router-group-autodetect"),
		config.WithRootPath(""),
		config.WithControllerRootPkgName("webtest"),
	)

	// Register with app.HandlerFunc (should be auto-detected as HandlerFuncTypeOfMethod)
	engine.GET("/handler-func", func(ctx context.Context, reqCtx *app.RequestContext) {
		reqCtx.JSON(200, map[string]string{"type": "handler-func"})
	})

	// Register with a regular function (should be auto-detected as DefaultTypeOfMethod)
	engine.POST("/regular-func", func(req *UserRequest) *UserResponse {
		return &UserResponse{
			Success: true,
			Message: "regular func",
		}
	})

	handler := engine.Handler()

	// Test HandlerFunc route
	req1 := httptest.NewRequest("GET", "/handler-func", nil)
	w1 := httptest.NewRecorder()
	handler.ServeHTTP(w1, req1)
	assert.Equal(t, 200, w1.Code)

	var resp1 map[string]string
	json.Unmarshal(w1.Body.Bytes(), &resp1)
	assert.Equal(t, "handler-func", resp1["type"])

	// Test regular function route
	body, _ := json.Marshal(UserRequest{
		ID:     1,
		Name:   "Test",
		Age:    25,
		Status: "active",
	})
	req2 := httptest.NewRequest("POST", "/regular-func", bytes.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)
	assert.Equal(t, 200, w2.Code)
}

// TestRouterGroupAny tests the Any method which registers all HTTP methods
func TestRouterGroupAny(t *testing.T) {
	engine := web.Default(
		config.WithAddr(":0"),
		config.WithName("test-router-group-any"),
		config.WithRootPath(""),
		config.WithControllerRootPkgName("webtest"),
	)

	engine.Any("/any", func(ctx context.Context, reqCtx *app.RequestContext) {
		reqCtx.JSON(200, map[string]string{"method": string(reqCtx.Method())})
	})

	handler := engine.Handler()

	for _, m := range []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS", "HEAD"} {
		req := httptest.NewRequest(m, "/any", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		assert.Equal(t, 200, w.Code, "method %s should return 200", m)
	}
}

// TestRouterGroupHandle tests custom HTTP method registration
func TestRouterGroupHandle(t *testing.T) {
	engine := web.Default(
		config.WithAddr(":0"),
		config.WithName("test-router-group-handle"),
		config.WithRootPath(""),
		config.WithControllerRootPkgName("webtest"),
	)

	engine.Handle("GET", "/custom", func(ctx context.Context, reqCtx *app.RequestContext) {
		reqCtx.JSON(200, map[string]string{"custom": "true"})
	})

	handler := engine.Handler()

	req := httptest.NewRequest("GET", "/custom", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
}

// TestRouterGroupGRPCWithDifferentMethods tests PUTGRPC and PATCHGRPC
func TestRouterGroupGRPCWithDifferentMethods(t *testing.T) {
	engine := web.Default(
		config.WithAddr(":0"),
		config.WithName("test-router-group-grpc-methods"),
		config.WithRootPath(""),
		config.WithControllerRootPkgName("webtest"),
	)

	// Register gRPC with different HTTP methods
	engine.GRPC("PUT", "/grpc/update", _IService_Search_Handler, &HelloImpl{})
	engine.GRPC("PATCH", "/grpc/patch", _IService_Search_Handler, &HelloImpl{})

	handler := engine.Handler()

	// Test PUT gRPC route
	req1 := httptest.NewRequest("PUT", "/grpc/update", bytes.NewReader([]byte(`{"query":"test"}`)))
	req1.Header.Set("Content-Type", "application/json")
	w1 := httptest.NewRecorder()
	handler.ServeHTTP(w1, req1)
	assert.NotEqual(t, 404, w1.Code, "PUT gRPC route should be found")

	// Test PATCH gRPC route
	req2 := httptest.NewRequest("PATCH", "/grpc/patch", bytes.NewReader([]byte(`{"query":"test"}`)))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)
	assert.NotEqual(t, 404, w2.Code, "PATCH gRPC route should be found")
}

// TestRouterGroupMiddlewareIsolation tests that middleware in one group does not affect another group
func TestRouterGroupMiddlewareIsolation(t *testing.T) {
	engine := web.Default(
		config.WithAddr(":0"),
		config.WithName("test-router-group-isolation"),
		config.WithRootPath(""),
		config.WithControllerRootPkgName("webtest"),
	)

	var groupAMiddlewareCalled bool
	var groupBMiddlewareCalled bool

	groupA := engine.Group("/a")
	groupA.Use(func(ctx context.Context, reqCtx *app.RequestContext) {
		groupAMiddlewareCalled = true
		reqCtx.Next(ctx)
	})
	groupA.GET("/test", func(ctx context.Context, reqCtx *app.RequestContext) {
		reqCtx.JSON(200, map[string]string{"group": "a"})
	})

	groupB := engine.Group("/b")
	groupB.GET("/test", func(ctx context.Context, reqCtx *app.RequestContext) {
		reqCtx.JSON(200, map[string]string{"group": "b"})
	})

	handler := engine.Handler()

	// Request /b/test should NOT trigger groupA's middleware
	req := httptest.NewRequest("GET", "/b/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	assert.False(t, groupAMiddlewareCalled, "groupA middleware should NOT be called for /b/test")
	assert.False(t, groupBMiddlewareCalled, "groupB has no middleware, should remain false")
}

// TestRouterGroupMiddlewareChainedUse tests multiple Use calls execute middleware in order
func TestRouterGroupMiddlewareChainedUse(t *testing.T) {
	engine := web.Default(
		config.WithAddr(":0"),
		config.WithName("test-router-group-chained-use"),
		config.WithRootPath(""),
		config.WithControllerRootPkgName("webtest"),
	)

	var order []string

	api := engine.Group("/api")
	api.Use(func(ctx context.Context, reqCtx *app.RequestContext) {
		order = append(order, "mw1")
		reqCtx.Next(ctx)
	})
	api.Use(func(ctx context.Context, reqCtx *app.RequestContext) {
		order = append(order, "mw2")
		reqCtx.Next(ctx)
	})
	api.Use(func(ctx context.Context, reqCtx *app.RequestContext) {
		order = append(order, "mw3")
		reqCtx.Next(ctx)
	})
	api.GET("/test", func(ctx context.Context, reqCtx *app.RequestContext) {
		order = append(order, "handler")
		reqCtx.JSON(200, map[string]bool{"ok": true})
	})

	handler := engine.Handler()

	req := httptest.NewRequest("GET", "/api/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	assert.Equal(t, []string{"mw1", "mw2", "mw3", "handler"}, order, "middleware and handler should execute in registration order")
}

// TestRouterGroupMiddlewareNestedInheritance tests that nested groups inherit parent middleware
func TestRouterGroupMiddlewareNestedInheritance(t *testing.T) {
	engine := web.Default(
		config.WithAddr(":0"),
		config.WithName("test-router-group-nested-mw"),
		config.WithRootPath(""),
		config.WithControllerRootPkgName("webtest"),
	)

	var order []string

	api := engine.Group("/api")
	api.Use(func(ctx context.Context, reqCtx *app.RequestContext) {
		order = append(order, "api-mw")
		reqCtx.Next(ctx)
	})

	v1 := api.Group("/v1")
	v1.Use(func(ctx context.Context, reqCtx *app.RequestContext) {
		order = append(order, "v1-mw")
		reqCtx.Next(ctx)
	})
	v1.GET("/test", func(ctx context.Context, reqCtx *app.RequestContext) {
		order = append(order, "handler")
		reqCtx.JSON(200, map[string]bool{"ok": true})
	})

	handler := engine.Handler()

	req := httptest.NewRequest("GET", "/api/v1/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	assert.Equal(t, []string{"api-mw", "v1-mw", "handler"}, order, "parent and child middleware should both execute in order")
}

// TestRouterGroupDefaultTypeWithValidation tests DefaultTypeOfMethod with parameter validation
func TestRouterGroupDefaultTypeWithValidation(t *testing.T) {
	engine := web.Default(
		config.WithAddr(":0"),
		config.WithName("test-router-group-validation"),
		config.WithRootPath(""),
		config.WithControllerRootPkgName("webtest"),
	)

	// Register a regular function route (DefaultTypeOfMethod) with validation tags
	engine.POST("/users", func(req *UserRequest) *UserResponse {
		return &UserResponse{
			Success: true,
			Message: "ok",
			Data: map[string]interface{}{
				"id":     req.ID,
				"name":   req.Name,
				"age":    req.Age,
				"status": req.Status,
			},
		}
	})

	handler := engine.Handler()

	t.Run("valid request", func(t *testing.T) {
		body, _ := json.Marshal(UserRequest{
			ID:     1,
			Name:   "John",
			Age:    25,
			Status: "active",
		})
		req := httptest.NewRequest("POST", "/users", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		assert.Equal(t, 200, w.Code)
		var result resp.Result
		json.Unmarshal(w.Body.Bytes(), &result)
		// DefaultTypeOfMethod return value is wrapped in result.Data
		respMap, ok := result.Data.(map[string]interface{})
		assert.True(t, ok)
		assert.Equal(t, true, respMap["success"])
		innerData, ok := respMap["data"].(map[string]interface{})
		assert.True(t, ok)
		assert.Equal(t, float64(1), innerData["id"])
		assert.Equal(t, "John", innerData["name"])
	})

	t.Run("missing required name", func(t *testing.T) {
		body, _ := json.Marshal(UserRequest{
			ID:     1,
			Name:   "", // required, empty should fail
			Age:    25,
			Status: "active",
		})
		req := httptest.NewRequest("POST", "/users", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		assert.NotEqual(t, 200, w.Code)
		var result resp.Result
		json.Unmarshal(w.Body.Bytes(), &result)
		assert.NotNil(t, result.Error)
		assert.Contains(t, result.Error.Message, "Name")
	})

	t.Run("age out of range", func(t *testing.T) {
		body, _ := json.Marshal(UserRequest{
			ID:     1,
			Name:   "John",
			Age:    150, // max 120
			Status: "active",
		})
		req := httptest.NewRequest("POST", "/users", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		assert.NotEqual(t, 200, w.Code)
		var result resp.Result
		json.Unmarshal(w.Body.Bytes(), &result)
		assert.NotNil(t, result.Error)
		assert.Contains(t, result.Error.Message, "Age")
	})

	t.Run("invalid status value", func(t *testing.T) {
		body, _ := json.Marshal(UserRequest{
			ID:     1,
			Name:   "John",
			Age:    25,
			Status: "unknown", // not in [active, inactive, pending]
		})
		req := httptest.NewRequest("POST", "/users", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		assert.NotEqual(t, 200, w.Code)
		var result resp.Result
		json.Unmarshal(w.Body.Bytes(), &result)
		assert.NotNil(t, result.Error)
		assert.Contains(t, result.Error.Message, "Status")
	})

	t.Run("id out of range", func(t *testing.T) {
		body, _ := json.Marshal(UserRequest{
			ID:     0, // must be between 1-1000
			Name:   "John",
			Age:    25,
			Status: "active",
		})
		req := httptest.NewRequest("POST", "/users", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		assert.NotEqual(t, 200, w.Code)
		var result resp.Result
		json.Unmarshal(w.Body.Bytes(), &result)
		assert.NotNil(t, result.Error)
		assert.Contains(t, result.Error.Message, "ID")
	})
}

// TestRouterGroupDefaultTypeWithGroupAndValidation tests DefaultTypeOfMethod in a group with validation
func TestRouterGroupDefaultTypeWithGroupAndValidation(t *testing.T) {
	engine := web.Default(
		config.WithAddr(":0"),
		config.WithName("test-router-group-group-validation"),
		config.WithRootPath(""),
		config.WithControllerRootPkgName("webtest"),
	)

	api := engine.Group("/api")
	api.POST("/products", func(req *ProductRequest) *UserResponse {
		return &UserResponse{
			Success: true,
			Message: "product created",
			Data: map[string]interface{}{
				"product_id":   req.ProductID,
				"product_name": req.ProductName,
				"price":        req.Price,
				"category":     req.Category,
			},
		}
	})

	handler := engine.Handler()

	t.Run("valid product request", func(t *testing.T) {
		body, _ := json.Marshal(ProductRequest{
			ProductID:   100,
			ProductName: "Laptop",
			Price:       999.99,
			Category:    "Electronics",
		})
		req := httptest.NewRequest("POST", "/api/products", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		assert.Equal(t, 200, w.Code)
		var result resp.Result
		json.Unmarshal(w.Body.Bytes(), &result)
		respMap, ok := result.Data.(map[string]interface{})
		assert.True(t, ok)
		assert.Equal(t, true, respMap["success"])
		innerData, ok := respMap["data"].(map[string]interface{})
		assert.True(t, ok)
		assert.Equal(t, float64(100), innerData["product_id"])
	})

	t.Run("missing required product_name", func(t *testing.T) {
		body, _ := json.Marshal(ProductRequest{
			ProductID:   100,
			ProductName: "", // required
			Price:       999.99,
			Category:    "Electronics",
		})
		req := httptest.NewRequest("POST", "/api/products", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		assert.NotEqual(t, 200, w.Code)
		var result resp.Result
		json.Unmarshal(w.Body.Bytes(), &result)
		assert.NotNil(t, result.Error)
		assert.Contains(t, result.Error.Message, "ProductName")
	})
}

// TestRouterGroupUnsupportedHandlerPanic tests that registering an unsupported handler type panics
func TestRouterGroupUnsupportedHandlerPanic(t *testing.T) {
	engine := web.Default(
		config.WithAddr(":0"),
		config.WithName("test-router-group-panic"),
		config.WithRootPath(""),
		config.WithControllerRootPkgName("webtest"),
	)

	t.Run("string handler should panic", func(t *testing.T) {
		assert.Panics(t, func() {
			engine.GET("/bad", "not-a-function")
		}, "registering a non-function handler should panic")
	})

	t.Run("int handler should panic", func(t *testing.T) {
		assert.Panics(t, func() {
			engine.GET("/bad2", 123)
		}, "registering an int handler should panic")
	})

	t.Run("nil handler should panic", func(t *testing.T) {
		assert.Panics(t, func() {
			engine.GET("/bad3", nil)
		}, "registering a nil handler should panic")
	})

	t.Run("struct handler should panic", func(t *testing.T) {
		assert.Panics(t, func() {
			engine.GET("/bad4", UserRequest{})
		}, "registering a struct handler should panic")
	})
}

// TestRouterGroupImplementsIRoutes verifies RouterGroup implements IRoutes interface
func TestRouterGroupImplementsIRoutes(t *testing.T) {
	var _ router.IRoutes = (*router.RouterGroup)(nil)
	var _ router.IRouter = (*router.RouterGroup)(nil)
}

// TestRouterGroupMiddlewareWithNext tests that middleware can call ctx.Next()
// and execute post-handler logic after the handler completes.
func TestRouterGroupMiddlewareWithNext(t *testing.T) {
	engine := web.Default(
		config.WithAddr(":0"),
		config.WithName("test-next"),
		config.WithRootPath(""),
		config.WithControllerRootPkgName("webtest"),
	)

	engine.Use(func(ctx context.Context, reqCtx *app.RequestContext) {
		reqCtx.SetHeader("X-Before", "true")
		reqCtx.Next(ctx)
		reqCtx.SetHeader("X-After", "true")
	})

	engine.GET("/test", func(ctx context.Context, reqCtx *app.RequestContext) {
		reqCtx.JSON(200, map[string]string{"message": "hello"})
	})

	handler := engine.Handler()

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	assert.Equal(t, "true", w.Header().Get("X-Before"))
	assert.Equal(t, "true", w.Header().Get("X-After"))

	var resp map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "hello", resp["message"])
}

// TestRouterGroupMiddlewareChainedNext tests multiple middleware calling ctx.Next() in sequence.
func TestRouterGroupMiddlewareChainedNext(t *testing.T) {
	engine := web.Default(
		config.WithAddr(":0"),
		config.WithName("test-chained-next"),
		config.WithRootPath(""),
		config.WithControllerRootPkgName("webtest"),
	)

	var order []string

	engine.Use(func(ctx context.Context, reqCtx *app.RequestContext) {
		order = append(order, "mw1-before")
		reqCtx.Next(ctx)
		order = append(order, "mw1-after")
	})

	engine.Use(func(ctx context.Context, reqCtx *app.RequestContext) {
		order = append(order, "mw2-before")
		reqCtx.Next(ctx)
		order = append(order, "mw2-after")
	})

	engine.GET("/test", func(ctx context.Context, reqCtx *app.RequestContext) {
		order = append(order, "handler")
		reqCtx.JSON(200, map[string]string{"message": "ok"})
	})

	handler := engine.Handler()

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	assert.Equal(t, []string{"mw1-before", "mw2-before", "handler", "mw2-after", "mw1-after"}, order)
}

// TestRouterGroupMiddlewareWithoutNext tests that middleware NOT calling ctx.Next()
// blocks the chain (backward compatible behavior).
func TestRouterGroupMiddlewareWithoutNext(t *testing.T) {
	engine := web.Default(
		config.WithAddr(":0"),
		config.WithName("test-no-next"),
		config.WithRootPath(""),
		config.WithControllerRootPkgName("webtest"),
	)

	handlerCalled := false

	engine.Use(func(ctx context.Context, reqCtx *app.RequestContext) {
		// Middleware does NOT call ctx.Next() - handler should NOT execute
		reqCtx.JSON(200, map[string]string{"blocked": "true"})
	})

	engine.GET("/test", func(ctx context.Context, reqCtx *app.RequestContext) {
		handlerCalled = true
		reqCtx.JSON(200, map[string]string{"message": "should not reach"})
	})

	handler := engine.Handler()

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	assert.False(t, handlerCalled, "handler should not be called when middleware doesn't call Next()")

	var resp map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "true", resp["blocked"])
}

// TestRouterGroupMiddlewareAbortInterruptsNext tests that ctx.Abort() stops the chain.
func TestRouterGroupMiddlewareAbortInterruptsNext(t *testing.T) {
	engine := web.Default(
		config.WithAddr(":0"),
		config.WithName("test-abort-next"),
		config.WithRootPath(""),
		config.WithControllerRootPkgName("webtest"),
	)

	handlerCalled := false
	afterAbortCalled := false

	engine.Use(func(ctx context.Context, reqCtx *app.RequestContext) {
		reqCtx.AbortWithMsg("forbidden", 403)
		// After Abort, Next() should not execute further handlers
		reqCtx.Next(ctx)
		afterAbortCalled = true
	})

	engine.GET("/test", func(ctx context.Context, reqCtx *app.RequestContext) {
		handlerCalled = true
		reqCtx.JSON(200, map[string]string{"message": "should not reach"})
	})

	handler := engine.Handler()

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, 403, w.Code)
	assert.False(t, handlerCalled, "handler should not be called after Abort()")
	assert.True(t, afterAbortCalled, "code after Next() should still execute in the middleware")
}

// TestRouterGroupMiddlewareConditionalSkipNext tests that when a middleware in the
// chain does NOT call Next(), all subsequent middlewares and the handler are skipped.
func TestRouterGroupMiddlewareConditionalSkipNext(t *testing.T) {
	engine := web.Default(
		config.WithAddr(":0"),
		config.WithName("test-conditional-skip"),
		config.WithRootPath(""),
		config.WithControllerRootPkgName("webtest"),
	)

	var order []string

	engine.Use(func(ctx context.Context, reqCtx *app.RequestContext) {
		order = append(order, "mw1-before")
		reqCtx.Next(ctx)
		order = append(order, "mw1-after")
	})

	engine.Use(func(ctx context.Context, reqCtx *app.RequestContext) {
		order = append(order, "mw2-before")
		// mw2 does NOT call Next() - simulating a condition check failure (e.g. auth denied)
		reqCtx.JSON(403, map[string]string{"error": "forbidden"})
		order = append(order, "mw2-after")
	})

	engine.Use(func(ctx context.Context, reqCtx *app.RequestContext) {
		order = append(order, "mw3-before")
		reqCtx.Next(ctx)
		order = append(order, "mw3-after")
	})

	engine.GET("/test", func(ctx context.Context, reqCtx *app.RequestContext) {
		order = append(order, "handler")
		reqCtx.JSON(200, map[string]string{"message": "ok"})
	})

	handler := engine.Handler()

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, 403, w.Code)
	// mw2 不调用 Next()，mw3 和 handler 都不执行
	// 但 mw1 调用了 Next()，mw2 返回后 mw1-after 仍会执行（递归模型特性）
	assert.Equal(t, []string{"mw1-before", "mw2-before", "mw2-after", "mw1-after"}, order)
}

// TestRouterGroupNextWithDefaultTypeMethod tests ctx.Next() with DefaultTypeOfMethod handler.
func TestRouterGroupNextWithDefaultTypeMethod(t *testing.T) {
	engine := web.Default(
		config.WithAddr(":0"),
		config.WithName("test-next-default"),
		config.WithRootPath(""),
		config.WithControllerRootPkgName("webtest"),
	)

	middlewareCalled := false
	afterNextCalled := false

	engine.Use(func(ctx context.Context, reqCtx *app.RequestContext) {
		middlewareCalled = true
		reqCtx.SetHeader("X-Middleware", "true")
		reqCtx.Next(ctx)
		afterNextCalled = true
	})

	type GreetRequest struct {
		Name string `query:"name" validate:"required"`
	}

	engine.GET("/greet", func(req *GreetRequest) string {
		return "Hello " + req.Name
	})

	handler := engine.Handler()

	req := httptest.NewRequest("GET", "/greet?name=World", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	assert.True(t, middlewareCalled, "middleware should be called")
	assert.True(t, afterNextCalled, "code after Next() should execute")
	assert.Equal(t, "true", w.Header().Get("X-Middleware"))
}

// TestRouterGroupNextWithGroupMiddleware tests ctx.Next() in a group's middleware.
func TestRouterGroupNextWithGroupMiddleware(t *testing.T) {
	engine := web.Default(
		config.WithAddr(":0"),
		config.WithName("test-next-group"),
		config.WithRootPath(""),
		config.WithControllerRootPkgName("webtest"),
	)

	var order []string

	api := engine.Group("/api")
	api.Use(func(ctx context.Context, reqCtx *app.RequestContext) {
		order = append(order, "api-before")
		reqCtx.Next(ctx)
		order = append(order, "api-after")
	})

	api.GET("/test", func(ctx context.Context, reqCtx *app.RequestContext) {
		order = append(order, "handler")
		reqCtx.JSON(200, map[string]string{"message": "ok"})
	})

	handler := engine.Handler()

	req := httptest.NewRequest("GET", "/api/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	assert.Equal(t, []string{"api-before", "handler", "api-after"}, order)
}

// OrderController is used for testing struct method registration
type OrderController struct{}

type CreateOrderReq struct {
	ProductID int     `json:"product_id" verf:"required"`
	Quantity  int     `json:"quantity" verf:"required"`
	Price     float64 `json:"price" verf:"required"`
}

type OrderResponse struct {
	OrderID    int     `json:"order_id"`
	ProductID  int     `json:"product_id"`
	Quantity   int     `json:"quantity"`
	TotalPrice float64 `json:"total_price"`
}

func (oc *OrderController) CreateOrder(req *CreateOrderReq) *OrderResponse {
	return &OrderResponse{
		OrderID:    1001,
		ProductID:  req.ProductID,
		Quantity:   req.Quantity,
		TotalPrice: req.Price * float64(req.Quantity),
	}
}

type GetOrderReq struct {
	OrderID int `path:"orderId" verf:"required"`
}

func (oc *OrderController) GetOrder(req *GetOrderReq) *OrderResponse {
	return &OrderResponse{
		OrderID:   req.OrderID,
		ProductID: 42,
		Quantity:  2,
	}
}

// TestRouterGroupMethodValue tests registering struct method values directly (instance.Method).
func TestRouterGroupMethodValue(t *testing.T) {
	engine := web.Default(
		config.WithAddr(":0"),
		config.WithName("test-method-value"),
		config.WithRootPath(""),
		config.WithControllerRootPkgName("webtest"),
	)

	// Register method value directly: instance.MethodName
	oc := &OrderController{}
	engine.POST("/orders", oc.CreateOrder)
	engine.GET("/orders/:orderId", oc.GetOrder)

	handler := engine.Handler()

	// Test POST /orders
	body := `{"product_id":1,"quantity":3,"price":29.99}`
	req := httptest.NewRequest("POST", "/orders", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].(map[string]interface{})
	assert.Equal(t, float64(1001), data["order_id"])
	assert.Equal(t, float64(1), data["product_id"])
	assert.Equal(t, float64(3), data["quantity"])

	// Test GET /orders/:orderId
	req2 := httptest.NewRequest("GET", "/orders/55", nil)
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)

	assert.Equal(t, 200, w2.Code)
	var resp2 map[string]interface{}
	json.Unmarshal(w2.Body.Bytes(), &resp2)
	data2 := resp2["data"].(map[string]interface{})
	assert.Equal(t, float64(55), data2["order_id"])
}

// TestRouterGroupMethodValueWithMiddleware tests method value registration with middleware.
func TestRouterGroupMethodValueWithMiddleware(t *testing.T) {
	engine := web.Default(
		config.WithAddr(":0"),
		config.WithName("test-method-value-mw"),
		config.WithRootPath(""),
		config.WithControllerRootPkgName("webtest"),
	)

	var order []string

	engine.Use(func(ctx context.Context, reqCtx *app.RequestContext) {
		order = append(order, "mw-before")
		reqCtx.Next(ctx)
		order = append(order, "mw-after")
	})

	oc := &OrderController{}
	engine.POST("/orders", oc.CreateOrder)

	handler := engine.Handler()

	body := `{"product_id":1,"quantity":2,"price":10.0}`
	req := httptest.NewRequest("POST", "/orders", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	assert.Equal(t, []string{"mw-before", "mw-after"}, order)
}

// TestRouterGroupMethodValueInGroup tests method value registration within a RouterGroup.
func TestRouterGroupMethodValueInGroup(t *testing.T) {
	engine := web.Default(
		config.WithAddr(":0"),
		config.WithName("test-method-value-group"),
		config.WithRootPath(""),
		config.WithControllerRootPkgName("webtest"),
	)

	oc := &OrderController{}

	api := engine.Group("/api/v1")
	api.Use(func(ctx context.Context, reqCtx *app.RequestContext) {
		reqCtx.SetHeader("X-API", "v1")
		reqCtx.Next(ctx)
	})
	api.POST("/orders", oc.CreateOrder)
	api.GET("/orders/:orderId", oc.GetOrder)

	handler := engine.Handler()

	// Test POST /api/v1/orders
	body := `{"product_id":5,"quantity":1,"price":99.99}`
	req := httptest.NewRequest("POST", "/api/v1/orders", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	assert.Equal(t, "v1", w.Header().Get("X-API"))

	// Test GET /api/v1/orders/:orderId
	req2 := httptest.NewRequest("GET", "/api/v1/orders/10", nil)
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)

	assert.Equal(t, 200, w2.Code)
	assert.Equal(t, "v1", w2.Header().Get("X-API"))
}

// ===== Wrapper handler for testing XxxServiceYyyHandler naming pattern =====

// IServiceSearchHandler is a wrapper that exports the unexported protoc-generated
// _IService_Search_Handler. This simulates the pattern used in dagflow/backend/internal/proto/handlers.go
// where internal proto handlers are wrapped with exported functions.
func IServiceSearchHandler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	return _IService_Search_Handler(srv, ctx, dec, interceptor)
}

// TestRouterGroupGRPCWithWrapperHandler tests gRPC route registration using wrapper
// functions that follow the XxxServiceYyyHandler naming convention.
// This verifies that extractGRPCMethodName correctly parses method names from
// wrapper functions like "IServiceSearchHandler" → "Search".
func TestRouterGroupGRPCWithWrapperHandler(t *testing.T) {
	engine := web.Default(
		config.WithAddr(":0"),
		config.WithName("test-grpc-wrapper"),
		config.WithRootPath(""),
		config.WithControllerRootPkgName("webtest"),
	)

	// Register using wrapper handler (XxxServiceYyyHandler pattern)
	engine.GRPC("POST", "/grpc/wrapper-search", IServiceSearchHandler, &HelloImpl{})

	handler := engine.Handler()

	// Verify route is registered and handler executes correctly
	reqBody := `{"query":"2","hobby":["go","rust"]}`
	req := httptest.NewRequest("POST", "/grpc/wrapper-search", bytes.NewReader([]byte(reqBody)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.NotEqual(t, 404, w.Code, "wrapper handler route should be registered")
	assert.Equal(t, 200, w.Code)
}

// TestRouterGroupGRPCWrapperAndProtocCoexist tests that wrapper handlers and
// direct protoc handlers can coexist in the same engine.
func TestRouterGroupGRPCWrapperAndProtocCoexist(t *testing.T) {
	engine := web.Default(
		config.WithAddr(":0"),
		config.WithName("test-grpc-coexist"),
		config.WithRootPath(""),
		config.WithControllerRootPkgName("webtest"),
	)

	// Register with protoc-generated handler (underscore pattern)
	engine.GRPC("POST", "/grpc/protoc-search", _IService_Search_Handler, &HelloImpl{})
	// Register with wrapper handler (XxxServiceYyyHandler pattern)
	engine.GRPC("POST", "/grpc/wrapper-search", IServiceSearchHandler, &HelloImpl{})

	handler := engine.Handler()

	reqBody := `{"query":"2","hobby":["go","rust"]}`

	// Test protoc route
	req1 := httptest.NewRequest("POST", "/grpc/protoc-search", bytes.NewReader([]byte(reqBody)))
	req1.Header.Set("Content-Type", "application/json")
	w1 := httptest.NewRecorder()
	handler.ServeHTTP(w1, req1)
	assert.Equal(t, 200, w1.Code, "protoc handler route should work")

	// Test wrapper route
	req2 := httptest.NewRequest("POST", "/grpc/wrapper-search", bytes.NewReader([]byte(reqBody)))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)
	assert.Equal(t, 200, w2.Code, "wrapper handler route should work")
}
