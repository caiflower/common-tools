## MODIFIED Requirements

### Requirement: SubTaskExecutor 签名重构
SubTaskExecutor 函数类型 SHALL 接受 ctx context.Context 作为第一个参数，支持超时控制和取消传播。

#### Scenario: 执行器接收 Context
- **WHEN** 子任务被调度执行
- **THEN** SubTaskExecutor 接收 (ctx context.Context, data *TaskData) 两个参数

#### Scenario: 执行器响应超时
- **WHEN** 子任务设置了超时时间，且执行超过超时时间
- **THEN** ctx 被取消，执行器 SHALL 检查 ctx 并返回超时错误

### Requirement: TaskData 结构扩展
TaskData SHALL 扩展以支持传递合并后的多前驱数据和 Context 信息。

#### Scenario: TaskData 包含合并输入
- **WHEN** 子任务有多个数据前驱
- **THEN** TaskData 中新增 MergedInput 字段（map[string]any 类型），包含按字段映射合并后的数据

#### Scenario: 单前驱时 MergedInput 为 nil
- **WHEN** 子任务只有一个数据前驱且无字段映射
- **THEN** TaskData.Input 行为不变，MergedInput 为 nil

### Requirement: 执行器注册改为实例级
执行器注册 SHALL 从全局变量改为 Task 实例级，支持依赖注入和测试隔离。

#### Scenario: 实例级注册执行器
- **WHEN** 用户调用 task.RegisterTaskExecutor(executor, subtaskExecutors)
- **THEN** 执行器注册到该 Task 实例，不同 Task 实例可注册不同执行器

#### Scenario: 未注册执行器时报错
- **WHEN** 子任务执行时找不到对应的 SubTaskExecutor
- **THEN** 子任务标记为 Failed，错误信息包含缺失的执行器名称

#### Scenario: 测试隔离
- **WHEN** 测试用例 A 和测试用例 B 各自创建 Task 并注册不同执行器
- **THEN** 两个测试用例的执行器互不影响

### Requirement: Branch 条件执行器注册
系统 SHALL 支持在 Task 实例级注册 Branch 条件执行器。

#### Scenario: 注册 Branch 条件执行器
- **WHEN** 用户调用 task.RegisterBranchCondition(branchKey, conditionFunc)
- **THEN** 分支节点执行完成后，系统调用 conditionFunc 确定目标路径

### Requirement: Pre/Post Processor
每个子任务 SHALL 支持挂载前置处理器（PreProcessor）和后置处理器（PostProcessor）。

#### Scenario: 设置 PreProcessor
- **WHEN** 用户调用 subtask.SetPreProcessor(processor)
- **THEN** 子任务执行时，先执行 PreProcessor，其输出作为 SubTaskExecutor 的输入

#### Scenario: 设置 PostProcessor
- **WHEN** 用户调用 subtask.SetPostProcessor(processor)
- **THEN** 子任务执行时，SubTaskExecutor 的输出经过 PostProcessor 处理后作为节点输出

#### Scenario: PreProcessor 失败导致子任务失败
- **WHEN** PreProcessor 返回错误
- **THEN** 子任务标记为 Failed，不执行 SubTaskExecutor

#### Scenario: PostProcessor 失败导致子任务失败
- **WHEN** PostProcessor 返回错误
- **THEN** 子任务标记为 Failed

#### Scenario: 执行顺序
- **WHEN** 子任务同时设置了 PreProcessor 和 PostProcessor
- **THEN** 执行顺序为 PreProcessor → SubTaskExecutor → PostProcessor
