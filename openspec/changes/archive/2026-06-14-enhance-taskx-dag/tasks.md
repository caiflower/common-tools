## 1. 基础类型定义

- [ ] 1.1 创建 `taskx/dag.go`，定义核心枚举类型：`EdgeType`（ControlEdge/DataEdge）、`NodeState`（Pending/Running/Succeeded/Failed/Skipped）、`NodeTriggerMode`（AllPredecessor/AnyPredecessor）、`dependencyState`（depWaiting/depReady/depSkipped）
- [ ] 1.2 定义 `dagNode` 结构体：包含 key、subtask 引用、triggerMode、state、priority、timeout、preProcessor、postProcessor 字段
- [ ] 1.3 定义 `dagEdge` 结构体：包含 from、to、edgeType、mappings 字段
- [ ] 1.4 定义 `FieldMapping` 结构体：包含 SourceField、TargetField 字段
- [ ] 1.5 定义 `Branch` 结构体：包含 condition 函数、endNodes map
- [ ] 1.6 定义 `RollbackStrategy` 枚举：RollbackAll / RollbackFailed / RollbackCustom
- [ ] 1.7 定义 `Processor` 函数类型：`func(ctx context.Context, data any) (any, error)`
- [ ] 1.8 定义 `DAGCallback` 接口：OnSubtaskStart/OnSubtaskComplete/OnSubtaskFailed/OnSubtaskSkipped/OnBranchSelected

## 2. dagGraph 图结构实现

- [ ] 2.1 实现 `dagGraph` 结构体：包含 nodes map、edges 切片、controlAdj/controlPred/dataAdj/dataPred 邻接表、branches map、compiled bool
- [ ] 2.2 实现 `dagGraph.AddNode(key, subtask, triggerMode)` 方法
- [ ] 2.3 实现 `dagGraph.AddEdge(from, to, edgeType, mappings)` 方法，同时维护邻接表
- [ ] 2.4 实现 `dagGraph.AddBranch(nodeKey, branch)` 方法
- [ ] 2.5 实现 `dagGraph.GetNode(key)` 和 `dagGraph.GetNodesByState(state)` 查询方法
- [ ] 2.6 实现 `dagGraph.UpdateNodeState(key, state)` 方法
- [ ] 2.7 实现 `dagGraph.TopologicalSort()` 拓扑排序方法
- [ ] 2.8 编写 dagGraph 单元测试：节点添加、边添加、状态更新、拓扑排序

## 3. dagChannel 通道实现

- [ ] 3.1 创建 `taskx/channel.go`，实现 `dagChannel` 结构体：包含 nodeKey、controlPredecessors map、dataPredecessors map、values map、skipped bool
- [ ] 3.2 实现 `dagChannel.reportValues(ins map[string]any)` 方法
- [ ] 3.3 实现 `dagChannel.reportDependencies(deps []string)` 方法
- [ ] 3.4 实现 `dagChannel.reportSkip(keys []string) bool` 方法
- [ ] 3.5 实现 `dagChannel.get() (any, bool, error)` 方法：检查依赖就绪状态，返回合并输入
- [ ] 3.6 编写 dagChannel 单元测试：依赖就绪、值传递、Skip 传播

## 4. 图编译（Compile）实现

- [ ] 4.1 创建 `taskx/compile.go`，实现 `compiledDAG` 结构体：包含 dagGraph、channels map、branches map、startNodes、endNodes
- [ ] 4.2 实现 `dagGraph.Compile()` 方法：构建 channels、校验环、校验起止节点、校验分支目标
- [ ] 4.3 实现环检测算法 `validateDAG()`（参考 eino 的拓扑排序方式）
- [ ] 4.4 实现类型兼容性校验：无字段映射时检查前驱输出类型可赋值给后继输入类型
- [ ] 4.5 实现字段映射校验：目标字段冲突检测、源类型必须为 map/struct
- [ ] 4.6 实现编译后不可变保护：编译后调用 AddNode/AddEdge/AddBranch 返回 ErrGraphCompiled
- [ ] 4.7 编写 Compile 单元测试：合法图编译、环检测、缺少起止节点、类型不兼容、字段冲突

## 5. Task 模型重构

