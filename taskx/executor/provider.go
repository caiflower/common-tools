package executor

import (
	"context"
	"reflect"
)

// ExecutorProvider 执行器提供者接口（最小接口，仅2个方法）
// 各实现使用泛型替代 interface{} 以提供编译时类型安全
type ExecutorProvider interface {
	// Execute 执行子任务
	Execute(ctx context.Context, data *TaskData) (any, error)
	// Protocol 返回协议类型
	Protocol() ExecutorProtocol
}

// ExecutorProtocol 执行器协议类型
type ExecutorProtocol string

const (
	ProtocolLocal ExecutorProtocol = "local" // 本地函数
	ProtocolGRPC  ExecutorProtocol = "grpc"  // gRPC 远程调用
	ProtocolHTTP  ExecutorProtocol = "http"  // HTTP REST 调用
	ProtocolMCP   ExecutorProtocol = "mcp"   // MCP 工具调用
)

// TaskData 任务执行时的数据传递结构
// 与 taskx.TaskData 对应，用于 executor 包的独立定义
type TaskData struct {
	RequestId string         `json:"requestId"`
	TaskId    string         `json:"taskId"`
	SubTaskId string         `json:"subTaskId"`
	Input     string         `json:"input"`
	Subtasks  map[string]any `json:"subtasks"`
}

// UnmarshalInput 将 Input 反序列化到目标类型
func (d *TaskData) UnmarshalInput(target any) error {
	if d.Input == "" {
		return nil
	}
	return unmarshalJSON([]byte(d.Input), target)
}

// TypedProvider 可选接口，提供输入输出类型信息用于编译时校验
// ExecutorProvider 实现此接口后，DAG 编译时会自动校验数据边的类型兼容性
type TypedProvider interface {
	ExecutorProvider
	InputType() reflect.Type
	OutputType() reflect.Type
}
