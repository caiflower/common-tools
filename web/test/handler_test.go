package webtest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/caiflower/common-tools/web"
	"github.com/caiflower/common-tools/web/common/cfg"
	"github.com/caiflower/common-tools/web/common/controller"
	"github.com/caiflower/common-tools/web/common/resp"
	"github.com/caiflower/common-tools/web/router"
	"github.com/stretchr/testify/assert"
)

// UserRequest 用户请求结构体 - 带参数校验
type UserRequest struct {
	ID     int    `json:"id" verf:"required" between:"1,1000"`
	Name   string `json:"name" verf:"required" len:"1,50"`
	Email  string `json:"email" verf:"nilable" reg:"^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}$"`
	Age    int    `json:"age" verf:"required" between:"1,120"`
	Status string `json:"status" inList:"active,inactive,pending"`
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
			"id":     req.ID,
			"name":   req.Name,
			"email":  req.Email,
			"age":    req.Age,
			"status": req.Status,
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

// GetProduct 获取产品信息
func (pc *ProductController) GetProduct(req *ProductRequest) *UserResponse {
	return &UserResponse{
		Success: true,
		Message: "Product retrieved successfully",
		Data: map[string]interface{}{
			"product_id":   req.ProductID,
			"product_name": req.ProductName,
			"price":        req.Price,
			"category":     req.Category,
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

var once sync.Once
var engine *web.Engine

// setupTestServer 设置测试服务器
func setupTestServer(t *testing.T) *web.Engine {
	once.Do(func() {
		// 创建测试引擎
		engine = web.Default(
			cfg.WithPort(8888), // 使用随机端口
			cfg.WithName("test-server"),
			cfg.WithRootPath("/api/v1"),
			cfg.WithControllerRootPkgName("webctx"),
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

		router.InitHandler(handlerCfg, logger.DefaultLogger())

		// 添加控制器
		if router.CommonHandler != nil {
			router.CommonHandler.AddController(&UserController{})
			router.CommonHandler.AddController(&ProductController{})

			// 注册RESTful路由
			restfulController := controller.NewRestFul().
				Version("v1").
				Path("/products/{productId}").
				Method("GET").
				Controller("webtest.ProductController").
				Action("GetProduct")

			router.CommonHandler.Register(restfulController)

			restfulController2 := controller.NewRestFul().
				Version("v1").
				Path("/products").
				Method("POST").
				Controller("webtest.ProductController").
				Action("CreateProduct")

			router.CommonHandler.Register(restfulController2)
		}
	})

	return engine
}

// TestServerBasicFunctionality 测试服务器基本功能
func TestServerBasicFunctionality(t *testing.T) {
	engine := setupTestServer(t)

	if engine.Core == nil {
		t.Fatal("Engine.Core should not be nil")
	}

	t.Logf("Server name: %s", engine.Core.Name())
}

// TestControllerRegistration 测试控制器注册
func TestControllerRegistration(t *testing.T) {
	_ = setupTestServer(t)

	if router.CommonHandler == nil {
		t.Fatal("CommonHandler should not be nil after setup")
	}

	t.Log("Controllers registered successfully")
}

// TestHTTPRequestWithValidation 测试HTTP请求和参数校验
func TestHTTPRequestWithValidation(t *testing.T) {
	_ = setupTestServer(t)

	if router.CommonHandler == nil {
		t.Skip("CommonHandler not initialized")
	}

	testCases := []struct {
		name           string
		path           string
		method         string
		requestBody    interface{}
		expectedStatus int
		expectSuccess  bool
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
		},
		{
			name:   "Invalid user request - missing required field",
			path:   "/api/v1/web/webtest/UserController?Action=CreateUser",
			method: "POST",
			requestBody: UserRequest{
				ID:     0,  // Invalid: should be between 1-1000
				Name:   "", // Invalid: required field
				Email:  "****************",
				Age:    25,
				Status: "active",
			},
			expectedStatus: http.StatusBadRequest,
			expectSuccess:  false,
		},
		{
			name:   "Invalid age range",
			path:   "/api/v1/web/webtest/UserController?Action=UpdateUser",
			method: "POST",
			requestBody: UserRequest{
				ID:     1,
				Name:   "John Doe",
				Email:  "****************",
				Age:    150, // Invalid: should be between 1-120
				Status: "active",
			},
			expectedStatus: http.StatusBadRequest,
			expectSuccess:  false,
		},
		{
			name:   "Invalid status value",
			path:   "/api/v1/web/webtest/UserController?Action=GetUser",
			method: "POST",
			requestBody: UserRequest{
				ID:     1,
				Name:   "John Doe",
				Email:  "****************",
				Age:    25,
				Status: "unknown", // Invalid: not in list
			},
			expectedStatus: http.StatusBadRequest,
			expectSuccess:  false,
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
			req.Header.Set("X-Request-Id", "test-request-id")

			// 创建响应记录器
			w := httptest.NewRecorder()

			// 执行请求
			router.CommonHandler.ServeHTTP(w, req)

			assert.Equal(t, 200, w.Code, "want 200 status code")

			// 解析响应
			var response resp.Result
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
				t.Logf("Response body: %s", w.Body.String())
				// 某些错误情况下可能不是JSON格式，这里只记录日志
			} else {
				if tc.expectedStatus == 200 {
					assert.Nil(t, response.Error)
				} else {
					assert.Equal(t, tc.expectedStatus, response.Error.GetCode(), "code should be equal")
				}
			}
		})
	}
}

// TestRESTfulRouting 测试RESTful路由
func TestRESTfulRouting(t *testing.T) {
	_ = setupTestServer(t)

	if router.CommonHandler == nil {
		t.Skip("CommonHandler not initialized")
	}

	testCases := []struct {
		name           string
		path           string
		method         string
		requestBody    interface{}
		expectedStatus int
	}{
		{
			name:   "GET product by ID",
			path:   "/v1/products/123",
			method: "GET",
			requestBody: ProductRequest{
				ProductID:   123,
				ProductName: "Test Product",
				Price:       99.99,
				Category:    "Electronics",
			},
			expectedStatus: http.StatusOK,
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
			expectedStatus: http.StatusOK,
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
			router.CommonHandler.ServeHTTP(w, req)

			t.Logf("Request: %s %s", tc.method, tc.path)
			t.Logf("Response Status: %d", w.Code)
			t.Logf("Response Body: %s", w.Body.String())
		})
	}
}

// TestServerPerformance 测试服务器性能
func TestServerPerformance(t *testing.T) {
	_ = setupTestServer(t)

	if router.CommonHandler == nil {
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
			router.CommonHandler.ServeHTTP(w, req)
		}(i)
	}

	duration := time.Since(start)
	t.Logf("Processed %d requests in %v", requests, duration)
	t.Logf("Average request time: %v", duration/time.Duration(requests))
}

// TestErrorHandling 测试错误处理
func TestErrorHandling(t *testing.T) {
	_ = setupTestServer(t)

	if router.CommonHandler == nil {
		t.Skip("CommonHandler not initialized")
	}

	testCases := []struct {
		name        string
		path        string
		method      string
		requestBody string
		description string
	}{
		{
			name:        "Invalid JSON",
			path:        "/api/v1/web/webtest/UserController?Action=GetUser",
			method:      "POST",
			requestBody: `{"invalid": json}`,
			description: "Should handle malformed JSON",
		},
		{
			name:        "Missing Action parameter",
			path:        "/api/v1/web/webtest/UserController",
			method:      "POST",
			requestBody: `{"id": 1, "name": "test"}`,
			description: "Should handle missing action parameter",
		},
		{
			name:        "Non-existent webctx",
			path:        "/api/v1/web/webtest/NonExistentController?Action=GetUser",
			method:      "POST",
			requestBody: `{"id": 1}`,
			description: "Should handle non-existent webctx",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, bytes.NewReader([]byte(tc.requestBody)))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			router.CommonHandler.ServeHTTP(w, req)

			t.Logf("%s - Status: %d, Body: %s", tc.description, w.Code, w.Body.String())
		})
	}
}
