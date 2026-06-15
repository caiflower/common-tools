package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// ===== 测试用类型定义 =====

type testInput struct {
	Name string `json:"name"`
	Age  int    `json:"age"`
}

type testOutput struct {
	Greeting string `json:"greeting"`
	Length   int    `json:"length"`
}

type simpleInput struct {
	Value int `json:"value"`
}

type simpleOutput struct {
	Result int `json:"result"`
}

// ===== LocalExecutor 泛型单元测试 =====

func TestLocalExecutor_TypeSafeExecution(t *testing.T) {
	exec := NewLocalExecutor(func(ctx context.Context, input testInput) (testOutput, error) {
		return testOutput{
			Greeting: "Hello, " + input.Name,
			Length:   len(input.Name),
		}, nil
	})

	inputJSON, _ := json.Marshal(testInput{Name: "World", Age: 25})
	data := &TaskData{Input: string(inputJSON)}

	result, err := exec.Execute(context.Background(), data)
	assert.Nil(t, err)

	output, ok := result.(testOutput)
	assert.True(t, ok)
	assert.Equal(t, "Hello, World", output.Greeting)
	assert.Equal(t, 5, output.Length)
}

func TestLocalExecutor_Protocol(t *testing.T) {
	exec := NewLocalExecutor(func(ctx context.Context, input string) (string, error) {
		return input, nil
	})
	assert.Equal(t, ProtocolLocal, exec.Protocol())
}

func TestLocalExecutor_ErrorHandling(t *testing.T) {
	expectedErr := fmt.Errorf("business error")
	exec := NewLocalExecutor(func(ctx context.Context, input testInput) (testOutput, error) {
		return testOutput{}, expectedErr
	})

	inputJSON, _ := json.Marshal(testInput{Name: "test"})
	data := &TaskData{Input: string(inputJSON)}

	_, err := exec.Execute(context.Background(), data)
	assert.Equal(t, expectedErr, err)
}

func TestLocalExecutor_UnmarshalInputFailed(t *testing.T) {
	exec := NewLocalExecutor(func(ctx context.Context, input testInput) (testOutput, error) {
		return testOutput{Greeting: input.Name}, nil
	})

	// 传入无效 JSON
	data := &TaskData{Input: "invalid-json"}
	_, err := exec.Execute(context.Background(), data)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "unmarshal input failed")
}

func TestLocalExecutor_EmptyInput(t *testing.T) {
	exec := NewLocalExecutor(func(ctx context.Context, input testInput) (testOutput, error) {
		return testOutput{Greeting: "empty"}, nil
	})

	// 空输入应返回零值 input
	data := &TaskData{Input: ""}
	result, err := exec.Execute(context.Background(), data)
	assert.Nil(t, err)
	output, ok := result.(testOutput)
	assert.True(t, ok)
	assert.Equal(t, "empty", output.Greeting)
}

func TestLocalExecutor_SimpleTypes(t *testing.T) {
	// 测试简单类型泛型
	exec := NewLocalExecutor(func(ctx context.Context, input simpleInput) (simpleOutput, error) {
		return simpleOutput{Result: input.Value * 2}, nil
	})

	inputJSON, _ := json.Marshal(simpleInput{Value: 21})
	data := &TaskData{Input: string(inputJSON)}

	result, err := exec.Execute(context.Background(), data)
	assert.Nil(t, err)
	output, ok := result.(simpleOutput)
	assert.True(t, ok)
	assert.Equal(t, 42, output.Result)
}

func TestLocalExecutor_ContextPropagation(t *testing.T) {
	exec := NewLocalExecutor(func(ctx context.Context, input simpleInput) (simpleOutput, error) {
		// 验证 context 可以传递
		deadline, ok := ctx.Deadline()
		assert.True(t, ok)
		assert.False(t, deadline.IsZero())
		return simpleOutput{Result: input.Value}, nil
	})

	inputJSON, _ := json.Marshal(simpleInput{Value: 1})
	data := &TaskData{Input: string(inputJSON)}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := exec.Execute(ctx, data)
	assert.Nil(t, err)
}

