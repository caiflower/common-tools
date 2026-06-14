## ADDED Requirements

### Requirement: Context 传播
系统 SHALL 在子任务执行时传递 context.Context，支持超时控制、取消传播和链路追踪。

#### Scenario: Context 从 Task 传递到子任务
- **WHEN** Task 提交时携带 ctx
- **THEN** 子任务执行时接收从 Task ctx 派生的 context

#### Scenario: 取消信号传播
- **WHEN** Task 的 ctx 被取消
- **THEN** 所有正在执行的子任务 SHALL 感知取消信号并终止

#### Scenario: TraceID 传播
- **WHEN** Task 的 ctx 中包含 TraceID
- **THEN** 子任务执行时的 ctx 中 SHALL 包含相同的 TraceID

### Requirement: 子任务级超时控制
系统 SHALL 支持为每个子任务设置独立的执行超时时间。

#### Scenario: 设置子任务超时
- **WHEN** 用户调用 subtask.SetTimeout(30 * time.Second)
- **THEN** 子任务执行时，若超过 30 秒未完成，context 自动取消

#### Scenario: 超时导致子任务失败
- **WHEN** 子任务执行超过设定的超时时间
- **THEN** 子任务状态标记为 Failed，错误信息包含超时原因

#### Scenario: 未设置超时时使用默认值
- **WHEN** 子任务未设置超时
- **THEN** 子任务使用 Task 级别的默认超时（若配置），或无超时限制

### Requirement: SubTaskExecutor 签名包含 Context
SubTaskExecutor 函数类型 SHALL 接受 ctx context.Context 作为第一个参数。

#### Scenario: 执行器接收 Context
- **WHEN** 子任务被调度执行
- **THEN** SubTaskExecutor 接收 ctx context.Context 和 TaskData 两个参数

#### Scenario: 执行器响应取消
- **WHEN** ctx 被取消，且执行器检查 ctx.Done()
- **THEN** 执行器 SHALL 尽快返回 context.Canceled 错误
