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
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/caiflower/common-tools/pkg/bean"
	httpclient "github.com/caiflower/common-tools/pkg/http"
	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/caiflower/common-tools/web"
	"github.com/caiflower/common-tools/web/e"
	testv1 "github.com/caiflower/common-tools/web/test/controller/v1/test"
	"github.com/caiflower/common-tools/web/v1"
)

type TestInterceptor1 struct {
}

func (t *TestInterceptor1) Before(ctx *web.Context) (err e.ApiError) {
	fmt.Println("BeforeCallTargetMethod order:1")
	return
}

func (t *TestInterceptor1) After(ctx *web.Context, err e.ApiError) e.ApiError {
	fmt.Println("AfterCallTargetMethod order:1")
	if err != nil {
		fmt.Printf("AfterCallTargetMethod order:1 err: %v\n", err.GetMessage())
	}
	return nil
}

func (t *TestInterceptor1) OnPanic(ctx *web.Context, recover interface{}) (err e.ApiError) {
	fmt.Println("OnPanic order:1")
	return e.NewApiError(e.Unknown, "请稍后再试", nil)
}

type TestInterceptor2 struct {
}

func (t *TestInterceptor2) Before(ctx *web.Context) (err e.ApiError) {
	fmt.Println("BeforeCallTargetMethod order:2")
	return
}

func (t *TestInterceptor2) After(ctx *web.Context, err e.ApiError) e.ApiError {
	fmt.Println("AfterCallTargetMethod order:2")
	if err != nil {
		fmt.Printf("AfterCallTargetMethod order:2 err: %v\n", err.GetMessage())
	}
	return nil
}

func (t *TestInterceptor2) OnPanic(ctx *web.Context, recover interface{}) (err e.ApiError) {
	fmt.Println("OnPanic order:2")
	return
}

func initServer() *webv1.HttpServer {
	// 清理之前的Bean
	bean.ClearBeans()

	config := webv1.Config{
		Port:     9090,
		RootPath: "testhttp",
	}

	server := webv1.NewHttpServer(config)
	server.AddController(&testv1.StructService{})
	server.Register(webv1.NewRestFul().Method(http.MethodPost).Version("v1").Controller("test.StructService").Path("/tests/{testId}").Action("Test1"))
	server.Register(webv1.NewRestFul().Method(http.MethodGet).Version("v1").Controller("test.StructService").Path("/tests/{testId}").Action("Test1"))
	server.Register(webv1.NewRestFul().Method(http.MethodPost).Version("v1").Controller("test.StructService").Path("/tests/{testId}/test2s/{test2Id}").Action("Test2"))
	server.Register(webv1.NewRestFul().Method(http.MethodPost).Version("v1").Controller("test.StructService").Path("/tests/{testId}/test2s/{test2Id}/test").Action("Test2"))
	//server.AddInterceptor(&TestInterceptor2{}, 2)
	//server.AddInterceptor(&TestInterceptor1{}, 1)
	server.StartUp()

	return server
}

func TestHttpServer(t *testing.T) {
	logger.InitLogger(&logger.Config{
		EnableColor: "True",
	})
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		fmt.Println("加载时区出错:", err)
		return
	}
	// 设置time.Local为中国标准时间
	time.Local = loc

	server := initServer()
	defer server.Close()

	time.Sleep(500 * time.Millisecond) // 等待服务器启动

	// 初始化HTTP客户端
	client := httpclient.NewHttpClient(httpclient.Config{})

	// 测试用例列表
	tests := []struct {
		name   string
		testFn func(t *testing.T, client httpclient.HttpClient)
	}{
		{"RESTful POST Test1", testRestfulPost},
		{"RESTful GET Test1", testRestfulGet},
		{"RESTful POST Test2", testRestfulPostTest2},
		{"Error Response Test", testErrorResponse},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.testFn(t, client)
		})
	}
}

// 响应数据结构
type CommonResponse struct {
	RequestId string          `json:"requestId"`
	Data      json.RawMessage `json:"data"`
	Error     ErrorInfo       `json:"error"`
}

type ErrorInfo struct {
	Code    int    `json:"code"`
	Type    string `json:"type"`
	Message string `json:"message"`
}