// ===== LocalExecutor 结构体字面量初始化测试 =====

func TestLocalExecutor_StructLiteral_Execute(t *testing.T) {
	exec := &LocalExecutor[string, string]{
		fn: func(ctx context.Context, input string) (string, error) {
			return "local-result", nil
		},
	}

	inputJSON, _ := json.Marshal("test")
	result, err := exec.Execute(context.Background(), &TaskData{Input: string(inputJSON)})
	assert.Nil(t, err)
	assert.Equal(t, "local-result", result)
}

func TestLocalExecutor_StructLiteral_Protocol(t *testing.T) {
	exec := &LocalExecutor[string, int]{
		fn: func(ctx context.Context, input string) (int, error) {
			return len(input), nil
		},
	}
	assert.Equal(t, ProtocolLocal, exec.Protocol())
}

func TestLocalExecutor_StructLiteral_Error(t *testing.T) {
	expectedErr := fmt.Errorf("local error")
	exec := &LocalExecutor[string, string]{
		fn: func(ctx context.Context, input string) (string, error) {
			return "", expectedErr
		},
	}

	inputJSON, _ := json.Marshal("test")
	_, err := exec.Execute(context.Background(), &TaskData{Input: string(inputJSON)})
	assert.Equal(t, expectedErr, err)
}

// ===== HTTPExecutor 泛型集成测试 =====

func TestHTTPExecutor_PostRequest(t *testing.T) {
	// 创建 mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var input simpleInput
		err := json.NewDecoder(r.Body).Decode(&input)
		assert.Nil(t, err)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(simpleOutput{Result: input.Value * 3})
	}))
	defer server.Close()

	exec := NewHTTPExecutor[simpleInput, simpleOutput](server.URL, "POST")

	inputJSON, _ := json.Marshal(simpleInput{Value: 7})
	data := &TaskData{Input: string(inputJSON)}

	result, err := exec.Execute(context.Background(), data)
	assert.Nil(t, err)

	output, ok := result.(simpleOutput)
	assert.True(t, ok)
	assert.Equal(t, 21, output.Result)
}

func TestHTTPExecutor_GetRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(simpleOutput{Result: 100})
	}))
	defer server.Close()

	exec := NewHTTPExecutor[simpleInput, simpleOutput](server.URL, "GET")

	data := &TaskData{Input: "{}"}
	result, err := exec.Execute(context.Background(), data)
	assert.Nil(t, err)

	output, ok := result.(simpleOutput)
	assert.True(t, ok)
	assert.Equal(t, 100, output.Result)
}

func TestHTTPExecutor_Protocol(t *testing.T) {
	exec := NewHTTPExecutor[simpleInput, simpleOutput]("http://localhost", "POST")
	assert.Equal(t, ProtocolHTTP, exec.Protocol())
}

func TestHTTPExecutor_CustomHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(simpleOutput{Result: 1})
	}))
	defer server.Close()

	exec := NewHTTPExecutor[simpleInput, simpleOutput](server.URL, "POST",
		WithHTTPHeaders(map[string]string{"Authorization": "Bearer test-token"}),
	)

	data := &TaskData{Input: "{}"}
	result, err := exec.Execute(context.Background(), data)
	assert.Nil(t, err)

	output, ok := result.(simpleOutput)
	assert.True(t, ok)
	assert.Equal(t, 1, output.Result)
}

func TestHTTPExecutor_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	exec := NewHTTPExecutor[simpleInput, simpleOutput](server.URL, "POST",
		WithHTTPTimeout(100*time.Millisecond),
	)

	data := &TaskData{Input: "{}"}
	_, err := exec.Execute(context.Background(), data)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "request failed")
}

func TestHTTPExecutor_NonSuccessStatusCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}))
	defer server.Close()

	exec := NewHTTPExecutor[simpleInput, simpleOutput](server.URL, "POST")

	data := &TaskData{Input: "{}"}
	_, err := exec.Execute(context.Background(), data)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "status 500")
}

