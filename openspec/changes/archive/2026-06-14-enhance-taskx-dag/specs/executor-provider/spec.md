## NEW Requirements

### Requirement: ExecutorProvider 执行器协议抽象（泛型化）
系统 SHALL 提供 `ExecutorProvider` 接口，将子任务执行器抽象为统一接口，支持本地函数、gRPC、HTTP、MCP 等多种执行协议。各协议实现使用泛型替代 `interface{}` 以提供编译时类型安全。

#### Scenario: 泛型本地函数执行器
- **GIVEN** 用户通过 `NewLocalExecutor[I, O](fn)` 注册一个泛型本地函数执行器
- **WHEN** 子任务被调度执行
- **THEN** 系统从 TaskData 反序列化输入到 I 类型，调用 `fn(ctx, input)` 得到 O 类型输出
- **AND** 函数签名为 `func(ctx context.Context, input I) (O, error)`，无需 `interface{}` 类型断言
- **AND** `Protocol()` 返回 `ProtocolLocal`

#### Scenario: 泛型 gRPC 执行器
- **GIVEN** 用户通过 `NewGRPCExecutor[I, O](endpoint, serviceName, methodName)` 创建 gRPC 执行器
- **WHEN** 子任务被调度执行
- **THEN** 系统从 TaskData 反序列化输入到 I 类型，序列化后通过 gRPC 调用远程服务，反序列化响应到 O 类型
- **AND** `Protocol()` 返回 `ProtocolGRPC`

#### Scenario: 泛型 HTTP 执行器
- **GIVEN** 用户通过 `NewHTTPExecutor[I, O](url, method)` 创建 HTTP 执行器
- **WHEN** 子任务被调度执行
- **THEN** 系统从 TaskData 反序列化输入到 I 类型，序列化为 JSON 发起 HTTP 请求，反序列化响应到 O 类型
- **AND** `Protocol()` 返回 `ProtocolHTTP`

#### Scenario: MCP 执行器（非泛型）
- **GIVEN** 用户通过 `NewMCPExecutor(serverURL, toolName)` 创建 MCP 执行器
- **WHEN** 子任务被调度执行
- **THEN** 系统将 TaskData.Input 作为 JSON 参数，构造 MCP tools/call 请求，返回 `map[string]any` 类型结果
- **AND** `Protocol()` 返回 `ProtocolMCP`
- **AND** MCP 不使用泛型，因为其输入输出由 JSON Schema 运行时动态定义

### Requirement: 序列化约束（集群框架）
所有执行器的泛型参数 I/O SHALL 可 JSON 序列化，因为 taskx 是集群框架，数据需通过网络传输和持久化。

#### Scenario: 本地函数输入输出可序列化
- **GIVEN** 用户注册 `NewLocalExecutor[MyInput, MyOutput](fn)`
- **WHEN** MyInput 或 MyOutput 不可 JSON 序列化
- **THEN** 执行时反序列化/序列化失败，子任务标记为 Failed

#### Scenario: 集群间数据传输
- **GIVEN** 调度器将子任务分配到远程 Worker 节点
- **WHEN** Worker 节点接收到序列化的 TaskData
- **THEN** ExecutorProvider 从 TaskData 反序列化输入，执行后序列化输出存储到数据库

#### Scenario: TaskData 反序列化到泛型类型
- **GIVEN** 一个 `LocalExecutor[I, O]` 执行器
- **WHEN** 调用 `data.UnmarshalInput(&input)` 将 TaskData.Input 反序列化到 I 类型
- **THEN** I 类型实例被正确构造，传递给类型安全的函数

### Requirement: ExecutorProvider 注册 API
系统 SHALL 支持通过 `RegisterProviders` 注册 `ExecutorProvider`，同时保持对旧 `SubTaskExecutor` 的向后兼容。

#### Scenario: 注册泛型 ExecutorProvider
- **GIVEN** 用户创建一个 Task 实例
- **WHEN** 用户调用 `task.RegisterProviders(executor, map[string]ExecutorProvider{...})`
- **THEN** 系统将 ExecutorProvider 注册到该 Task 实例的 executorManager

#### Scenario: 向后兼容 SubTaskExecutor 注册
- **GIVEN** 用户使用旧的 `task.RegisterTaskExecutor(executor, map[string]SubTaskExecutor{...})` 方式注册
- **WHEN** 注册完成
- **THEN** 系统自动将每个 SubTaskExecutor 包装为 `legacyExecutorWrapper`，实现 ExecutorProvider 接口
- **AND** `legacyExecutorWrapper.Protocol()` 返回 `ProtocolLocal`
- **AND** `legacyExecutorWrapper.Execute()` 委托给原始 SubTaskExecutor 函数

#### Scenario: 获取 ExecutorProvider 优先级
- **GIVEN** executorManager 中同时存在 subtaskProviders 和 subtaskExecutors
- **WHEN** 调用 `getProvider(taskName, subTaskName)`
- **THEN** 优先返回 subtaskProviders 中的 ExecutorProvider
- **AND** 若不存在，则从 subtaskExecutors 中查找并包装为 legacyExecutorWrapper 返回

### Requirement: ExecutorProtocol 协议类型
系统 SHALL 定义 `ExecutorProtocol` 类型，标识执行器的底层协议。

#### Scenario: 协议类型定义
- **GIVEN** 系统定义了四种协议类型
- **THEN** `ProtocolLocal` = "local"，`ProtocolGRPC` = "grpc"，`ProtocolHTTP` = "http"，`ProtocolMCP` = "mcp"

#### Scenario: ExecutorProvider 返回协议类型
- **GIVEN** 任意 ExecutorProvider 实现
- **WHEN** 调用 `Protocol()` 方法
- **THEN** 返回该执行器对应的 ExecutorProtocol 值

### Requirement: 远程执行器超时与错误处理
远程执行器（gRPC/HTTP/MCP）SHALL 支持独立的超时配置，并区分网络错误和业务错误。

#### Scenario: 远程执行器超时
- **GIVEN** 一个 GRPCExecutor[I, O] 配置了 10 秒超时
- **WHEN** gRPC 调用超过 10 秒未返回
- **THEN** 执行器返回超时错误，子任务标记为 Failed

#### Scenario: 网络错误可重试
- **GIVEN** 一个 HTTPExecutor[I, O] 执行时遇到网络连接错误
- **WHEN** 错误为临时性网络错误（连接拒绝、超时等）
- **THEN** 系统可根据子任务的重试配置进行重试

#### Scenario: 业务错误不重试
- **GIVEN** 一个 GRPCExecutor[I, O] 执行时远程服务返回业务错误码
- **WHEN** 错误为非临时性业务错误
- **THEN** 子任务标记为 Failed，不进行重试

### Requirement: 自定义 ExecutorProvider 扩展
系统 SHALL 支持用户自定义实现 `ExecutorProvider` 接口。

#### Scenario: 自定义协议执行器
- **GIVEN** 用户实现了 `ExecutorProvider` 接口的自定义类型（含 Execute 和 Protocol 方法）
- **WHEN** 用户通过 `RegisterProviders` 注册该自定义执行器
- **THEN** 系统正常调度执行，调用其 `Execute` 方法

#### Scenario: Functional Options 模式配置
- **GIVEN** 各执行器支持 Functional Options 模式（GRPCOption、HTTPOption、MCPOption）
- **WHEN** 用户创建执行器时传入可选配置
- **THEN** 超时、headers、dial options 等配置被正确应用
