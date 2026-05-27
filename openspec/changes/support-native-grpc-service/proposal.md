## Why

当前集群的远程调用机制 (`RegisterFunc` + `CallFuncAs`) 使用 `interface{}` 参数和 `google.protobuf.Any` 序列化，存在以下问题：

1. **类型不安全**：`RegisterFunc` 签名为 `func(interface{}) (interface{}, error)`，参数和返回值在编译期无类型检查，运行时反序列化失败是常见问题（这也是 `CallFuncAs[T]` 需要 fallback marshal/unmarshal 的原因）。
2. **性能损耗**：每次远程调用需要 `interface{}` → JSON → `protobuf.Any` → 网络传输 → `protobuf.Any` → JSON → `interface{}` 的双重序列化，而原生 gRPC 直接使用 protobuf 二进制编码，性能差距显著。
3. **无法利用 gRPC 生态**：原生 gRPC 方法支持拦截器（interceptor）、流式 RPC、健康检查、gRPC 反射等，当前 `RemoteCall` 通用 RPC 无法享受这些能力。

## What Changes

- 新增 `RegisterGRPCService(sd *grpc.ServiceDesc, ss interface{})` 方法，允许在集群 gRPC server 上注册任意 protobuf 定义的原生 gRPC 服务
- 新增 `GetGRPCClient(nodeName string) (grpc.ClientConnInterface, error)` 方法，返回指定节点的 gRPC 连接，用于创建原生 gRPC 客户端 stub
- 新增 `RegisterGRPCServiceDesc(sd grpc.ServiceDesc)` 方法，在所有节点上注册服务描述符，使客户端连接可以自动发现和路由
- 保留现有 `RegisterFunc` / `CallFuncAs` 不变，向后兼容

## Capabilities

### New Capabilities
- `native-grpc-service`: 支持在集群中注册和调用原生 gRPC 服务，包括服务端注册、客户端连接获取、节点路由

### Modified Capabilities
- `grpc-cluster-transport`: 扩展集群 ICluster 接口，新增原生 gRPC 服务注册和客户端连接获取方法

## Impact

- **ICluster 接口**：新增 2-3 个方法（`RegisterGRPCService`、`GetGRPCClient`），不影响现有方法
- **cluster.go**：新增 gRPC 服务注册表和客户端连接管理逻辑
- **grpc_server.go**：支持动态注册 gRPC 服务到已运行的 server
- **grpc_client.go**：暴露 `grpc.ClientConnInterface` 供外部创建原生 stub
- **proto/cluster.proto**：无需修改，原生 gRPC 服务由用户自行定义 proto
