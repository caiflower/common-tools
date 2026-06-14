package executor

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"sync/atomic"
	"time"
)

// mcpRequestID MCP 请求的全局原子递增 ID
var mcpRequestID atomic.Int64

// MCPOption MCP 执行器配置选项
type MCPOption func(*MCPExecutor)

// WithMCPTimeout 设置超时时间
func WithMCPTimeout(timeout time.Duration) MCPOption {
	return func(e *MCPExecutor) {
		e.timeout = timeout
	}
}

// WithMCPHeaders 设置请求头
func WithMCPHeaders(headers map[string]string) MCPOption {
	return func(e *MCPExecutor) {
		if e.headers == nil {
			e.headers = make(map[string]string)
		}
		for k, v := range headers {
			e.headers[k] = v
		}
	}
}

// MCPExecutor MCP 工具执行器
// MCP 协议输入输出为 JSON Schema 定义，天然动态，不使用泛型
type MCPExecutor struct {
	serverURL string
	toolName  string
	timeout   time.Duration
	headers   map[string]string
	client    *http.Client
}

// NewMCPExecutor 创建 MCP 工具执行器
func NewMCPExecutor(serverURL, toolName string, opts ...MCPOption) *MCPExecutor {
	e := &MCPExecutor{
		serverURL: serverURL,
		toolName:  toolName,
		timeout:   30 * time.Second,
		headers:   map[string]string{"Content-Type": "application/json"},
		client:    &http.Client{},
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// mcpRequest MCP tools/call 请求结构
type mcpRequest struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	} `json:"params"`
	ID int `json:"id"`
}

// mcpResponse MCP tools/call 响应结构
type mcpResponse struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Result  struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"result"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// Execute 执行 MCP 工具调用
// 将 TaskData.Input 作为 JSON 参数，构造 MCP tools/call 请求
func (e *MCPExecutor) Execute(ctx context.Context, data *TaskData) (any, error) {
	// 解析输入参数
	var arguments map[string]any
	if data.Input != "" {
		if err := unmarshalJSON([]byte(data.Input), &arguments); err != nil {
			return nil, fmt.Errorf("mcp executor parse input failed: %w", err)
		}
	}

	// 构造 MCP 请求
	reqBody := mcpRequest{
		JSONRPC: "2.0",
		Method:  "tools/call",
		ID:      int(mcpRequestID.Add(1)),
	}
	reqBody.Params.Name = e.toolName
	reqBody.Params.Arguments = arguments

	reqBytes, err := marshalJSON(reqBody)
	if err != nil {
		return nil, fmt.Errorf("mcp executor marshal request failed: %w", err)
	}

	// 设置超时
	ctx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()

	// 发送 HTTP 请求
	req, err := http.NewRequestWithContext(ctx, "POST", e.serverURL, bytes.NewReader(reqBytes))
	if err != nil {
		return nil, fmt.Errorf("mcp executor create request failed: %w", err)
	}
	for k, v := range e.headers {
		req.Header.Set(k, v)
	}

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("mcp executor request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("mcp executor read response failed: %w", err)
	}

	// 解析 MCP 响应
	var mcpResp mcpResponse
	if err := unmarshalJSON(body, &mcpResp); err != nil {
		return nil, fmt.Errorf("mcp executor unmarshal response failed: %w", err)
	}

	if mcpResp.Error != nil {
		return nil, fmt.Errorf("mcp executor error: [%d] %s", mcpResp.Error.Code, mcpResp.Error.Message)
	}

	// 提取文本内容
	result := make(map[string]any)
	for i, content := range mcpResp.Result.Content {
		if content.Type == "text" {
			// 尝试解析为 JSON，否则作为字符串
			var parsed any
			if err := unmarshalJSON([]byte(content.Text), &parsed); err == nil {
				result[fmt.Sprintf("content_%d", i)] = parsed
			} else {
				result[fmt.Sprintf("content_%d", i)] = content.Text
			}
		}
	}

	return result, nil
}

// Protocol 返回协议类型
func (e *MCPExecutor) Protocol() ExecutorProtocol { return ProtocolMCP }
