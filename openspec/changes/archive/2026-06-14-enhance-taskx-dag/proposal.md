## Why

当前 taskx 的 DAG 实现功能较为基础：子任务完成后直接删除图节点和边（破坏性修改），依赖关系和数据流耦合在一起，不支持条件分支、节点跳过、字段级数据映射等高级特性。此外，输入输出全部为 string 缺乏类型安全，子任务执行没有 Context 传播和超时控制，执行器注册使用全局变量无法测试隔离，回滚策略硬编码，缺少可观测性回调。参考字节跳动开源 eino 项目的 DAG 设计，对 taskx 进行全面重构增强。

## What Changes

- **控制依赖与数据依赖分离**：将现有的单一有向边拆分为 controlEdge（执行依赖）和 dataEdge（数据依赖），支持"只等执行但不传数据"和"只传数据不阻塞执行"两种语义
- **非破坏性状态管理**：子任务完成后不再删除图节点和边，改为通过状态标记（Waiting/Ready/Skipped/Completed）跟踪节点执行进度，保留完整图结构以支持重放和回溯
- **条件分支（Branch）**：支持根据运行时条件动态选择执行路径，允许 DAG 中存在互斥分支
- **节点触发模式**：支持 AnyPredecessor（任一前驱完成即触发）和 AllPredecessor（所有前驱完成才触发）两种触发模式
- **Skip 机制**：前驱节点跳过时，后继节点可自动跳过，而非只能回滚
- **字段级数据映射**：子任务间支持字段级别的数据映射，而非只能传递整个 Output
- **图编译（Compile）阶段**：图构建后需编译，编译时进行环检测、类型校验等，生成不可变可运行对象，提前发现配置错误
- **泛型类型安全**：Subtask 支持泛型输入输出类型，Compile 阶段校验前驱输出类型与后继输入类型兼容
- **Context 传播与超时控制**：SubTaskExecutor 签名增加 ctx context.Context，子任务支持独立超时配置
- **执行器注册改为实例级**：移除全局变量，执行器注册改为 DAG 实例级，支持依赖注入和测试隔离
- **执行器协议抽象**：执行器注册接口抽象为支持多种协议（本地函数、gRPC、HTTP、MCP），通过 ExecutorProvider 统一接口屏蔽底层协议差异
- **回滚策略可配置**：支持 RollbackAll / RollbackFailed / RollbackCustom 三种回滚策略
- **Pre/Post Processor**：子任务支持前置/后置处理器，用于日志、指标、数据转换等横切关注点
- **进度追踪与回调**：支持 DAGCallback 接口，在子任务启动/完成/失败/跳过/分支选择时触发回调
- **图嵌套/子图**：支持将一个 Task 作为子图节点嵌入另一个 Task
- **优先级调度**：子任务支持设置优先级，调度时优先执行高优先级节点
- **图可视化增强**：支持输出 DOT/Mermaid 格式的图描述，方便可视化调试

## Capabilities

### New Capabilities
- `dag-channel`: DAG 通道管理，实现非破坏性的依赖状态跟踪（Waiting/Ready/Skipped/Completed），支持控制依赖和数据依赖的独立管理
- `dag-branch`: DAG 条件分支，支持运行时条件判断动态选择执行路径
- `dag-compile`: DAG 图编译，构建完成后进行环检测、类型校验，生成不可变可运行对象
- `dag-field-mapping`: DAG 字段映射，支持子任务间字段级别的数据传递和合并
- `dag-callback`: DAG 执行回调，支持子任务生命周期事件（启动/完成/失败/跳过/分支选择）的回调通知
- `dag-subgraph`: DAG 子图嵌套，支持将一个 Task 作为子图节点嵌入另一个 Task
- `task-timeout`: 子任务超时控制，支持独立超时配置和 Context 传播
- `task-rollback-strategy`: 回滚策略可配置，支持 All/Failed/Custom 三种策略
- `executor-provider`: 执行器协议抽象，通过 ExecutorProvider 统一接口支持本地函数、gRPC、HTTP、MCP 等多种执行器协议

### Modified Capabilities
- `task-model`: Task/Subtask 模型重构，自建 dagGraph 替代第三方库，增加节点触发模式、Skip 状态、泛型类型、优先级等字段，移除全局执行器注册
- `task-executor`: 执行器接口重构，SubTaskExecutor 增加 ctx 参数，执行器注册改为实例级，支持 Pre/Post Processor，通过 ExecutorProvider 抽象支持多协议

## Impact

- **核心代码**：taskx/task.go（全面重构）、taskx/executor.go（接口重构）、taskx/dispatch.go（调度逻辑适配）
- **新增文件**：taskx/dag.go（图结构）、taskx/channel.go（通道）、taskx/compile.go（编译）、taskx/branch.go（分支）、taskx/callback.go（回调）、taskx/executor/provider.go（执行器抽象）、taskx/executor/local.go（本地函数执行器）、taskx/executor/grpc.go（gRPC 执行器）、taskx/executor/http.go（HTTP 执行器）、taskx/executor/mcp.go（MCP 执行器）
- **数据模型**：dao/model 中的 Task/Subtask 模型需新增字段，涉及数据库迁移
- **API 变更**：**BREAKING** — 全面重构 API，不保留旧接口兼容
- **依赖**：移除 dominikbraun/graph 第三方库依赖
