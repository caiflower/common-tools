## ADDED Requirements

### Requirement: DAGCallback 接口定义
系统 SHALL 定义 DAGCallback 接口，在子任务生命周期的关键节点触发回调通知。Callback 不修改数据，仅做观察。

#### Scenario: 子任务启动时触发回调
- **WHEN** 子任务开始执行
- **THEN** 系统调用 OnSubtaskStart(ctx, key, input) 通知回调

#### Scenario: 子任务成功完成时触发回调
- **WHEN** 子任务执行成功
- **THEN** 系统调用 OnSubtaskComplete(ctx, key, output) 通知回调

#### Scenario: 子任务失败时触发回调
- **WHEN** 子任务执行失败
- **THEN** 系统调用 OnSubtaskFailed(ctx, key, err) 通知回调

#### Scenario: 子任务跳过时触发回调
- **WHEN** 子任务被标记为 Skipped
- **THEN** 系统调用 OnSubtaskSkipped(ctx, key) 通知回调

#### Scenario: 分支选择时触发回调
- **WHEN** 条件分支选择了目标节点
- **THEN** 系统调用 OnBranchSelected(ctx, fromNode, selectedNode) 通知回调

### Requirement: Callback 注册
系统 SHALL 支持在 Task 级别注册 DAGCallback。

#### Scenario: 注册回调
- **WHEN** 用户调用 task.SetCallback(myCallback)
- **THEN** 该 Task 的所有子任务生命周期事件均通知该回调

#### Scenario: 不注册回调时正常运行
- **WHEN** 用户未设置 Callback
- **THEN** Task 正常执行，不触发任何回调

### Requirement: Callback 与 Processor 的区别
Callback SHALL 只做观察，不修改数据；Processor 可以修改数据。Callback 的执行错误 SHALL 不影响任务执行流程。

#### Scenario: Callback 返回错误不影响任务
- **WHEN** Callback 的某个方法返回错误
- **THEN** 系统记录错误日志，但任务继续正常执行

#### Scenario: Processor 可以修改数据
- **WHEN** PreProcessor 修改了输入数据
- **THEN** SubTaskExecutor 接收修改后的数据
