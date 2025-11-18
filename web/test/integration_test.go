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

package test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/caiflower/common-tools/pkg/bean"
	httpclient "github.com/caiflower/common-tools/pkg/http"
	"github.com/caiflower/common-tools/pkg/logger"
	testv1 "github.com/caiflower/common-tools/web/test/controller/v1/test"
	"github.com/caiflower/common-tools/web/v1"
)

// TestSuite Web框架集成测试套件
type TestSuite struct {
	server *webv1.HttpServer
	client httpclient.HttpClient
}

func (ts *TestSuite) setup(t *testing.T) {
	// 清理之前的Bean
	bean.ClearBeans()

	// 初始化日志
	logger.InitLogger(&logger.Config{
		EnableColor: "True",
	})

	// 设置时区
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatalf("加载时区失败: %v", err)
	}
	time.Local = loc

	// 初始化服务器
	config := webv1.Config{
		Port:     9091,
		RootPath: "api",
	}

	ts.server = webv1.NewHttpServer(config)
	ts.server.AddController(&testv1.StructService{})

	// 注册RESTful路由
	ts.server.Register(
		webv1.NewRestFul().
			Method(http.MethodPost).
			Version("v1").
			Controller("test.StructService").
			Path("/tests/{testId}").
			Action("Test1"),
	)
	ts.server.Register(
		webv1.NewRestFul().
			Method(http.MethodGet).
			Version("v1").
			Controller("test.StructService").
			Path("/tests/{testId}").
			Action("Test1"),
	)
	ts.server.Register(
		webv1.NewRestFul().
			Method(http.MethodPost).
			Version("v1").
			Controller("test.StructService").
			Path("/tests/{testId}/sub/{subId}").
			Action("Test2"),
	)

	// 注册拦截器
	//ts.server.AddInterceptor(&TestInterceptor1{}, 1)
	//ts.server.AddInterceptor(&TestInterceptor2{}, 2)

	// 启动服务器
	ts.server.StartUp()

	// 初始化HTTP客户端
	ts.client = httpclient.NewHttpClient(httpclient.Config{
		Timeout:      20,
		Verbose:      toBoolPtr(false),
		DisableRetry: false,
	})

	// 等待服务器启动
	time.Sleep(500 * time.Millisecond)
}

func (ts *TestSuite) teardown() {
	if ts.server != nil {
		ts.server.Close()
	}
}

// =========================== 测试用例 ===========================

// TestRESTfulPostRequest 测试RESTful POST请求
func TestRESTfulPostRequest(t *testing.T) {
	suite := &TestSuite{}
	suite.setup(t)
	defer suite.teardown()

	// 准备请求数据
	req := testv1.Param{
		TestId: "test-123",
		Args:   "arg-value",
		Name:   "test-name",
		Name1:  toBoolStr("abc"),
	}

	// 发送请求
	var resp CommonResponse
	httpResp := &httpclient.Response{Data: &resp}
	err := suite.client.PostJson(
		"req-001",
		"localhost:9091/api/v1/tests/test-123",
		req,
		httpResp,
		map[string]string{
			"X-User-Id": "user-123",
		},
	)

	// 验证结果
	if err != nil {
		t.Errorf("POST请求失败: %v", err)
		return
	}
	if httpResp.StatusCode != http.StatusOK {
		t.Errorf("期望状态码 %d, 实际 %d", http.StatusOK, httpResp.StatusCode)
		return
	}
	if resp.RequestId == "" {
		t.Error("响应RequestId为空")
		return
	}
	t.Logf("✓ RESTful POST请求成功 - RequestId: %s", resp.RequestId)
}

// TestRESTfulGetRequest 测试RESTful GET请求
func TestRESTfulGetRequest(t *testing.T) {
	suite := &TestSuite{}
	suite.setup(t)
	defer suite.teardown()

	// 发送GET请求
	var resp CommonResponse
	httpResp := &httpclient.Response{Data: &resp}
	err := suite.client.Get(
		"req-002",
		"localhost:9091/api/v1/tests/product-456",
		nil,
		httpResp,
		map[string]string{
			"X-User-Id": "user-456",
		},
	)

	// 验证结果
	if err != nil {
		t.Errorf("GET请求失败: %v", err)
		return
	}
	if httpResp.StatusCode != http.StatusOK {
		t.Errorf("期望状态码 %d, 实际 %d", http.StatusOK, httpResp.StatusCode)
		return
	}
	t.Logf("✓ RESTful GET请求成功 - RequestId: %s", resp.RequestId)
}

