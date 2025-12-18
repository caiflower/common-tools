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
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/caiflower/common-tools/web"
	"github.com/caiflower/common-tools/web/common/resp"
	"github.com/caiflower/common-tools/web/router"
	"github.com/caiflower/common-tools/web/router/controller"
	"github.com/caiflower/common-tools/web/server/config"
	"github.com/stretchr/testify/assert"
)

// UserRequest 用户请求结构体 - 带参数校验
type UserRequest struct {
	RequestID string `header:"X-Request-Id"`
	ID        int    `json:"id" verf:"required" between:"1,1000"`
	Name      string `json:"name" verf:"required" len:"1,100"`
	Email     string `json:"email" verf:"nilable" reg:"^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}$"`
	Age       int    `json:"age" verf:"required" between:"1,120"`
	Status    string `json:"status" inList:"active,inactive,pending"`
}

// UserResponse 用户响应结构体
type UserResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

// UserController 用户控制器
type UserController struct{}

// GetUser 获取用户信息 - 测试基本控制器功能
func (uc *UserController) GetUser(req *UserRequest) *UserResponse {
	return &UserResponse{
		Success: true,
		Message: "User retrieved successfully",
		Data: map[string]interface{}{
			"id":        req.ID,
			"name":      req.Name,
			"email":     req.Email,
			"age":       req.Age,
			"status":    req.Status,
			"requestId": req.RequestID,
		},
	}
}

// CreateUser 创建用户 - 测试参数校验
func (uc *UserController) CreateUser(req *UserRequest) *UserResponse {
	return &UserResponse{
		Success: true,
		Message: "User created successfully",
		Data: map[string]interface{}{
			"id":     req.ID,
			"name":   req.Name,
			"email":  req.Email,
			"age":    req.Age,
			"status": req.Status,
		},
	}
}

// UpdateUser 更新用户
func (uc *UserController) UpdateUser(req *UserRequest) *UserResponse {
	return &UserResponse{
		Success: true,
		Message: "User updated successfully",
		Data: map[string]interface{}{
			"id":     req.ID,
			"name":   req.Name,
			"email":  req.Email,
			"age":    req.Age,
			"status": req.Status,
		},
	}
}

// ProductController RESTful风格控制器
type ProductController struct{}

// ProductRequest 产品请求
type ProductRequest struct {
	ProductID   int     `json:"product_id" verf:"required" between:"1,10000"`
	ProductName string  `json:"product_name" verf:"required" len:"1,100"`
	Price       float64 `json:"price" verf:"required"`
	Category    string  `json:"category" verf:"required" len:"1,50"`
}

type ProductRequestV1 struct {
	ProductID int `json:"product_id" verf:"required" between:"1,10000"`
}

// GetProduct 获取产品信息
func (pc *ProductController) GetProduct(req *ProductRequestV1) *UserResponse {
	return &UserResponse{
		Success: true,
		Message: "Product retrieved successfully",
		Data: map[string]interface{}{
			"product_id":   req.ProductID,
			"product_name": "Test Product",
			"price":        99.99,
			"category":     "Electronics",
		},
	}
}

// CreateProduct 创建产品
func (pc *ProductController) CreateProduct(req *ProductRequest) *UserResponse {
	return &UserResponse{
		Success: true,
		Message: "Product created successfully",
		Data: map[string]interface{}{
			"product_id":   req.ProductID,
			"product_name": req.ProductName,
			"price":        req.Price,
			"category":     req.Category,
		},
	}
}

func (pc *ProductController) CreateProductPanic() *UserResponse {
	panic("panic")
}

var once sync.Once
var engine *web.Engine
var handler *router.Handler