- [ ] 5.1 重构 `Task` 结构体：将 `g graph.Graph` 替换为 `dag *dagGraph`，新增 `compiled *compiledDAG`、`em *executorManager`、`callback DAGCallback`、`rollbackStrategy RollbackStrategy`、`customRollbackFunc` 字段
- [ ] 5.2 修改 `NewTask()` 构造函数：使用 dagGraph 替代 dominikbraun/graph，初始化实例级 executorManager
- [ ] 5.3 实现 `Task.AddControlEdge(src, dst)` 方法
- [ ] 5.4 实现 `Task.AddDataEdge(src, dst, mappings...)` 方法
- [ ] 5.5 实现 `Task.AddEdge(src, dst)` 方法（同时添加 ControlEdge + DataEdge）
- [ ] 5.6 修改 `Task.AddSubtask()` 适配 dagGraph
- [ ] 5.7 修改 `Task.NextSubTasks()` 基于 dagChannel 状态查询可执行节点，按优先级降序排列
- [ ] 5.8 修改 `Task.updateSubtaskState()` 为非破坏性状态更新，完成后调用 dagChannel 的 reportValues/reportDependencies
- [ ] 5.9 实现 `Task.skipSubtask(key)` 方法，调用 dagChannel 的 reportSkip
- [ ] 5.10 实现 `Task.Compile()` 方法，委托给 dagGraph.Compile()
- [ ] 5.11 实现 `Task.SetRollbackStrategy(strategy)` 和 `Task.SetCustomRollbackFunc(fn)` 方法
- [ ] 5.12 实现 `Task.SetCallback(callback DAGCallback)` 方法
- [ ] 5.13 实现 `Task.RegisterTaskExecutor(executor, subtaskExecutors)` 实例级注册
- [ ] 5.14 实现 `Task.RegisterBranchCondition(branchKey, conditionFunc)` 实例级注册
- [ ] 5.15 实现 `Task.AddSubtaskGraph(key, subTask)` 子图嵌套
- [ ] 5.16 修改 `Task.Graph()` 拓扑排序展示方法适配新图结构
- [ ] 5.17 实现 `Task.GraphDOT()` 方法输出 Graphviz DOT 格式
- [ ] 5.18 实现 `Task.GraphMermaid()` 方法输出 Mermaid 格式
- [ ] 5.19 修改 `Task.convert2Bean()` 适配新的边类型（持久化控制边和数据边）
- [ ] 5.20 修改 `Task.initByBean()` 适配新的边类型（从数据库恢复图结构）
- [ ] 5.21 编写 Task 重构单元测试：AddControlEdge/AddDataEdge/AddEdge、NextSubTasks、状态更新、Compile、实例级注册

## 6. Subtask 模型重构

- [ ] 6.1 重构 `Subtask` 为泛型结构体 `Subtask[I any, O any]`，包含 inputType/outputType reflect.Type
- [ ] 6.2 实现 `NewSubtask[I, O](name)` 泛型构造函数
- [ ] 6.3 为 Subtask 新增 `triggerMode NodeTriggerMode` 字段及 `SetTriggerMode(mode)` 方法
- [ ] 6.4 为 Subtask 新增 `priority int` 字段及 `SetPriority(n)` 方法
- [ ] 6.5 为 Subtask 新增 `timeout time.Duration` 字段及 `SetTimeout(d)` 方法
- [ ] 6.6 为 Subtask 新增 `preProcessor Processor` 字段及 `SetPreProcessor(fn)` 方法
- [ ] 6.7 为 Subtask 新增 `postProcessor Processor` 字段及 `SetPostProcessor(fn)` 方法
- [ ] 6.8 修改 Subtask 状态判断方法适配 Skipped 状态
- [ ] 6.9 编写 Subtask 泛型单元测试

## 7. 执行器接口重构

- [ ] 7.1 修改 `SubTaskExecutor` 签名为 `func(ctx context.Context, data *TaskData) (output interface{}, err error)`
- [ ] 7.2 扩展 `TaskData` 结构体：新增 `MergedInput map[string]any` 字段
- [ ] 7.3 重构 `executorManager` 为非全局结构体，支持实例化
- [ ] 7.4 实现 `executorManager` 的 `registerTaskExecutor`、`registerSubTaskExecutor`、`registerRollbackTaskExecutor`、`registerBranchCondition` 方法
- [ ] 7.5 移除全局变量 `_em`，所有注册通过 Task 实例调用
- [ ] 7.6 编写执行器注册隔离测试：多个 Task 实例互不影响

## 8. 数据模型与 DAO 迁移

- [ ] 8.1 修改 `dao/model/subtask.go`：新增 trigger_mode、priority、timeout、edge_type 字段
- [ ] 8.2 修改 `dao/model/subtask.go`：新增 control_pre_subtask_ids 和 data_pre_subtask_ids 字段（替代 PreSubtaskID）
- [ ] 8.3 修改 `dao/subtask.go` DAO 层适配新字段
- [ ] 8.4 编写数据库迁移脚本

## 9. 调度器适配

- [ ] 9.1 修改 `dispatch.go` 中 `analysisTask()` 适配新的 NextSubTasks 返回值（按优先级排序的节点列表）
- [ ] 9.2 修改 `dispatch.go` 中 `handleTaskImmediately()` 适配 compiledDAG
- [ ] 9.3 实现分支条件执行逻辑：分支节点完成后调用 condition 确定路径，Skip 未选中节点
- [ ] 9.4 实现字段映射数据传递：子任务执行时将合并后的输入传入 TaskData.MergedInput
- [ ] 9.5 实现 Context 传播：Task 提交时的 ctx 传递到子任务执行器
- [ ] 9.6 实现子任务超时控制：执行时创建带超时的 context
- [ ] 9.7 实现 PreProcessor / PostProcessor 调用链
- [ ] 9.8 实现 DAGCallback 回调触发
- [ ] 9.9 修改回滚逻辑适配 RollbackStrategy：RollbackAll/RollbackFailed/RollbackCustom
- [ ] 9.10 实现子图节点执行逻辑：编译子图 → 执行子图内部 DAG → 返回最终输出