func TestHTTPExecutor_ConnectionRefused(t *testing.T) {
	exec := NewHTTPExecutor[simpleInput, simpleOutput](
		"http://localhost:1", // 不存在的端口
		"POST",
		WithHTTPTimeout(2*time.Second),
	)

	data := &TaskData{Input: "{}"}
	_, err := exec.Execute(context.Background(), data)
	assert.NotNil(t, err)
}

func TestHTTPExecutor_UnmarshalInputFailed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	exec := NewHTTPExecutor[simpleInput, simpleOutput](server.URL, "POST")
	data := &TaskData{Input: "not-json"}
	_, err := exec.Execute(context.Background(), data)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "unmarshal input failed")
}

// ===== MCPExecutor 集成测试 =====

func TestMCPExecutor_ToolCall(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)

		var req mcpRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		assert.Nil(t, err)
		assert.Equal(t, "2.0", req.JSONRPC)
		assert.Equal(t, "tools/call", req.Method)
		assert.Equal(t, "my-tool", req.Params.Name)

		w.Header().Set("Content-Type", "application/json")
		resp := map[string]any{
			"jsonrpc": "2.0",
			"id":      1,
			"result": map[string]any{
				"content": []map[string]any{
					{"type": "text", "text": `{"answer": 42}`},
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	exec := NewMCPExecutor(server.URL, "my-tool")

	inputJSON, _ := json.Marshal(map[string]any{"query": "test"})
	data := &TaskData{Input: string(inputJSON)}

	result, err := exec.Execute(context.Background(), data)
	assert.Nil(t, err)
	assert.NotNil(t, result)

	// 验证结果包含解析后的内容
	resultMap, ok := result.(map[string]any)
	assert.True(t, ok)
	assert.Contains(t, resultMap, "content_0")
}

func TestMCPExecutor_Protocol(t *testing.T) {
	exec := NewMCPExecutor("http://localhost:8080", "tool")
	assert.Equal(t, ProtocolMCP, exec.Protocol())
}

func TestMCPExecutor_InvalidInput(t *testing.T) {
	exec := NewMCPExecutor("http://localhost:8080", "tool")
	data := &TaskData{Input: "not-json"}
	_, err := exec.Execute(context.Background(), data)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "parse input failed")
}

func TestMCPExecutor_ConnectionError(t *testing.T) {
	exec := NewMCPExecutor("http://localhost:1", "tool",
		WithMCPTimeout(2*time.Second),
	)

	data := &TaskData{Input: "{}"}
	_, err := exec.Execute(context.Background(), data)
	assert.NotNil(t, err)
}

func TestMCPExecutor_EmptyInput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req mcpRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		assert.Equal(t, "my-tool", req.Params.Name)
		assert.Nil(t, req.Params.Arguments) // 空输入

		w.Header().Set("Content-Type", "application/json")
		resp := map[string]any{
			"jsonrpc": "2.0",
			"id":      1,
			"result": map[string]any{
				"content": []map[string]any{
					{"type": "text", "text": "ok"},
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	exec := NewMCPExecutor(server.URL, "my-tool")
	data := &TaskData{Input: ""}

	result, err := exec.Execute(context.Background(), data)
	assert.Nil(t, err)
	assert.NotNil(t, result)
}

// ===== ExecutorProtocol 测试 =====

func TestExecutorProtocol_Values(t *testing.T) {
	assert.Equal(t, ExecutorProtocol("local"), ProtocolLocal)
	assert.Equal(t, ExecutorProtocol("grpc"), ProtocolGRPC)
	assert.Equal(t, ExecutorProtocol("http"), ProtocolHTTP)
	assert.Equal(t, ExecutorProtocol("mcp"), ProtocolMCP)
}

// ===== TaskData 测试 =====

func TestTaskData_UnmarshalInput(t *testing.T) {
	inputJSON, _ := json.Marshal(testInput{Name: "Alice", Age: 30})
	data := &TaskData{Input: string(inputJSON)}

	var input testInput
	err := data.UnmarshalInput(&input)
	assert.Nil(t, err)
	assert.Equal(t, "Alice", input.Name)
	assert.Equal(t, 30, input.Age)
}

func TestTaskData_UnmarshalInput_Empty(t *testing.T) {
	data := &TaskData{Input: ""}
	var input testInput
	err := data.UnmarshalInput(&input)
	assert.Nil(t, err)
	// 零值
	assert.Equal(t, "", input.Name)
	assert.Equal(t, 0, input.Age)
}

func TestTaskData_UnmarshalInput_InvalidJSON(t *testing.T) {
	data := &TaskData{Input: "not-valid-json"}
	var input testInput
	err := data.UnmarshalInput(&input)
	assert.NotNil(t, err)
}

// ===== 序列化约束测试 =====

func TestSerialization_RoundTrip(t *testing.T) {
	original := testInput{Name: "Bob", Age: 25}

	// 序列化
	bytes, err := json.Marshal(original)
	assert.Nil(t, err)

	// 反序列化
	var restored testInput
	err = json.Unmarshal(bytes, &restored)
	assert.Nil(t, err)
	assert.Equal(t, original, restored)
}

func TestSerialization_LocalExecutorRoundTrip(t *testing.T) {
	// 模拟集群传输场景：序列化 → 传输 → 反序列化 → 执行
	exec := NewLocalExecutor(func(ctx context.Context, input testInput) (testOutput, error) {
		return testOutput{
			Greeting: "Hi, " + input.Name,
			Length:   len(input.Name),
		}, nil
	})

	// 模拟序列化输入
	original := testInput{Name: "Cluster", Age: 1}
	inputBytes, _ := json.Marshal(original)

	// 模拟传输后的 TaskData
	data := &TaskData{Input: string(inputBytes)}

	// 执行
	result, err := exec.Execute(context.Background(), data)
	assert.Nil(t, err)

	output := result.(testOutput)
	assert.Equal(t, "Hi, Cluster", output.Greeting)
	assert.Equal(t, 7, output.Length)

	// 模拟序列化输出
	outputBytes, err := json.Marshal(output)
	assert.Nil(t, err)

	// 模拟反序列化输出
	var restored testOutput
	err = json.Unmarshal(outputBytes, &restored)
	assert.Nil(t, err)
	assert.Equal(t, output, restored)
}

// ===== 混合协议注册测试 =====

func TestMixedProviderTypes(t *testing.T) {
	// 创建不同协议的执行器
	localExec := NewLocalExecutor(func(ctx context.Context, input simpleInput) (simpleOutput, error) {
		return simpleOutput{Result: input.Value * 2}, nil
	})

	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var input simpleInput
		_ = json.NewDecoder(r.Body).Decode(&input)
		json.NewEncoder(w).Encode(simpleOutput{Result: input.Value * 3})
	}))
	defer httpServer.Close()

	httpExec := NewHTTPExecutor[simpleInput, simpleOutput](httpServer.URL, "POST")

	mcpExec := NewMCPExecutor("http://localhost:1", "tool")

	// 验证各执行器返回正确的协议
	assert.Equal(t, ProtocolLocal, localExec.Protocol())
	assert.Equal(t, ProtocolHTTP, httpExec.Protocol())
	assert.Equal(t, ProtocolMCP, mcpExec.Protocol())
}