func testRestfulPost(t *testing.T, client httpclient.HttpClient) {
	requestData := testv1.Param{
		TestId: "testId",
		Args:   "testArgs",
		Name:   "testName",
		Name1:  ptrStr("test"),
	}

	var respData CommonResponse
	resp := &httpclient.Response{Data: &respData}

	err := client.PostJson("test-001", "localhost:9090/testhttp/v1/tests/testId", requestData, resp, map[string]string{
		"X-User-Id": "user123",
	})

	if err != nil {
		t.Fatalf("请求失败: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("期望状态码 200, 实际 %d", resp.StatusCode)
	}

	if respData.RequestId == "" {
		t.Fatalf("响应请求ID为空")
	}

	t.Logf("✓ RESTful POST Test1 成功 - RequestId: %s", respData.RequestId)
}

func testRestfulGet(t *testing.T, client httpclient.HttpClient) {
	var respData CommonResponse
	resp := &httpclient.Response{Data: &respData}

	err := client.Get("test-002", "localhost:9090/testhttp/v1/tests/testId123", nil, resp, map[string]string{
		"X-User-Id": "user456",
	})

	if err != nil {
		t.Fatalf("GET请求失败: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("期望状态码 200, 实际 %d", resp.StatusCode)
	}

	t.Logf("✓ RESTful GET Test1 成功 - RequestId: %s", respData.RequestId)
}

func testRestfulPostTest2(t *testing.T, client httpclient.HttpClient) {
	requestData := testv1.Param{
		TestId: "testId",
	}

	var respData CommonResponse
	resp := &httpclient.Response{Data: &respData}

	err := client.PostJson(
		"test-003",
		"localhost:9090/testhttp/v1/tests/testId123/test2s/test2Id456",
		requestData,
		resp,
		map[string]string{"X-User-Id": "user789"},
	)

	if err != nil {
		t.Fatalf("POST Test2请求失败: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("期望状态码 200, 实际 %d", resp.StatusCode)
	}

	t.Logf("✓ RESTful POST Test2 成功 - RequestId: %s", respData.RequestId)
}

func testErrorResponse(t *testing.T, client httpclient.HttpClient) {
	// 发送不含必填字段的请求
	requestData := map[string]interface{}{
		"name": "test",
	}

	var respData CommonResponse
	resp := &httpclient.Response{Data: &respData}

	err := client.PostJson("test-004", "localhost:9090/testhttp/v1/tests/testId", requestData, resp, map[string]string{
		// 故意不发送X-User-Id以触发验证错误
	})

	if err != nil {
		t.Fatalf("请求失败: %v", err)
	}

	if resp.StatusCode != http.StatusBadRequest {
		t.Logf("验证错误响应 - 状态码: %d", resp.StatusCode)
	} else {
		t.Logf("✓ 错误响应验证成功 - 错误类型: %s, 信息: %s", respData.Error.Type, respData.Error.Message)
	}
}

func ptrStr(s string) *string {
	return &s
}

func TestHttpServerInteractive(t *testing.T) {
	logger.InitLogger(&logger.Config{
		EnableColor: "True",
	})
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		fmt.Println("加载时区出错:", err)
		return
	}
	time.Local = loc

	server := initServer()
	defer server.Close()

	time.Sleep(500 * time.Millisecond)

	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("HTTP Server 交互式测试开始")
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println("服务器运行于: http://localhost:9090")
	fmt.Println("\n可用的测试端点:")
	fmt.Println("  POST /testhttp/v1/tests/{testId}")
	fmt.Println("  GET  /testhttp/v1/tests/{testId}")
	fmt.Println("  POST /testhttp/v1/tests/{testId}/test2s/{test2Id}")
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println("\n测试示例命令:")
	fmt.Println(`curl -X POST http://localhost:9090/testhttp/v1/tests/123 \`)
	fmt.Println(`  -H "Content-Type: application/json" \`)
	fmt.Println(`  -H "X-User-Id: user123" \`)
	fmt.Println(`  -d '{"testId":"123","name":"test","args":"value"}'`)
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println("\n服务器将在30秒后自动关闭...")

	time.Sleep(30 * time.Second)
}
