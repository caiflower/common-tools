package executor

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/caiflower/common-tools/pkg/json"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// jsonCodec 实现 gRPC Codec 接口，用于 JSON 编解码
type jsonCodec struct{}

func (jsonCodec) Marshal(v interface{}) ([]byte, error) {
	return marshalJSON(v)
}

func (jsonCodec) Unmarshal(data []byte, v interface{}) error {
	return unmarshalJSON(data, v)
}

func (jsonCodec) Name() string {
	return "json"
}

// grpcConfig gRPC 执行器配置
type grpcConfig struct {
	dialOpts []grpc.DialOption
	timeout  time.Duration
}

// GRPCOption gRPC 执行器配置选项
type GRPCOption func(*grpcConfig)

// WithGRPCDialOptions 设置 gRPC dial 选项
func WithGRPCDialOptions(opts ...grpc.DialOption) GRPCOption {
	return func(c *grpcConfig) {
		c.dialOpts = append(c.dialOpts, opts...)
	}
}

// WithGRPCTimeout 设置超时时间
func WithGRPCTimeout(timeout time.Duration) GRPCOption {
	return func(c *grpcConfig) {
		c.timeout = timeout
	}
}

// GRPCExecutor gRPC 远程执行器，泛型参数指定请求/响应类型
type GRPCExecutor[I any, O any] struct {
	endpoint    string
	serviceName string
	methodName  string
	timeout     time.Duration
	dialOpts    []grpc.DialOption
	conn        *grpc.ClientConn
	connOnce    sync.Once
	connErr     error
}

// NewGRPCExecutor 创建 gRPC 远程执行器
func NewGRPCExecutor[I any, O any](endpoint, serviceName, methodName string, opts ...GRPCOption) *GRPCExecutor[I, O] {
	cfg := grpcConfig{
		timeout:  30 * time.Second,
		dialOpts: []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())},
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	return &GRPCExecutor[I, O]{
		endpoint:    endpoint,
		serviceName: serviceName,
		methodName:  methodName,
		timeout:     cfg.timeout,
		dialOpts:    cfg.dialOpts,
	}
}

// initConn 懒加载 gRPC 连接，使用 sync.Once 保证并发安全
func (e *GRPCExecutor[I, O]) initConn() error {
	e.connOnce.Do(func() {
		conn, err := grpc.NewClient(e.endpoint, e.dialOpts...)
		if err != nil {
			e.connErr = err
			return
		}
		e.conn = conn
	})
	return e.connErr
}

// Execute 通过 gRPC 调用远程服务
// 从 TaskData 反序列化输入到 I 类型，使用 JSON codec 通过 gRPC 调用，反序列化响应到 O 类型
func (e *GRPCExecutor[I, O]) Execute(ctx context.Context, data *TaskData) (any, error) {
	var input I
	if err := data.UnmarshalInput(&input); err != nil {
		return nil, fmt.Errorf("grpc executor unmarshal input failed: %w", err)
	}

	// 建立连接（sync.Once 保护）
	if err := e.initConn(); err != nil {
		return nil, fmt.Errorf("grpc executor connect failed: %w", err)
	}

	// 设置超时
	ctx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()

	// 序列化输入
	inputBytes, err := marshalJSON(input)
	if err != nil {
		return nil, fmt.Errorf("grpc executor marshal input failed: %w", err)
	}

	// 使用 JSON codec 调用 gRPC
	var output O
	err = e.conn.Invoke(ctx, "/"+e.serviceName+"/"+e.methodName, json.RawMessage(inputBytes), &output,
		grpc.ForceCodec(jsonCodec{}),
	)
	if err != nil {
		return nil, fmt.Errorf("grpc executor invoke failed: %w", err)
	}

	return output, nil
}

// Protocol 返回协议类型
func (e *GRPCExecutor[I, O]) Protocol() ExecutorProtocol { return ProtocolGRPC }

// Close 关闭 gRPC 连接
func (e *GRPCExecutor[I, O]) Close() error {
	if e.conn != nil {
		return e.conn.Close()
	}
	return nil
}