// setupTestServer 设置测试服务器
func setupTestServer(t *testing.T) (*web.Engine, *router.Handler) {
	once.Do(func() {
		// 创建测试引擎
		engine = web.Default(
			config.WithAddr(":8888"),
			config.WithName("test-server"),
			config.WithRootPath("/api/v1"),
			config.WithControllerRootPkgName("webctx"),
		)

		if engine == nil {
			t.Fatal("Failed to create engine")
		}

		// 初始化处理器
		handlerCfg := router.HandlerCfg{
			Name:                  "test-server",
			RootPath:              "/api/v1",
			HeaderTraceID:         "X-Request-Id",
			ControllerRootPkgName: "webctx",
			EnablePprof:           false,
		}

		handler = router.NewHandler(handlerCfg, logger.DefaultLogger())

		// 添加控制器
		if handler != nil {
			handler.AddController(&UserController{})
			productController := handler.AddController(&ProductController{})

			group := controller.NewRestFul().Version("v1").Group("/products")

			restfulController := group.
				Path("/:productID").
				Method("GET").
				Controller(productController.GetPaths()[0]).
				Action("GetProduct")

			handler.Register(restfulController)

			restfulController2 := group.
				Method("POST").
				TargetMethod(productController.GetTargetMethod("CreateProduct"))

			handler.Register(restfulController2)

			restfulController3 := group.
				Method("POST").
				Path("panic").
				TargetMethod(productController.GetTargetMethod("CreateProductPanic"))
			handler.Register(restfulController3)
		}
	})

	return engine, handler
}

// TestServerBasicFunctionality 测试服务器基本功能
func TestServerBasicFunctionality(t *testing.T) {
	engine, handler = setupTestServer(t)

	if engine.Core == nil {
		t.Fatal("Engine.Core should not be nil")
	}
	if handler == nil {
		t.Fatal("handler should not be nil")
	}

	t.Logf("Server name: %s", engine.Core.Name())
}

