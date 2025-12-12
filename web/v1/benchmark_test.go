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

package webv1

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/caiflower/common-tools/web"
)

// TestController 测试控制器
type TestController struct{}

// SimpleAction 简单动作
func (tc *TestController) SimpleAction(req SimpleRequest) (SimpleResponse, error) {
	return SimpleResponse{
		Message: fmt.Sprintf("Hello, %s!", req.Name),
		Time:    time.Now().Unix(),
	}, nil
}

// ComplexAction 复杂动作
func (tc *TestController) ComplexAction(req ComplexRequest) (ComplexResponse, error) {
	// 模拟一些计算
	result := make([]int, req.Count)
	for i := 0; i < req.Count; i++ {
		result[i] = i * i
	}

	return ComplexResponse{
		Data:   result,
		Status: "success",
		Time:   time.Now().Unix(),
	}, nil
}

// 请求和响应结构
type SimpleRequest struct {
	web.Context
	Name string `json:"name"`
}

type SimpleResponse struct {
	Message string `json:"message"`
	Time    int64  `json:"time"`
}

type ComplexRequest struct {
	web.Context
	Count int `json:"count"`
}

type ComplexResponse struct {
	Data   []int  `json:"data"`
	Status string `json:"status"`
	Time   int64  `json:"time"`
}

// BenchmarkStandardServer 标准服务器基准测试
func BenchmarkStandardServer(b *testing.B) {
	// 创建标准服务器
	config := NewConfigBuilder().
		WithName("benchmark-standard").
		WithPort(8081).
		WithStandardMode().
		Build()

	server := NewUnifiedWebServer(config)
	server.AddController(&TestController{})

	// 启动服务器
	go server.Start()
	time.Sleep(100 * time.Millisecond) // 等待服务器启动
	defer server.Close()

	// 运行基准测试
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = makeRequest("http://localhost:8081/TestController?Action=SimpleAction", SimpleRequest{Name: "World"})
		}
	})
}

// BenchmarkNetpollServer Netpoll 服务器基准测试
func BenchmarkNetpollServer(b *testing.B) {
	// 创建 Netpoll 服务器
	config := NewConfigBuilder().
		WithName("benchmark-netpoll").
		WithPort(8082).
		WithNetpollMode().
		WithMaxConnections(10000).
		WithBufferSize(8192).
		Build()

	server := NewUnifiedWebServer(config)
	server.AddController(&TestController{})

	// 启动服务器
	go server.Start()
	time.Sleep(100 * time.Millisecond) // 等待服务器启动
	defer server.Close()

	// 运行基准测试
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = makeRequest("http://localhost:8082/TestController?Action=SimpleAction", SimpleRequest{Name: "World"})
		}
	})
}

// BenchmarkComplexStandardServer 复杂请求标准服务器基准测试
func BenchmarkComplexStandardServer(b *testing.B) {
	config := NewConfigBuilder().
		WithName("benchmark-complex-standard").
		WithPort(8083).
		WithStandardMode().
		Build()

	server := NewUnifiedWebServer(config)
	server.AddController(&TestController{})

	go server.Start()
	time.Sleep(100 * time.Millisecond)
	defer server.Close()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			makeRequest("http://localhost:8083/TestController?Action=ComplexAction", ComplexRequest{Count: 1000})
		}
	})
}

// BenchmarkComplexNetpollServer 复杂请求 Netpoll 服务器基准测试
func BenchmarkComplexNetpollServer(b *testing.B) {
	config := NewConfigBuilder().
		WithName("benchmark-complex-netpoll").
		WithPort(8084).
		WithNetpollMode().
		WithMaxConnections(10000).
		WithBufferSize(8192).
		Build()

	server := NewUnifiedWebServer(config)
	server.AddController(&TestController{})

	go server.Start()
	time.Sleep(100 * time.Millisecond)
	defer server.Close()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = makeRequest("http://localhost:8084/TestController?Action=ComplexAction", ComplexRequest{Count: 1000})
		}
	})
}

// makeRequest 发送 HTTP 请求
func makeRequest(url string, payload interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	_, err = io.ReadAll(resp.Body)
	return err
}

