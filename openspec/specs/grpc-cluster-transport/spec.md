## ADDED Requirements

### Requirement: gRPC 服务端启动与管理
系统 SHALL 在 `Cluster.Start()` 中启动 gRPC Server，监听当前节点配置的地址（ip:port），并注册 `ClusterService` 服务实现。gRPC Server SHALL 存储在 `Cluster` 结构体中，替代原有的 `nio.IServer`。

#### Scenario: 集群启动时 gRPC 服务端正常监听
- **WHEN** 调用 `Cluster.Start()` 且 `modeCluster` 模式下
- **THEN** gRPC Server 在配置的地址上启动监听，且 `ClusterService` 已注册

#### Scenario: 集群关闭时 gRPC 服务端优雅停止
- **WHEN** 调用 `Cluster.Close()`
- **THEN** gRPC Server 执行 `GracefulStop()`，等待进行中的 RPC 完成后关闭

### Requirement: gRPC 客户端连接管理
系统 SHALL 为每个远程 `Node` 创建并管理一个 `grpc.ClientConn` 和对应的 `ClusterServiceClient` 存根。连接管理 SHALL 替代原有的 `nio.IClient`。

#### Scenario: 节点连接建立
- **WHEN** 调用 `Cluster.reconnect()` 且发现某节点不在 `aliveNodes` 中
- **THEN** 为该节点创建 `grpc.ClientConn`，并生成 `ClusterServiceClient` 存根存入 `Node` 结构体

#### Scenario: 节点连接失败处理
- **WHEN** 创建 `grpc.ClientConn` 失败（如目标节点不可达）
- **THEN** 记录错误日志，不将该节点加入 `aliveNodes`，下次 `reconnect()` 时重试

#### Scenario: 节点连接关闭
- **WHEN** 调用 `Node.close()` 或集群关闭
- **THEN** 关闭该节点的 `grpc.ClientConn`，清理相关资源

### Requirement: Node 结构体适配 gRPC
`Node` 结构体 SHALL 将 `connection nio.IClient` 字段替换为 gRPC 相关字段，包括 `grpcConn *grpc.ClientConn` 和 `client ClusterServiceClient`（生成的 gRPC 客户端存根接口）。`Node.sendMessage` 方法 SHALL 被替换为对应的 gRPC RPC 调用方法。

#### Scenario: Node 发送 AskLeader 请求
- **WHEN** 调用 Node 的 AskLeader RPC 方法
- **THEN** 通过 gRPC 客户端存根发送 `AskLeaderRequest`，返回 `AskLeaderResponse`

#### Scenario: Node 连接未就绪时调用 RPC
- **WHEN** Node 的 gRPC 客户端存根为 nil 时调用 RPC
- **THEN** 返回明确的错误信息，指示连接未就绪

### Requirement: Raft 选举协议的 gRPC 服务端实现
系统 SHALL 实现 `ClusterServiceServer` 接口中的选举相关 RPC 方法（AskLeader、AskVote、BroadcastLeader），逻辑与现有 `getServerHandler()` 中的处理一致。

#### Scenario: AskLeader RPC 处理
- **WHEN** 收到 `AskLeaderRequest`，且集群已就绪且请求 term <= 当前 term
- **THEN** 返回 `AskLeaderResponse`，包含当前 leader 名称和 term，success=true

#### Scenario: AskVote RPC 处理
- **WHEN** 收到 `AskVoteRequest`，且集群未就绪或请求 term > 当前 term
- **THEN** 释放当前 leader，投票给请求节点，返回 success=true

#### Scenario: BroadcastLeader RPC 处理
- **WHEN** 收到 `BroadcastLeaderRequest`，且请求的 leader 节点存在于已知节点中
- **THEN** 签署该 leader，返回 success=true

### Requirement: 远程调用的 gRPC 服务端实现
系统 SHALL 实现 `ClusterServiceServer` 接口中的 `RemoteCall` RPC 方法，逻辑与现有 `getServerHandler()` 中 `messageRemoteCallReq` 的处理一致。

#### Scenario: 远程调用成功执行
- **WHEN** 收到 `RemoteCallRequest`，且 func_name 对应的本地函数已注册
- **THEN** 执行本地函数，返回 `RemoteCallResponse` 包含执行结果

#### Scenario: 远程调用函数不存在
- **WHEN** 收到 `RemoteCallRequest`，但 func_name 对应的本地函数未注册
- **THEN** 返回 `RemoteCallResponse`，err 字段包含"not such function"错误信息

### Requirement: Raft 选举协议的 gRPC 客户端调用
系统 SHALL 将 `sendMsgWhitTimeout` 中的 nio 消息发送替换为 gRPC Unary RPC 调用，保持相同的超时和响应收集逻辑。

#### Scenario: 广播 AskLeader 并收集响应
- **WHEN** 调用 `sendMsgWhitTimeout` 发送 AskLeader 请求
- **THEN** 并行向所有存活节点发起 `AskLeader` gRPC 调用，在超时时间内收集响应

#### Scenario: 广播 AskVote 并收集投票
- **WHEN** 调用 `sendMsgWhitTimeout` 发送 AskVote 请求
- **THEN** 并行向所有存活节点发起 `AskVote` gRPC 调用，在超时时间内收集投票结果

### Requirement: 远程调用的 gRPC 客户端调用
系统 SHALL 将 `callRemoteFunc` 中的 nio 消息发送替换为 gRPC `RemoteCall` Unary RPC 调用。

#### Scenario: 同步远程调用
- **WHEN** 发起同步远程调用（`FuncSpec.sync=true`）
- **THEN** 通过 gRPC `RemoteCall` RPC 发送请求，等待响应返回后设置结果

#### Scenario: 异步远程调用
- **WHEN** 发起异步远程调用（`FuncSpec.sync=false`）
- **THEN** 通过 gRPC `RemoteCall` RPC 发送请求，不等待响应，通过回调设置结果

### Requirement: modeRedis 和 modeSingle 模式不受影响
迁移 SHALL 仅影响 `modeCluster` 模式的网络层。`modeRedis` 和 `modeSingle` 模式 SHALL 不引入任何 gRPC 依赖或代码变更。

#### Scenario: Redis 模式启动不使用 gRPC
- **WHEN** 配置 `mode=redis` 启动集群
- **THEN** 不启动 gRPC Server，不创建 gRPC Client，使用现有的 Redis 选举机制

#### Scenario: Single 模式启动不使用 gRPC
- **WHEN** 配置 `mode=single` 启动集群
- **THEN** 不启动 gRPC Server，直接签署自身为 leader
