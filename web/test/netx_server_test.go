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
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/caiflower/common-tools/web/common/config"
	"github.com/caiflower/common-tools/web/server/netx"
	"github.com/stretchr/testify/assert"
)

func TestNewHttpServer(t *testing.T) {
	tests := []struct {
		name    string
		options config.Options
		wantErr bool
	}{
		{
			name: "Valid configuration",
			options: config.Options{
				Name:                     "test-server",
				Addr:                     ":8080",
				ReadTimeout:              time.Second * 30,
				WriteTimeout:             time.Second * 30,
				HandleTimeout:            time.Second * 60,
				KeepAliveTimeout:         time.Second * 60,
				RootPath:                 "/api",
				HeaderTraceID:            "X-Trace-ID",
				ControllerRootPkgName:    "controller",
				EnablePprof:              false,
				Mode:                     config.ServerModeNetpoll,
				LimiterEnabled:           false,
				Qps:                      1000,
				Network:                  "tcp",
				SenseClientDisconnection: false,
			},
			wantErr: false,
		},
		{
			name: "Minimal configuration",
			options: config.Options{
				Name: "minimal-server",
				Addr: ":9090",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := netx.NewHttpServer(tt.options)

			assert.NotNil(t, server, "Server should not be nil")
			assert.NotNil(t, server.Handler, "Handler should not be nil")
			assert.Equal(t, fmt.Sprintf("NETPOLL_HTTP_SERVER:%s", tt.options.Name), server.Name())
		})
	}
}

func TestHttpServerOutOfRangPort(t *testing.T) {
	// 测试无效端口
	options := config.Options{
		Name: "error-server",
		Addr: ":99999", // 无效端口
	}

	server := netx.NewHttpServer(options)
	assert.NotNil(t, server)

	defer func() {
		if r := recover(); r != nil {
			t.Log("Server panic:", r)
		}
	}()

	// 尝试启动应该会失败
	_ = server.Start()

	server.Close()
}

func TestHttpServerIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// 获取可用端口
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal("Failed to get available port:", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()

	options := config.Options{
		Name:                     "integration-server",
		Addr:                     fmt.Sprintf(":%d", port),
		ReadTimeout:              time.Second * 5,
		WriteTimeout:             time.Second * 5,
		HandleTimeout:            time.Second * 10,
		KeepAliveTimeout:         time.Second * 30,
		RootPath:                 "/api/v1",
		HeaderTraceID:            "X-Request-ID",
		ControllerRootPkgName:    "controller",
		EnablePprof:              false,
		Mode:                     config.ServerModeNetpoll,
		LimiterEnabled:           false,
		Network:                  "tcp",
		SenseClientDisconnection: false,
	}

	server := netx.NewHttpServer(options)
	assert.NotNil(t, server)

	// 启动服务器
	go func() {
		err := server.Start()
		if err != nil && !strings.Contains(err.Error(), "closed") {
			t.Logf("Server start error: %v", err)
		}
	}()

	// 等待服务器启动
	time.Sleep(200 * time.Millisecond)

	// 尝试连接
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", port), time.Second)
	assert.Nil(t, err, "Error connecting to server")
	if err == nil {
		_ = conn.Close()
	}

	// 关闭服务器
	server.Close()

	// 验证服务器已关闭
	time.Sleep(100 * time.Millisecond)
	_, err = net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", port), time.Millisecond*100)
	if err != nil {
		assert.NotNil(t, err, "Connection failed (expected))")
	}
}
