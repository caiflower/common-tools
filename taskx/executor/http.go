package executor

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

// httpConfig HTTP 执行器内部配置结构
type httpConfig struct {
	headers map[string]string
	timeout time.Duration
	client  *http.Client
}

// HTTPOption HTTP 执行器配置选项
type HTTPOption func(*httpConfig)

// WithHTTPHeaders 设置请求头
func WithHTTPHeaders(headers map[string]string) HTTPOption {
	return func(cfg *httpConfig) {
		if cfg.headers == nil {
			cfg.headers = make(map[string]string)
		}
		for k, v := range headers {
			cfg.headers[k] = v
		}
	}
}

// WithHTTPTimeout 设置超时时间
func WithHTTPTimeout(timeout time.Duration) HTTPOption {
	return func(cfg *httpConfig) {
		cfg.timeout = timeout
	}
}

// WithHTTPClient 设置自定义 HTTP 客户端
func WithHTTPClient(client *http.Client) HTTPOption {
	return func(cfg *httpConfig) {
		cfg.client = client
	}
}

// HTTPExecutor HTTP 远程执行器，泛型参数指定请求体/响应体类型
type HTTPExecutor[I any, O any] struct {
	url     string
	method  string
	headers map[string]string
	timeout time.Duration
	client  *http.Client
}

// NewHTTPExecutor 创建 HTTP 远程执行器
func NewHTTPExecutor[I any, O any](url, method string, opts ...HTTPOption) *HTTPExecutor[I, O] {
	cfg := httpConfig{
		timeout: 30 * time.Second,
		headers: map[string]string{"Content-Type": "application/json"},
		client:  &http.Client{},
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	return &HTTPExecutor[I, O]{
		url:     url,
		method:  method,
		timeout: cfg.timeout,
		headers: cfg.headers,
		client:  cfg.client,
	}
}

// Execute 通过 HTTP 调用远程服务
// 从 TaskData 反序列化输入到 I 类型，序列化为 JSON 发起 HTTP 请求，反序列化响应到 O 类型
func (e *HTTPExecutor[I, O]) Execute(ctx context.Context, data *TaskData) (any, error) {
	var input I
	if err := data.UnmarshalInput(&input); err != nil {
		return nil, fmt.Errorf("http executor unmarshal input failed: %w", err)
	}

	// 序列化输入
	inputBytes, err := marshalJSON(input)
	if err != nil {
		return nil, fmt.Errorf("http executor marshal input failed: %w", err)
	}

	// 设置超时
	ctx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()

	// 构造请求
	req, err := http.NewRequestWithContext(ctx, e.method, e.url, bytes.NewReader(inputBytes))
	if err != nil {
		return nil, fmt.Errorf("http executor create request failed: %w", err)
	}

	// 设置请求头
	for k, v := range e.headers {
		req.Header.Set(k, v)
	}

	// 发送请求
	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http executor request failed: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("http executor read response failed: %w", err)
	}

	// 检查状态码
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("http executor received status %d: %s", resp.StatusCode, string(body))
	}

	// 反序列化响应到 O 类型
	var output O
	if err := unmarshalJSON(body, &output); err != nil {
		return nil, fmt.Errorf("http executor unmarshal response failed: %w", err)
	}

	return output, nil
}

// Protocol 返回协议类型
func (e *HTTPExecutor[I, O]) Protocol() ExecutorProtocol { return ProtocolHTTP }