// ===== 自定义 ExecutorProvider 扩展测试 =====

type customExecutor struct {
	customValue string
}

func (e *customExecutor) Execute(ctx context.Context, data *TaskData) (any, error) {
	var input simpleInput
	if err := data.UnmarshalInput(&input); err != nil {
		return nil, err
	}
	return simpleOutput{Result: input.Value + len(e.customValue)}, nil
}

func (e *customExecutor) Protocol() ExecutorProtocol {
	return ExecutorProtocol("custom")
}

func TestCustomExecutor(t *testing.T) {
	exec := &customExecutor{customValue: "hello"}

	inputJSON, _ := json.Marshal(simpleInput{Value: 10})
	data := &TaskData{Input: string(inputJSON)}

	result, err := exec.Execute(context.Background(), data)
	assert.Nil(t, err)

	output, ok := result.(simpleOutput)
	assert.True(t, ok)
	assert.Equal(t, 15, output.Result) // 10 + len("hello")
	assert.Equal(t, ExecutorProtocol("custom"), exec.Protocol())
}

// ===== Functional Options 测试 =====

func TestHTTPOptions_Defaults(t *testing.T) {
	exec := NewHTTPExecutor[simpleInput, simpleOutput]("http://localhost", "POST")
	assert.Equal(t, 30*time.Second, exec.timeout)
	assert.Equal(t, "application/json", exec.headers["Content-Type"])
}