// TestHTTPRequestWithValidation 测试HTTP请求和参数校验
func TestHTTPRequestWithValidation(t *testing.T) {
	engine, handler = setupTestServer(t)
	if handler == nil {
		t.Skip("CommonHandler not initialized")
	}

	requestId := "test-request-id"
	testCases := []struct {
		name             string
		path             string
		method           string
		requestBody      interface{}
		expectedStatus   int
		expectSuccess    bool
		expectErrMessage string
		expectData       interface{}
	}{
		{
			name:   "Valid user request",
			path:   "/api/v1/web/webtest/UserController?Action=GetUser",
			method: "POST",
			requestBody: UserRequest{
				ID:     1,
				Name:   "John Doe",
				Email:  "1239811789@qq.com",
				Age:    25,
				Status: "active",
			},
			expectedStatus: http.StatusOK,
			expectSuccess:  true,
			expectData: map[string]interface{}{
				"success": true,
				"message": "User retrieved successfully",
				"data": map[string]interface{}{
					"id":        float64(1),
					"name":      "John Doe",
					"email":     "1239811789@qq.com",
					"age":       float64(25),
					"status":    "active",
					"requestId": requestId,
				},
			},
		},
		{
			name:   "nilable email",
			path:   "/api/v1/web/webtest/UserController?Action=GetUser",
			method: "POST",
			requestBody: UserRequest{
				ID:     1,
				Name:   "John Doe",
				Age:    25,
				Status: "active",
			},
			expectedStatus: http.StatusOK,
			expectSuccess:  true,
			expectData: map[string]interface{}{
				"success": true,
				"message": "User retrieved successfully",
				"data": map[string]interface{}{
					"id":        float64(1),
					"name":      "John Doe",
					"email":     "",
					"age":       float64(25),
					"status":    "active",
					"requestId": requestId,
				},
			},
		},
		{
			name:   "name out of range",
			path:   "/api/v1/web/webtest/UserController?Action=GetUser",
			method: "POST",
			requestBody: UserRequest{
				ID:     1,
				Name:   strings.Repeat("a", 101),
				Age:    25,
				Status: "active",
			},
			expectedStatus:   http.StatusBadRequest,
			expectSuccess:    false,
			expectErrMessage: `UserRequest.Name len is greater than 100`,
		},
		{
			name:   "Invalid user request - missing required field",
			path:   "/api/v1/web/webtest/UserController?Action=CreateUser",
			method: "POST",
			requestBody: UserRequest{
				ID:     0, // Invalid: should be between 1-1000
				Name:   "John Doe",
				Email:  "1239811789@qq.com",
				Age:    25,
				Status: "active",
			},
			expectedStatus:   http.StatusBadRequest,
			expectSuccess:    false,
			expectErrMessage: `UserRequest.ID is not between 1 and 1000`,
		},
		{
			name:   "Invalid user request - missing required field",
			path:   "/api/v1/web/webtest/UserController?Action=CreateUser",
			method: "POST",
			requestBody: UserRequest{
				ID:     1,
				Name:   "", // Invalid: required field
				Email:  "1239811789@qq.com",
				Age:    25,
				Status: "active",
			},
			expectedStatus:   http.StatusBadRequest,
			expectSuccess:    false,
			expectErrMessage: "UserRequest.Name is missing",
		},
		{
			name:   "Invalid age range",
			path:   "/api/v1/web/webtest/UserController?Action=UpdateUser",
			method: "POST",
			requestBody: UserRequest{
				ID:     1,
				Name:   "John Doe",
				Email:  "1239811789@qq.com",
				Age:    150, // Invalid: should be between 1-120
				Status: "active",
			},
			expectedStatus:   http.StatusBadRequest,
			expectSuccess:    false,
			expectErrMessage: "UserRequest.Age is not between 1 and 120",
		},
		{
			name:   "Invalid status value",
			path:   "/api/v1/web/webtest/UserController?Action=GetUser",
			method: "POST",
			requestBody: UserRequest{
				ID:     1,
				Name:   "John Doe",
				Email:  "1239811789@qq.com",
				Age:    25,
				Status: "unknown", // Invalid: not in list
			},
			expectedStatus:   http.StatusBadRequest,
			expectSuccess:    false,
			expectErrMessage: `UserRequest.Status is not in [active inactive pending]`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// 准备请求体
			requestBody, err := json.Marshal(tc.requestBody)
			if err != nil {
				t.Fatalf("Failed to marshal request body: %v", err)
			}

			// 创建HTTP请求
			req := httptest.NewRequest(tc.method, tc.path, bytes.NewReader(requestBody))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Request-Id", requestId)

			// 创建响应记录器
			w := httptest.NewRecorder()

			// 执行请求
			handler.ServeHTTP(w, req)

			assert.Equal(t, 200, w.Code, "want 200 status code")

			// 解析响应
			var response resp.Result
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
				t.Logf("Response body: %s", w.Body.String())
				// 某些错误情况下可能不是JSON格式，这里只记录日志
			} else {
				if tc.expectedStatus == 200 {
					assert.Nil(t, response.Error)
					assert.Equal(t, response.Data, tc.expectData)
					assert.NotNil(t, response.RequestId, "want request id not nil")
				} else {
					assert.Equal(t, tc.expectedStatus, response.Error.GetCode(), "code should be equal")
					assert.Equal(t, tc.expectErrMessage, response.Error.GetMessage(), "message should be equal")
				}
			}
			assert.Equal(t, response.RequestId, requestId, "request id should be equal")
		})
	}
}