## 11. ExecutorProvider 执行器协议抽象（泛型化）

- [ ] 11.1 创建 `taskx/executor/provider.go`，定义 `ExecutorProvider` 接口（Execute、Protocol 两个方法）、`ExecutorProtocol` 类型及常量（ProtocolLocal/ProtocolGRPC/ProtocolHTTP/ProtocolMCP）
- [ ] 11.2 创建 `taskx/executor/local.go`，实现泛型 `LocalExecutor[I any, O any]`：包装 `func(ctx context.Context, input I) (O, error)` 为 ExecutorProvider，Execute 方法从 TaskData 反序列化到 I 类型后调用函数，提供 `NewLocalExecutor[I, O](fn)` 构造函数
- [ ] 11.3 创建 `taskx/executor/grpc.go`，实现泛型 `GRPCExecutor[I any, O any]`：配置 endpoint/serviceName/methodName/timeout/dialOpts，Execute 方法反序列化输入→gRPC 调用→反序列化响应，提供 `NewGRPCExecutor[I, O]` 构造函数和 `GRPCOption` Functional Options
- [ ] 11.4 创建 `taskx/executor/http.go`，实现泛型 `HTTPExecutor[I any, O any]`：配置 url/method/headers/timeout，Execute 方法反序列化输入→JSON 编码→HTTP 请求→JSON 解码→反序列化响应，提供 `NewHTTPExecutor[I, O]` 构造函数和 `HTTPOption` Functional Options
- [ ] 11.5 创建 `taskx/executor/mcp.go`，实现 `MCPExecutor`（非泛型）：配置 serverURL/toolName/timeout，Execute 方法将 TaskData.Input 作为 JSON 参数构造 MCP tools/call 请求，返回 `map[string]any`，提供 `NewMCPExecutor` 构造函数和 `MCPOption` Functional Options
- [ ] 11.6 实现 `legacyExecutorWrapper`：包装旧 `SubTaskExecutor` 为 ExecutorProvider，Protocol 返回 ProtocolLocal，Execute 委托给原始函数
- [ ] 11.7 修改 `executorManager`：新增 `subtaskProviders map[string]map[string]ExecutorProvider` 字段，实现 `registerProviders(taskName, providers)` 和 `getProvider(taskName, subTaskName)` 方法（优先返回 Provider，否则用 legacyExecutorWrapper 包装旧 SubTaskExecutor）
- [ ] 11.8 新增 `Task.RegisterProviders(executor, map[string]ExecutorProvider)` 方法，保留旧 `Task.RegisterTaskExecutor(executor, map[string]SubTaskExecutor)` 向后兼容
- [ ] 11.9 修改 `TaskData`：确保 `UnmarshalInput(v any)` 方法支持将序列化的 Input 反序列化到泛型类型 I
- [ ] 11.10 修改调度执行逻辑（receiver.go）：子任务执行时通过 `getProvider` 获取 `ExecutorProvider`，调用 `Execute` 方法
- [ ] 11.11 编写 LocalExecutor 泛型单元测试：类型安全函数调用、序列化/反序列化、Protocol 返回值
- [ ] 11.12 编写 GRPCExecutor 泛型集成测试（使用 mock gRPC server）
- [ ] 11.13 编写 HTTPExecutor 泛型集成测试（使用 httptest.Server）
- [ ] 11.14 编写 MCPExecutor 集成测试（使用 mock MCP server）
- [ ] 11.15 编写向后兼容测试：旧 SubTaskExecutor 注册方式自动包装为 legacyExecutorWrapper，行为一致
- [ ] 11.16 编写混合协议注册测试：同一个 Task 中同时注册 Local/GRPC/HTTP/MCP 执行器
- [ ] 11.17 编写序列化约束测试：验证不可 JSON 序列化的类型在执行时正确返回错误

## 12. 集成测试与文档

- [ ] 10.1 编写端到端集成测试：控制/数据依赖分离场景
- [ ] 10.2 编写端到端集成测试：条件分支场景
- [ ] 10.3 编写端到端集成测试：Skip 传播场景
- [ ] 10.4 编写端到端集成测试：字段映射场景
- [ ] 10.5 编写端到端集成测试：Context 传播与超时控制场景
- [ ] 10.6 编写端到端集成测试：回滚策略场景（All/Failed/Custom）
- [ ] 10.7 编写端到端集成测试：Pre/Post Processor 场景
- [ ] 10.8 编写端到端集成测试：DAGCallback 回调场景
- [ ] 10.9 编写端到端集成测试：子图嵌套场景
- [ ] 10.10 编写端到端集成测试：优先级调度场景
- [ ] 10.11 编写端到端集成测试：泛型类型安全场景
- [ ] 10.12 编写图可视化测试：DOT 和 Mermaid 输出
- [ ] 10.13 更新 `docs/taskx/README.md` 文档