func TestHTTPOptions_CustomTimeout(t *testing.T) {
	exec := NewHTTPExecutor[simpleInput, simpleOutput]("http://localhost", "POST",
		WithHTTPTimeout(5*time.Second),
	)
	assert.Equal(t, 5*time.Second, exec.timeout)
}

func TestHTTPOptions_CustomHeaders(t *testing.T) {
	exec := NewHTTPExecutor[simpleInput, simpleOutput]("http://localhost", "POST",
		WithHTTPHeaders(map[string]string{
			"Authorization": "Bearer token",
			"X-Custom":      "value",
		}),
	)
	assert.Equal(t, "Bearer token", exec.headers["Authorization"])
	assert.Equal(t, "value", exec.headers["X-Custom"])
}

func TestMCPOptions_Defaults(t *testing.T) {
	exec := NewMCPExecutor("http://localhost:8080", "tool")
	assert.Equal(t, 30*time.Second, exec.timeout)
	assert.Equal(t, "application/json", exec.headers["Content-Type"])
}

func TestMCPOptions_CustomTimeout(t *testing.T) {
	exec := NewMCPExecutor("http://localhost:8080", "tool",
		WithMCPTimeout(10*time.Second),
	)
	assert.Equal(t, 10*time.Second, exec.timeout)
}

// ===== 并发安全测试 =====

func TestLocalExecutor_ConcurrentExecution(t *testing.T) {
	exec := NewLocalExecutor(func(ctx context.Context, input simpleInput) (simpleOutput, error) {
		time.Sleep(10 * time.Millisecond) // 模拟耗时
		return simpleOutput{Result: input.Value * 2}, nil
	})

	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(val int) {
			inputJSON, _ := json.Marshal(simpleInput{Value: val})
			data := &TaskData{Input: string(inputJSON)}
			result, err := exec.Execute(context.Background(), data)
			assert.Nil(t, err)
			output := result.(simpleOutput)
			assert.Equal(t, val*2, output.Result)
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

// ===== 完整 DAG + ExecutorProvider 集成测试 =====

func TestLocalExecutor_IntegrationWithDAG(t *testing.T) {
	// 模拟 DAG 执行流程中使用 ExecutorProvider
	step1Exec := NewLocalExecutor(func(ctx context.Context, input simpleInput) (simpleOutput, error) {
		return simpleOutput{Result: input.Value + 1}, nil
	})

	step2Exec := NewLocalExecutor(func(ctx context.Context, input simpleOutput) (simpleOutput, error) {
		return simpleOutput{Result: input.Result * 2}, nil
	})

	// step1: input=10 → output.Result=11
	inputJSON, _ := json.Marshal(simpleInput{Value: 10})
	data := &TaskData{Input: string(inputJSON)}
	result1, err := step1Exec.Execute(context.Background(), data)
	assert.Nil(t, err)

	output1 := result1.(simpleOutput)
	assert.Equal(t, 11, output1.Result)

	// step2: input=11 → output.Result=22
	input2JSON, _ := json.Marshal(output1)
	data2 := &TaskData{Input: string(input2JSON)}
	result2, err := step2Exec.Execute(context.Background(), data2)
	assert.Nil(t, err)

	output2 := result2.(simpleOutput)
	assert.Equal(t, 22, output2.Result)
}

func TestHTTPExecutor_IntegrationWithDAG(t *testing.T) {
	// 模拟 DAG 中 HTTP 执行器与其他执行器协作
	step1Exec := NewLocalExecutor(func(ctx context.Context, input simpleInput) (simpleOutput, error) {
		return simpleOutput{Result: input.Value + 1}, nil
	})

	// HTTP 服务模拟 step2
	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var input simpleOutput
		_ = json.NewDecoder(r.Body).Decode(&input)
		json.NewEncoder(w).Encode(simpleOutput{Result: input.Result * 10})
	}))
	defer httpServer.Close()

	step2Exec := NewHTTPExecutor[simpleOutput, simpleOutput](httpServer.URL, "POST")

	// step1: input=5 → output.Result=6
	inputJSON, _ := json.Marshal(simpleInput{Value: 5})
	data := &TaskData{Input: string(inputJSON)}
	result1, err := step1Exec.Execute(context.Background(), data)
	assert.Nil(t, err)

	output1 := result1.(simpleOutput)
	assert.Equal(t, 6, output1.Result)

	// step2 (HTTP): input=6 → output.Result=60
	input2JSON, _ := json.Marshal(output1)
	data2 := &TaskData{Input: string(input2JSON)}
	result2, err := step2Exec.Execute(context.Background(), data2)
	assert.Nil(t, err)

	output2 := result2.(simpleOutput)
	assert.Equal(t, 60, output2.Result)
}