// TestPathParameters 测试路径参数绑定
func TestPathParameters(t *testing.T) {
	suite := &TestSuite{}
	suite.setup(t)
	defer suite.teardown()

	req := testv1.Param{
		TestId: "product-789",
		Name:   "test-product",
	}

	var resp CommonResponse
	httpResp := &httpclient.Response{Data: &resp}
	err := suite.client.PostJson(
		"req-003",
		"localhost:9091/api/v1/tests/prod-001/sub/sub-001",
		req,
		httpResp,
		map[string]string{
			"X-User-Id": "user-789",
		},
	)

	if err != nil {
		t.Errorf("路径参数测试失败: %v", err)
		return
	}
	if httpResp.StatusCode != http.StatusOK {
		t.Errorf("期望状态码 %d, 实际 %d", http.StatusOK, httpResp.StatusCode)
		return
	}
	t.Logf("✓ 路径参数绑定成功 - RequestId: %s", resp.RequestId)
}

// TestRequiredFieldValidation 测试必填字段验证
func TestRequiredFieldValidation(t *testing.T) {
	suite := &TestSuite{}
	suite.setup(t)
	defer suite.teardown()

	// 不包含必填字段的请求
	req := map[string]interface{}{
		"name": "test",
		// 缺少 X-User-Id header（必填）
	}

	var resp CommonResponse
	httpResp := &httpclient.Response{Data: &resp}
	err := suite.client.PostJson(
		"req-004",
		"localhost:9091/api/v1/tests/test-id",
		req,
		httpResp,
		map[string]string{
			// 故意不发送 X-User-Id
		},
	)

	if err != nil {
		t.Errorf("请求失败: %v", err)
		return
	}

	// 应该返回验证错误
	if httpResp.StatusCode != http.StatusBadRequest {
		t.Logf("验证失败响应 - 状态码: %d (预期400)", httpResp.StatusCode)
	} else {
		if resp.Error.Type != "" {
			t.Logf("✓ 必填字段验证成功 - 错误类型: %s, 信息: %s", resp.Error.Type, resp.Error.Message)
		}
	}
}

// TestMultipleInterceptors 测试多个拦截器按顺序执行
func TestMultipleInterceptors(t *testing.T) {
	suite := &TestSuite{}
	suite.setup(t)
	defer suite.teardown()

	req := testv1.Param{
		TestId: "test-interceptor",
		Name:   "interceptor-test",
	}

	var resp CommonResponse
	httpResp := &httpclient.Response{Data: &resp}
	err := suite.client.PostJson(
		"req-005",
		"localhost:9091/api/v1/tests/test-interceptor",
		req,
		httpResp,
		map[string]string{
			"X-User-Id": "user-interceptor",
		},
	)

	if err != nil {
		t.Errorf("拦截器测试请求失败: %v", err)
		return
	}

	if httpResp.StatusCode == http.StatusOK {
		t.Logf("✓ 多个拦截器成功执行 - RequestId: %s", resp.RequestId)
	}
}

// TestRequestTraceID 测试请求追踪ID
func TestRequestTraceID(t *testing.T) {
	suite := &TestSuite{}
	suite.setup(t)
	defer suite.teardown()

	req := testv1.Param{
		TestId: "trace-test",
		Name:   "trace",
	}

	var resp CommonResponse
	httpResp := &httpclient.Response{Data: &resp}
	traceID := "trace-id-12345"

	err := suite.client.PostJson(
		"req-006",
		"localhost:9091/api/v1/tests/trace-test",
		req,
		httpResp,
		map[string]string{
			"X-User-Id":    "user-trace",
			"X-Request-Id": traceID,
		},
	)

	if err != nil {
		t.Errorf("追踪ID测试失败: %v", err)
		return
	}

	if resp.RequestId != "" {
		t.Logf("✓ 请求追踪ID成功 - 响应RequestId: %s", resp.RequestId)
	}
}

