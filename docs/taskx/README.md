# taskx 使用说明

## 1. 简介

taskx 是一个分布式任务调度框架，支持任务编排、依赖管理、重试机制和回滚功能。

## 2. 核心概念

### 2.1 Task（任务）
- 一个完整的业务流程单元
- 包含多个子任务（Subtask）
- 支持任务状态管理和重试配置

### 2.2 Subtask（子任务）
- 任务的最小执行单元
- 支持依赖关系定义
- 可配置重试次数和重试间隔

### 2.3 Executor（执行器）
- 任务执行逻辑的实现者
- 分为 TaskExecutor 和 SubTaskExecutor
- 支持回滚逻辑注册

## 3. 快速开始

### 3.1 定义执行器

```go
// 定义子任务执行逻辑
var mySubtaskExecutor = func(data *taskx.TaskData) (interface{}, error) {
    // 子任务执行逻辑
    return "result", nil
}

// 定义任务执行器
type MyTaskExecutor struct{}

func (e *MyTaskExecutor) Name() string {
    return "MyTask"
}

func (e *MyTaskExecutor) FinishedTask(data *taskx.TaskData) error {
    // 任务完成后回调
    return nil
}

func (e *MyTaskExecutor) FailedTask(data *taskx.TaskData) error {
    // 任务失败后回调
    return nil
}
```

### 3.2 注册执行器

```go
// 注册任务执行器和子任务执行器
taskx.RegisterTaskExecutor(&MyTaskExecutor{}, map[string]taskx.SubTaskExecutor{
    "MySubtask": mySubtaskExecutor,
})
```

### 3.3 创建并提交任务

```go
// 创建任务
task := taskx.NewTask("MyTask")

// 创建子任务
subtask := taskx.NewSubtask("MySubtask")
subtask.SetInput("input data")

// 添加子任务到任务
task.AddSubtask(subtask)

// 提交任务
err := taskx.SubmitTask(task)
if err != nil {
    // 处理错误
}
```

## 4. 高级功能

### 4.1 任务编排

```go
// 创建任务
task := taskx.NewTask("MyTask")

// 创建子任务 A 和 B
subtaskA := taskx.NewSubtask("SubtaskA")
subtaskB := taskx.NewSubtask("SubtaskB")

// 添加子任务依赖：B 依赖 A 完成
task.AddSubtask(subtaskA)
task.AddSubtask(subtaskB)
task.AddDirectedEdge(subtaskA, subtaskB)
```

### 4.2 重试配置

```go
// 设置子任务重试次数为 5 次，重试间隔为 10 秒
subtask := taskx.NewSubtask("MySubtask")
subtask.SetRetry(5)
subtask.SetRetryInterval(10)
```

### 4.3 回滚机制

```go
// 定义回滚逻辑
var myRollbackExecutor = func(data *taskx.TaskData) (interface{}, error) {
    // 回滚逻辑
    return nil, nil
}

// 注册带回滚的执行器
taskx.RegisterTaskExecutorWithRollback(&MyTaskExecutor{}, 
    map[string]taskx.SubTaskExecutor{"MySubtask": mySubtaskExecutor},
    map[string]taskx.SubTaskExecutor{"MySubtask": myRollbackExecutor},
)
```

## 5. 配置参数

| 参数名 | 类型 | 默认值 | 描述 |
|--------|------|--------|------|
| TaskWorker | int | 20 | 任务处理协程数 |
| TaskQueueSize | int | 100 | 任务队列大小 |
| SubtaskWorker | int | 100 | 子任务处理协程数 |
| SubtaskQueueSize | int | 200 | 子任务队列大小 |
| SubtaskRollbackWorker | int | 50 | 子任务回滚处理协程数 |
| SubtaskRollbackQueueSize | int | 100 | 子任务回滚队列大小 |
| RemoteCallTimeout | time.Duration | 3s | 远程调用超时时间 |
| BackupTaskAge | time.Duration | 168h | 任务备份保留时间 |

## 6. 任务状态

- **Pending**: 任务待执行
- **Running**: 任务执行中
- **SubtaskRunning**: 子任务执行中
- **Failed**: 任务失败
- **Succeeded**: 任务成功

## 7. 错误处理

### 7.1 可重试错误

```go
// 普通错误会触发重试
return nil, fmt.Errorf("retryable error")
```

### 7.2 不可重试错误

```go
// 不可重试错误会直接标记任务失败
return nil, taskx.ErrNonRetryable
```

## 8. 最佳实践

1. **任务拆分**：将大型任务拆分为多个子任务，提高并行度和可维护性
2. **依赖管理**：合理定义子任务依赖关系，确保执行顺序正确
3. **重试配置**：根据业务特性调整重试次数和间隔
4. **回滚设计**：对有状态操作实现回滚逻辑，确保数据一致性
5. **错误处理**：合理使用可重试和不可重试错误类型

## 9. API 参考

### 9.1 Task 方法

- `NewTask(taskName string) *Task`: 创建新任务
- `AddSubtask(subtask *Subtask) error`: 添加子任务
- `AddDirectedEdge(src, dst *Subtask) error`: 添加子任务依赖
- `SetInput(content interface{}) *Task`: 设置任务输入
- `SetDescription(description string) *Task`: 设置任务描述
- `SetRetry(retry int8) *Task`: 设置任务重试次数
- `SetRetryInterval(retryInterval int32) *Task`: 设置任务重试间隔
- `SetUrgent() *Task`: 设置为紧急任务

### 9.2 Subtask 方法

- `NewSubtask(taskName string) *Subtask`: 创建新子任务
- `SetInput(content interface{}) *Subtask`: 设置子任务输入
- `SetRetry(retry int8) *Subtask`: 设置重试次数
- `SetRetryInterval(retryInterval int32) *Subtask`: 设置重试间隔

### 9.3 执行器注册

- `RegisterTaskExecutor(taskExecutor TaskExecutor, subTaskExecutor map[string]SubTaskExecutor)`: 注册任务执行器
- `RegisterTaskExecutorWithRollback(taskExecutor TaskExecutor, subtaskExecutor map[string]SubTaskExecutor, subtaskRollbackExecutor map[string]SubTaskExecutor)`: 注册带回滚的任务执行器

### 9.4 任务提交

- `SubmitTask(task *Task) error`: 提交任务
- `SubmitTaskWithTx(task *Task, tx *bun.Tx) error`: 带事务提交任务