// ===== GRPCExecutor 测试 =====

func TestGRPCExecutor_Protocol(t *testing.T) {
	exec := NewGRPCExecutor[simpleInput, simpleOutput]("localhost:50051", "TestService", "TestMethod")
	assert.Equal(t, ProtocolGRPC, exec.Protocol())
}

func TestGRPCOptions_Defaults(t *testing.T) {
	exec := NewGRPCExecutor[simpleInput, simpleOutput]("localhost:50051", "TestService", "TestMethod")
	assert.Equal(t, 30*time.Second, exec.timeout)
	assert.Equal(t, 1, len(exec.dialOpts)) // 默认有 TransportCredentials
}

func TestGRPCOptions_CustomTimeout(t *testing.T) {
	exec := NewGRPCExecutor[simpleInput, simpleOutput]("localhost:50051", "TestService", "TestMethod",
		WithGRPCTimeout(10*time.Second),
	)
	assert.Equal(t, 10*time.Second, exec.timeout)
}

func TestGRPCOptions_CustomDialOptions(t *testing.T) {
	exec := NewGRPCExecutor[simpleInput, simpleOutput]("localhost:50051", "TestService", "TestMethod",
		WithGRPCDialOptions(grpc.WithTransportCredentials(insecure.NewCredentials())),
	)
	// 默认1个 + 自定义1个
	assert.Equal(t, 2, len(exec.dialOpts))
}

func TestGRPCExecutor_UnmarshalInputFailed(t *testing.T) {
	exec := NewGRPCExecutor[simpleInput, simpleOutput]("localhost:50051", "TestService", "TestMethod")
	data := &TaskData{Input: "not-json"}
	_, err := exec.Execute(context.Background(), data)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "unmarshal input failed")
}

func TestGRPCExecutor_ConnectionFailed(t *testing.T) {
	// 使用无效地址，连接建立失败（通过 initConn 懒加载）
	exec := NewGRPCExecutor[simpleInput, simpleOutput]("invalid://host", "TestService", "TestMethod",
		WithGRPCTimeout(2*time.Second),
	)
	// NewClient 本身不会立即连接，但 Invoke 时会失败
	// 这里主要验证 initConn 能正常返回
	data := &TaskData{Input: `{"value": 1}`}
	_, err := exec.Execute(context.Background(), data)
	assert.NotNil(t, err)
}

