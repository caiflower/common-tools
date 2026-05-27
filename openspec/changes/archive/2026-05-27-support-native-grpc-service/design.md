## Context

集群当前通过 `RegisterFunc` + `CallFuncAs` 提供远程调用能力，底层使用 `RemoteCall` gRPC 方法 + `google.protobuf.Any` 包装参数和结果。这种设计虽然灵活，但存在类型不安全、双重序列化性能损耗、无法利用 gRPC 生态（拦截器、流式 RPC、反射等）的问题。

集群已运行 gRPC server（`clusterServiceServer`），监听端口由 `config.ListenPort` 配置。所有节点通过 `grpcNodeClient` 互联。当前 gRPC server 仅注册了 `ClusterService`，未暴露给用户注册自定义服务的入口。

## Goals / Non-Goals

**Goals:**
- 允许用户在集群 gRPC server 上注册自定义 protobuf 定义的 gRPC 服务
- 允许用户通过集群获取任意节点的 `grpc.ClientConnInterface`，创建原生 gRPC 客户端 stub
- 保持与现有 `RegisterFunc` / `CallFuncAs` 完全兼容，不破坏已有 API
- 支持在集群启动前和运行时注册 gRPC 服务

**Non-Goals:**
- 不自动生成 protobuf 代码或客户端 stub（用户自行定义 .proto 并生成）
- 不实现 gRPC 服务发现/注册中心（用户通过节点名路由）
- 不替换现有 `RemoteCall` 机制（两种方式并存）
- 不支持跨集群 gRPC 调用（仅限同一集群内节点）

## Decisions

### D1: 通过 `RegisterGRPCService` 在集群 server 上注册服务

**选择**：提供 `RegisterGRPCService(sd *grpc.ServiceDesc, ss interface{})` 方法，将用户定义的 gRPC 服务注册到集群的 gRPC server 上。

**理由**：
- 集群已运行 gRPC server，复用同一端口和连接，无需额外监听端口
- 用户定义标准 protobuf 服务，生成标准 Go 代码，直接注册
- 与 gRPC 生态完全兼容（拦截器、反射、健康检查等）

**替代方案**：
- 独立 gRPC server：需要额外端口和连接管理，增加运维复杂度
- 通过 `RemoteCall` 代理：无法享受原生 gRPC 的类型安全和流式能力

### D2: 通过 `GetGRPCClient` 暴露节点连接

**选择**：提供 `GetGRPCClient(nodeName string) (grpc.ClientConnInterface, error)` 方法，返回指定节点的 `grpc.ClientConnInterface`。

**理由**：
- `grpc.ClientConnInterface` 是 gRPC 客户端 stub 的标准依赖，用户可直接 `proto.NewXxxClient(conn)` 创建客户端
- 复用集群已有的 gRPC 连接池，无需用户自行管理连接
- 支持任意 protobuf 生成的客户端类型

**替代方案**：
- 返回 `*grpc.ClientConn`：暴露过多内部细节，`grpc.ClientConnInterface` 更精简
- 返回地址字符串让用户自行创建连接：浪费已有连接，增加用户负担

### D3: 服务注册时机 — 启动前注册 + 运行时动态注册

**选择**：支持两种注册时机：
1. 集群 `Start()` 前注册：服务随 `ClusterService` 一起注册到 gRPC server
2. 集群运行时注册：通过 `grpc.Server.RegisterService()` 动态注册

**理由**：
- 启动前注册最简单，gRPC server 创建时一次性注册所有服务
- 运行时注册通过 `grpc.Server.RegisterService()` 实现，gRPC 原生支持动态注册

**注意**：gRPC 不支持运行时注销已注册的服务，这是 gRPC 本身的限制。

### D4: ICluster 接口扩展方式

**选择**：在 `ICluster` 接口新增方法，不创建新接口。

**理由**：
- 只有 2 个新方法，扩展幅度小
- 用户通常通过 `ICluster` 接口使用集群，新增方法保持一致性
- 如果创建子接口（如 `IGRPCCluster`），用户需要类型断言，增加使用复杂度

## Risks / Trade-offs

- **[端口共享]** 用户注册的 gRPC 服务与集群内部服务共享同一端口 → 需在文档中说明端口用途，建议使用有意义的 service name 避免冲突
- **[连接生命周期]** `GetGRPCClient` 返回的连接由集群管理，集群关闭时连接关闭 → 用户需在集群关闭前完成所有 gRPC 调用
- **[动态注册限制]** gRPC 运行时不支持注销服务 → 文档说明此限制，建议在 `Start()` 前完成所有注册
- **[向后兼容]** 新增方法不改变现有 API → 无风险，完全向后兼容