// TestRESTfulRouting 测试RESTful路由
func TestRESTfulRouting(t *testing.T) {
	engine, handler = setupTestServer(t)

	if handler == nil {
		t.Skip("CommonHandler not initialized")
	}

	testCases := []struct {
		name           string
		path           string
		method         string
		requestBody    interface{}
		expectData     interface{}
		expectedStatus int
	}{
		{
			name:   "GET product by ID",
			path:   "/v1/products/123",
			method: "GET",
			expectData: map[string]interface{}{
				"success": true,
				"message": "Product retrieved successfully",
				"data": map[string]interface{}{
					"product_id":   float64(123),
					"product_name": "Test Product",
					"price":        float64(99.99),
					"category":     "Electronics",
				},
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "GET product by ID Not found",
			path:           "/v1/products/123",
			method:         "POST",
			expectedStatus: http.StatusNotFound,
		},
		{
			name:   "POST create product",
			path:   "/v1/products",
			method: "POST",
			requestBody: ProductRequest{
				ProductID:   456,
				ProductName: "New Product",
				Price:       149.99,
				Category:    "Books",
			},
			expectData: map[string]interface{}{
				"success": true,
				"message": "Product created successfully",
				"data": map[string]interface{}{
					"product_id":   float64(456),
					"product_name": "New Product",
					"price":        float64(149.99),
					"category":     "Books",
				},
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:   "POST create product",
			path:   "/v1/products",
			method: "PUT",
			requestBody: ProductRequest{
				ProductID:   456,
				ProductName: "New Product",
				Price:       149.99,
				Category:    "Books",
			},
			expectedStatus: http.StatusNotFound,
		},
		{
			name:   "POST create product failed",
			path:   "/v1/products",
			method: "POST",
			requestBody: ProductRequest{
				ProductID:   10001,
				ProductName: "New Product",
				Price:       149.99,
				Category:    "Books",
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "POST create product panic",
			path:           "/v1/products/panic",
			method:         "POST",
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// 准备请求体
			requestBody, err := json.Marshal(tc.requestBody)
			if err != nil {
				t.Fatalf("Failed to marshal request body: %v", err)
			}

			// 创建HTTP请求
			req := httptest.NewRequest(tc.method, tc.path, bytes.NewReader(requestBody))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Request-Id", "test-restful-request")

			// 创建响应记录器
			w := httptest.NewRecorder()

			// 执行请求
			handler.ServeHTTP(w, req)

			var response resp.Result
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
				t.Logf("Response body: %s", w.Body.String())
				// 某些错误情况下可能不是JSON格式，这里只记录日志
			}
			assert.Equal(t, tc.expectedStatus, w.Code, tc.name)
			if w.Code == http.StatusOK {
				assert.Equal(t, response.Data, tc.expectData)
				assert.NotNil(t, response.Data, tc.name)
			} else {
				assert.NotNil(t, response.Error, tc.name)
				assert.Equal(t, response.Error.GetCode(), w.Code, tc.name)
			}

			t.Logf("Request: %s %s", tc.method, tc.path)
			t.Logf("Response Status: %d", w.Code)
			t.Logf("Response Body: %s", w.Body.String())
		})
	}
}

// TestServerPerformance 测试服务器性能
func TestServerPerformance(t *testing.T) {
	engine, handler = setupTestServer(t)

	if handler == nil {
		t.Skip("CommonHandler not initialized")
	}

	// 并发测试
	requests := 100

	start := time.Now()

	for i := 0; i < requests; i++ {
		go func(index int) {
			requestBody := UserRequest{
				ID:     index + 1,
				Name:   fmt.Sprintf("User%d", index),
				Email:  fmt.Sprintf("user%d@example.com", index),
				Age:    25 + (index % 50),
				Status: "active",
			}

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/web/webtest/UserController?Action=GetUser", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
		}(i)
	}

	duration := time.Since(start)
	t.Logf("Processed %d requests in %v", requests, duration)
	t.Logf("Average request time: %v", duration/time.Duration(requests))
}

// TestErrorHandling 测试错误处理
func TestErrorHandling(t *testing.T) {
	engine, handler = setupTestServer(t)

	if handler == nil {
		t.Skip("CommonHandler not initialized")
	}

	testCases := []struct {
		name           string
		path           string
		method         string
		requestBody    string
		description    string
		expectedStatus int
	}{
		{
			name:           "Invalid JSON",
			path:           "/api/v1/web/webtest/UserController?Action=GetUser",
			method:         "POST",
			requestBody:    `{"invalid": json}`,
			description:    "Should handle malformed JSON",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Missing Action parameter",
			path:           "/api/v1/web/webtest/UserController",
			method:         "POST",
			requestBody:    `{"id": 1, "name": "test"}`,
			description:    "Should handle missing action parameter",
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "Non-existent webctx",
			path:           "/api/v1/web/webtest/NonExistentController?Action=GetUser",
			method:         "POST",
			requestBody:    `{"id": 1}`,
			description:    "Should handle non-existent webctx",
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, bytes.NewReader([]byte(tc.requestBody)))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			var response resp.Result
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
				t.Logf("Response body: %s", w.Body.String())
			} else {
				assert.Equal(t, tc.expectedStatus, response.Error.GetCode(), "code should be equal")
			}

			t.Logf("%s - Status: %d, Body: %s", tc.description, w.Code, w.Body.String())
		})
	}
}
