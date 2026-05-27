## MODIFIED Requirements

### Requirement: gRPC 客户端连接管理
系统 SHALL 为每个远程 `Node` 创建并管理一个 `grpc.ClientConn` 和对应的 `ClusterServiceClient` 存根。连接管理 SHALL 替代原有的 `nio.IClient`。系统 SHALL 支持通过 `GetGRPCClient(nodeName string)` 方法暴露节点的 `grpc.ClientConnInterface`，允许用户创建原生 gRPC 客户端 stub。

#### Scenario: 节点连接建立
- **WHEN** 调用 `Cluster.reconnect()` 且发现某节点不在 `aliveNodes` 中
- **THEN** 为该节点创建 `grpc.ClientConn`，并生成 `ClusterServiceClient` 存根存入 `Node` 结构体

#### Scenario: 节点连接失败处理
- **WHEN** 创建 `grpc.ClientConn` 失败（如目标节点不可达）
- **THEN** 记录错误日志，不将该节点加入 `aliveNodes`，下次 `reconnect()` 时重试

#### Scenario: 节点连接关闭
- **WHEN** 调用 `Node.close()` 或集群关闭
- **THEN** 关闭该节点的 `grpc.ClientConn`，清理相关资源

#### Scenario: 获取存活节点的 gRPC 连接
- **WHEN** 调用 `GetGRPCClient(nodeName)` 且节点在 `aliveNodes` 中且有活跃 gRPC 连接
- **THEN** 返回该节点的 `grpc.ClientConnInterface`，用户可基于此创建任意 gRPC 客户端 stub

#### Scenario: 获取未知节点的 gRPC 连接
- **WHEN** 调用 `GetGRPCClient(nodeName)` 且节点名不存在
- **THEN** 返回 `nil` 和错误，指示节点不存在

#### Scenario: 获取不可达节点的 gRPC 连接
- **WHEN** 调用 `GetGRPCClient(nodeName)` 且节点存在但无活跃 gRPC 连接
- **THEN** 返回 `nil` 和错误，指示节点不可达

### Requirement: gRPC 服务端启动与管理
系统 SHALL 在 `Cluster.Start()` 中启动 gRPC Server，监听当前节点配置的地址（ip:port），并注册 `ClusterService` 服务实现。gRPC Server SHALL 存储在 `Cluster` 结构体中，替代原有的 `nio.IServer`。系统 SHALL 支持通过 `RegisterGRPCService(sd *grpc.ServiceDesc, ss interface{})` 方法在 gRPC Server 上注册用户自定义的 gRPC 服务。

#### Scenario: 集群启动时 gRPC 服务端正常监听
- **WHEN** 调用 `Cluster.Start()` 且 `modeCluster` 模式下
- **THEN** gRPC Server 在配置的地址上启动监听，且 `ClusterService` 和所有预注册的用户服务已注册

#### Scenario: 集群关闭时 gRPC 服务端优雅停止
- **WHEN** 调用 `Cluster.Close()`
- **THEN** gRPC Server 执行 `GracefulStop()`，等待进行中的 RPC 完成后关闭

#### Scenario: 启动前注册用户 gRPC 服务
- **WHEN** 在 `Start()` 前调用 `RegisterGRPCService(sd, impl)`
- **THEN** 服务随 `ClusterService` 一起注册到 gRPC server，客户端可通过同一端口访问

#### Scenario: 运行时动态注册用户 gRPC 服务
- **WHEN** 在 `Start()` 后调用 `RegisterGRPCService(sd, impl)`
- **THEN** 通过 `grpc.Server.RegisterService()` 动态注册，客户端可立即访问

#### Scenario: 注册重复服务名
- **WHEN** 调用 `RegisterGRPCService` 注册已存在的服务名
- **THEN** 返回错误，指示服务已注册
