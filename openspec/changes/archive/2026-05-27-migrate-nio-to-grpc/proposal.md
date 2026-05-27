## Why

当前集群模块基于自研的 `pkg/nio` TCP 网络框架实现节点间通信（Raft 选举、心跳、远程调用），该框架需要手动处理粘包/断包、编解码、连接管理和错误重试，性能不佳且维护成本高。项目已引入 `google.golang.org/grpc` 依赖，应利用 gRPC 的成熟特性（基于 HTTP/2 的多路复用、Protobuf 高效序列化、内置连接管理和健康检查、流式通信支持）替换自研协议，提升网络层性能和可维护性。

## What Changes

- **BREAKING**: 移除 `pkg/nio` 包对 `cluster` 包的依赖，将集群通信层从自研 NIO TCP 协议迁移到 gRPC
- 定义 gRPC Protobuf 服务契约，覆盖现有 Raft 协议消息（AskLeader、AskVote、BroadcastLeader、Heartbeat）和远程调用（RemoteCall）
- 用 gRPC Server/Client 替换 `nio.IServer`/`nio.IClient`，重写 `cluster_contact.go` 中的 Handler 逻辑
- 重构 `Node` 结构体，将 `nio.IClient` 连接替换为 gRPC Client 连接
- 利用 gRPC 双向流（Bidirectional Streaming）优化心跳和选举的消息交互模式，减少连接数和延迟
- 保留 `pkg/nio` 包本身不删除（其他模块可能使用），仅解除 `cluster` 包对它的依赖
- 保留 `modeRedis` 和 `modeSingle` 模式不受影响，仅改造 `modeCluster` 模式

## Capabilities

### New Capabilities
- `grpc-cluster-protocol`: 定义集群节点间 gRPC 通信的 Protobuf 服务契约，包括 Raft 选举 RPC（AskLeader、AskVote、BroadcastLeader、Heartbeat）和远程调用 RPC（RemoteCall），以及对应的消息类型定义
- `grpc-cluster-transport`: 基于 gRPC 实现集群网络传输层，替换 nio 的 Server/Client/Handler/Session 模型，包括 gRPC 服务端启动、客户端连接管理、连接池、重连机制
- `grpc-bidirectional-stream`: 利用 gRPC 双向流实现心跳和选举消息的高效交互，替代当前的请求-响应模式，减少网络往返和连接开销

### Modified Capabilities
- `cluster-election-performance`: 选举性能将因 gRPC 的多路复用和流式通信而改善，选举超时和重试策略需要适配 gRPC 语义

## Impact

- **代码变更**: `cluster/cluster_contact.go`（核心重写）、`cluster/node.go`（连接模型变更）、`cluster/message.go`（消息类型适配）、`cluster/cluster.go`（listen/reconnect/heartbeat/fighting 方法适配）、`cluster/remote_call.go`（远程调用适配）
- **新增文件**: Protobuf 定义文件（`cluster/proto/cluster.proto`）、生成的 Go 代码（`cluster/proto/cluster.pb.go`、`cluster/proto/cluster_grpc.pb.go`）
- **依赖变更**: 新增 `google.golang.org/grpc` 已在 go.mod 中；新增 `google.golang.org/protobuf` 已在 go.mod 中；可能需要新增 `github.com/golang/protobuf` 工具链
- **API 影响**: `ICluster` 接口不变，外部调用方无感知；`Node` 结构体内部字段变更但不暴露
- **配置影响**: 节点地址格式可能需要适配 gRPC（如 `ip:port` 不变，但连接方式改变）
- **测试影响**: `cluster_test.go`、`remote_call_test.go`、`election_performance_test.go` 需要适配 gRPC 环境