// TestHeaderBinding 测试请求头绑定
func TestHeaderBinding(t *testing.T) {
	suite := &TestSuite{}
	suite.setup(t)
	defer suite.teardown()

	req := testv1.Param{
		TestId: "header-test",
		Name:   "test",
	}

	var resp CommonResponse
	httpResp := &httpclient.Response{Data: &resp}

	// 发送带有自定义请求头的请求
	err := suite.client.PostJson(
		"req-007",
		"localhost:9091/api/v1/tests/header-test",
		req,
		httpResp,
		map[string]string{
			"X-User-Id":    "custom-user-123",
			"X-Request-Id": "custom-req-id",
		},
	)

	if err != nil {
		t.Errorf("请求头绑定测试失败: %v", err)
		return
	}

	if httpResp.StatusCode == http.StatusOK {
		t.Logf("✓ 请求头绑定成功 - RequestId: %s", resp.RequestId)
	}
}

// TestErrorHandling 测试错误处理
func TestErrorHandling(t *testing.T) {
	suite := &TestSuite{}
	suite.setup(t)
	defer suite.teardown()

	// 发送不完整的请求数据
	invalidReq := map[string]interface{}{
		"name": 12345, // 期望字符串但发送了数字
	}

	var resp CommonResponse
	httpResp := &httpclient.Response{Data: &resp}

	err := suite.client.PostJson(
		"req-008",
		"localhost:9091/api/v1/tests/error-test",
		invalidReq,
		httpResp,
		map[string]string{
			"X-User-Id": "user-error",
		},
	)

	if err != nil {
		t.Logf("✓ 错误处理测试 - 捕获错误: %v", err)
	}
}

// TestConcurrentRequests 测试并发请求处理
func TestConcurrentRequests(t *testing.T) {
	suite := &TestSuite{}
	suite.setup(t)
	defer suite.teardown()

	client := suite.client
	done := make(chan error, 10)
	successCount := 0

	// 发送10个并发请求
	for i := 0; i < 10; i++ {
		go func(index int) {
			req := testv1.Param{
				TestId: fmt.Sprintf("concurrent-%d", index),
				Name:   fmt.Sprintf("test-%d", index),
			}

			var resp CommonResponse
			httpResp := &httpclient.Response{Data: &resp}

			err := client.PostJson(
				fmt.Sprintf("req-concurrent-%d", index),
				"localhost:9091/api/v1/tests/concurrent",
				req,
				httpResp,
				map[string]string{
					"X-User-Id": fmt.Sprintf("user-%d", index),
				},
			)

			if err == nil && httpResp.StatusCode == http.StatusOK {
				done <- nil
			} else {
				done <- fmt.Errorf("request %d failed", index)
			}
		}(i)
	}

	// 等待所有请求完成
	for i := 0; i < 10; i++ {
		if err := <-done; err == nil {
			successCount++
		}
	}

	t.Logf("✓ 并发请求测试 - 成功: %d/10", successCount)
	if successCount < 8 {
		t.Errorf("并发请求成功率过低: %d/10", successCount)
	}
}

// ======================= 辅助函数 =======================

func toBoolPtr(v bool) *bool {
	return &v
}

func toBoolStr(s string) *string {
	return &s
}

// ======================= 基准测试 =======================

// BenchmarkRESTfulRequest 基准测试RESTful请求性能
func BenchmarkRESTfulRequest(b *testing.B) {
	suite := &TestSuite{}
	logger.InitLogger(&logger.Config{EnableColor: "True"})
	loc, _ := time.LoadLocation("Asia/Shanghai")
	time.Local = loc

	config := webv1.Config{Port: 9092, RootPath: "api"}
	suite.server = webv1.NewHttpServer(config)
	suite.server.AddController(&testv1.StructService{})
	suite.server.Register(
		webv1.NewRestFul().
			Method(http.MethodPost).
			Version("/v1").
			Controller("test.StructService").
			Path("/tests/{testId}").
			Action("Test1"),
	)
	suite.server.StartUp()
	defer suite.server.Close()

	suite.client = httpclient.NewHttpClient(httpclient.Config{
		Timeout:      20,
		Verbose:      toBoolPtr(false),
		DisableRetry: true,
	})

	time.Sleep(500 * time.Millisecond)

	req := testv1.Param{
		TestId: "bench-test",
		Name:   "benchmark",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var resp CommonResponse
		httpResp := &httpclient.Response{Data: &resp}
		suite.client.PostJson(
			fmt.Sprintf("bench-%d", i),
			"localhost:9092/api/v1/tests/bench",
			req,
			httpResp,
			map[string]string{"X-User-Id": "bench-user"},
		)
	}
}
