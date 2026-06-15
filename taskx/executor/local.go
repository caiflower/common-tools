package executor

import (
	"context"
	"fmt"
	"reflect"
)

// LocalExecutor 本地函数执行器，泛型参数 I/O 提供编译时类型安全
// I 和 O 必须可 JSON 序列化（集群框架约束）
type LocalExecutor[I any, O any] struct {
	fn func(ctx context.Context, input I) (O, error)
}

// NewLocalExecutor 创建本地函数执行器
// 函数签名为 func(ctx context.Context, input I) (O, error)，编译时类型检查，无需 interface{} 断言
func NewLocalExecutor[I any, O any](fn func(ctx context.Context, input I) (O, error)) *LocalExecutor[I, O] {
	return &LocalExecutor[I, O]{fn: fn}
}

// Execute 执行本地函数
// 从 TaskData 反序列化输入到 I 类型，调用函数，返回 O 类型输出
func (e *LocalExecutor[I, O]) Execute(ctx context.Context, data *TaskData) (any, error) {
	var input I
	if err := data.UnmarshalInput(&input); err != nil {
		return nil, fmt.Errorf("local executor unmarshal input failed: %w", err)
	}
	output, err := e.fn(ctx, input)
	if err != nil {
		return nil, err
	}
	return output, nil
}

// Protocol 返回协议类型
func (e *LocalExecutor[I, O]) Protocol() ExecutorProtocol { return ProtocolLocal }

// InputType 返回输入类型（实现 TypedProvider 接口）
func (e *LocalExecutor[I, O]) InputType() reflect.Type {
	var i I
	return reflect.TypeOf(i)
}

// OutputType 返回输出类型（实现 TypedProvider 接口）
func (e *LocalExecutor[I, O]) OutputType() reflect.Type {
	var o O
	return reflect.TypeOf(o)
}