func TestGRPCExecutor_Close(t *testing.T) {
	exec := NewGRPCExecutor[simpleInput, simpleOutput]("localhost:50051", "TestService", "TestMethod")
	// 未初始化连接时 Close 应返回 nil
	assert.Nil(t, exec.Close())
}

func TestGRPCExecutor_CloseAfterInit(t *testing.T) {
	exec := NewGRPCExecutor[simpleInput, simpleOutput]("localhost:50051", "TestService", "TestMethod")
	// 手动初始化连接
	err := exec.initConn()
	assert.Nil(t, err)
	// 关闭连接
	assert.Nil(t, exec.Close())
}

func TestGRPCExecutor_ConcurrentInitConn(t *testing.T) {
	exec := NewGRPCExecutor[simpleInput, simpleOutput]("localhost:50051", "TestService", "TestMethod")

	var wg sync.WaitGroup
	errCount := int64(0)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := exec.initConn()
			if err != nil {
				errCount++
			}
		}()
	}
	wg.Wait()

	// 所有并发调用都应成功（sync.Once 保护）
	assert.Equal(t, int64(0), errCount)
	assert.NotNil(t, exec.conn)
	// 清理
	exec.Close()
}

func TestGRPCExecutor_JsonCodec(t *testing.T) {
	codec := jsonCodec{}
	assert.Equal(t, "json", codec.Name())

	// 测试 Marshal/Unmarshal
	data := simpleInput{Value: 42}
	bytes, err := codec.Marshal(data)
	assert.Nil(t, err)
	assert.Contains(t, string(bytes), "42")

	var restored simpleInput
	err = codec.Unmarshal(bytes, &restored)
	assert.Nil(t, err)
	assert.Equal(t, 42, restored.Value)
}

// ===== TypedProvider 接口测试 =====

func TestLocalExecutor_ImplementsTypedProvider(t *testing.T) {
	exec := NewLocalExecutor(func(ctx context.Context, input testInput) (testOutput, error) {
		return testOutput{Greeting: "Hello, " + input.Name}, nil
	})

	// 验证 LocalExecutor 实现了 TypedProvider 接口
	var _ TypedProvider = exec

	assert.Equal(t, "executor.testInput", exec.InputType().String())
	assert.Equal(t, "executor.testOutput", exec.OutputType().String())
}

func TestLocalExecutor_TypedProviderMapTypes(t *testing.T) {
	exec := NewLocalExecutor(func(ctx context.Context, input map[string]any) (map[string]any, error) {
		return input, nil
	})

	var _ TypedProvider = exec

	assert.Equal(t, "map[string]interface {}", exec.InputType().String())
	assert.Equal(t, "map[string]interface {}", exec.OutputType().String())
}

func TestLocalExecutor_TypedProviderStringTypes(t *testing.T) {
	exec := NewLocalExecutor(func(ctx context.Context, input string) (string, error) {
		return input, nil
	})

	var _ TypedProvider = exec

	assert.Equal(t, "string", exec.InputType().String())
	assert.Equal(t, "string", exec.OutputType().String())
}

func TestTypedProvider_InterfaceAssertion(t *testing.T) {
	// LocalExecutor 可以通过接口断言获取类型信息
	var provider ExecutorProvider = NewLocalExecutor(func(ctx context.Context, input testInput) (testOutput, error) {
		return testOutput{}, nil
	})

	tp, ok := provider.(TypedProvider)
	assert.True(t, ok, "LocalExecutor should implement TypedProvider")
	assert.NotNil(t, tp.InputType())
	assert.NotNil(t, tp.OutputType())
}

func TestTypedProvider_CustomExecutorNotImplemented(t *testing.T) {
	// 自定义执行器不实现 TypedProvider，断言应失败
	var provider ExecutorProvider = &customExecutor{customValue: "test"}

	_, ok := provider.(TypedProvider)
	assert.False(t, ok, "customExecutor should not implement TypedProvider")
}