// PerformanceTest 性能对比测试
func TestPerformanceComparison(t *testing.T) {
	// 测试参数
	testCases := []struct {
		name        string
		concurrency int
		requests    int
		payload     interface{}
	}{
		{"Simple-Low", 10, 1000, SimpleRequest{Name: "Test"}},
		{"Simple-Medium", 50, 5000, SimpleRequest{Name: "Test"}},
		{"Simple-High", 100, 10000, SimpleRequest{Name: "Test"}},
		{"Complex-Low", 10, 1000, ComplexRequest{Count: 500}},
		{"Complex-Medium", 50, 5000, ComplexRequest{Count: 500}},
		{"Complex-High", 1000, 50000, ComplexRequest{Count: 500}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// 测试标准服务器
			standardResult := runLoadTest(t, "standard", 8085, tc.concurrency, tc.requests, tc.payload)

			// 测试 Netpoll 服务器
			netpollResult := runLoadTest(t, "netpoll", 8086, tc.concurrency, tc.requests, tc.payload)

			// 输出对比结果
			t.Logf("Test Case: %s", tc.name)
			t.Logf("Standard Server - Avg: %.2fms, QPS: %.2f, Errors: %d",
				standardResult.AvgLatency, standardResult.QPS, standardResult.Errors)
			t.Logf("Netpoll Server  - Avg: %.2fms, QPS: %.2f, Errors: %d",
				netpollResult.AvgLatency, netpollResult.QPS, netpollResult.Errors)

			improvement := (standardResult.AvgLatency - netpollResult.AvgLatency) / standardResult.AvgLatency * 100
			qpsImprovement := (netpollResult.QPS - standardResult.QPS) / standardResult.QPS * 100

			t.Logf("Performance Improvement - Latency: %.1f%%, QPS: %.1f%%", improvement, qpsImprovement)
		})
	}
}

// LoadTestResult 负载测试结果
type LoadTestResult struct {
	AvgLatency float64 // 平均延迟（毫秒）
	QPS        float64 // 每秒请求数
	Errors     int     // 错误数量
	Duration   time.Duration
}

// runLoadTest 运行负载测试
func runLoadTest(t *testing.T, serverType string, port int, concurrency, requests int, payload interface{}) LoadTestResult {
	// 创建服务器
	var server *UnifiedWebServer
	if serverType == "netpoll" {
		config := NewConfigBuilder().
			WithName(fmt.Sprintf("loadtest-%s", serverType)).
			WithPort(uint(port)).
			WithNetpollMode().
			WithMaxConnections(10000).
			Build()
		server = NewUnifiedWebServer(config)
	} else {
		config := NewConfigBuilder().
			WithName(fmt.Sprintf("loadtest-%s", serverType)).
			WithPort(uint(port)).
			WithStandardMode().
			Build()
		server = NewUnifiedWebServer(config)
	}

	server.AddController(&TestController{})

	// 启动服务器
	go server.Start()
	time.Sleep(200 * time.Millisecond) // 等待服务器启动
	defer server.Close()

	// 确定请求 URL
	var url string
	switch payload.(type) {
	case SimpleRequest:
		url = fmt.Sprintf("http://localhost:%d/TestController?Action=SimpleAction", port)
	case ComplexRequest:
		url = fmt.Sprintf("http://localhost:%d/TestController?Action=ComplexAction", port)
	}

	// 执行负载测试
	var wg sync.WaitGroup
	results := make(chan time.Duration, requests)
	errors := make(chan error, requests)

	startTime := time.Now()

	// 启动并发请求
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			requestsPerWorker := requests / concurrency

			for j := 0; j < requestsPerWorker; j++ {
				reqStart := time.Now()
				err := makeRequest(url, payload)
				reqDuration := time.Since(reqStart)

				if err != nil {
					errors <- err
				} else {
					results <- reqDuration
				}
			}
		}()
	}

	wg.Wait()
	totalDuration := time.Since(startTime)
	close(results)
	close(errors)

	// 计算统计信息
	var totalLatency time.Duration
	successCount := 0
	errorCount := 0

	for latency := range results {
		totalLatency += latency
		successCount++
	}

	for range errors {
		errorCount++
	}

	avgLatency := float64(totalLatency.Nanoseconds()) / float64(successCount) / 1e6 // 转换为毫秒
	qps := float64(successCount) / totalDuration.Seconds()

	return LoadTestResult{
		AvgLatency: avgLatency,
		QPS:        qps,
		Errors:     errorCount,
		Duration:   totalDuration,
	}
}

// BenchmarkMemoryUsage 内存使用基准测试
func BenchmarkMemoryUsage(b *testing.B) {
	b.Run("Standard", func(b *testing.B) {
		config := NewConfigBuilder().
			WithName("memory-standard").
			WithPort(8087).
			WithStandardMode().
			Build()

		server := NewUnifiedWebServer(config)
		server.AddController(&TestController{})

		go server.Start()
		time.Sleep(100 * time.Millisecond)
		defer server.Close()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = makeRequest("http://localhost:8087/TestController?Action=SimpleAction", SimpleRequest{Name: "Test"})
		}
	})

	b.Run("Netpoll", func(b *testing.B) {
		config := NewConfigBuilder().
			WithName("memory-netpoll").
			WithPort(8088).
			WithNetpollMode().
			WithMaxConnections(10000).
			Build()

		server := NewUnifiedWebServer(config)
		server.AddController(&TestController{})

		go server.Start()
		time.Sleep(100 * time.Millisecond)
		defer server.Close()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = makeRequest("http://localhost:8088/TestController?Action=SimpleAction", SimpleRequest{Name: "Test"})
		}
	})
}
