## 1. Protobuf 定义与代码生成

- [x] 1.1 创建 `cluster/proto/cluster.proto` 文件，定义 `ClusterService` 服务和所有消息类型（AskLeader、AskVote、BroadcastLeader、Heartbeat、RemoteCall 的 Request/Response）
- [x] 1.2 添加 `protoc` 和 `protoc-gen-go`、`protoc-gen-go-grpc` 工具的 Makefile target，确保代码生成可复现
- [x] 1.3 执行 `protoc` 生成 `cluster.pb.go` 和 `cluster_grpc.pb.go`，确认 Go 编译通过
- [x] 1.4 验证生成的 gRPC 代码包含 `ClusterServiceServer`、`ClusterServiceClient` 接口及所有 RPC 方法

## 2. gRPC 传输层基础实现

- [x] 2.1 创建 `cluster/grpc_server.go`，实现 `ClusterServiceServer` 接口中的选举 RPC 方法（AskLeader、AskVote、BroadcastLeader），逻辑从 `getServerHandler()` 迁移
- [x] 2.2 在 `grpc_server.go` 中实现 `RemoteCall` RPC 方法，逻辑从 `getServerHandler()` 中 `messageRemoteCallReq` 处理迁移
- [x] 2.3 创建 `cluster/grpc_client.go`，封装 gRPC 客户端调用方法（AskLeader、AskVote、BroadcastLeader、RemoteCall），包含超时控制（context.WithTimeout）
- [x] 2.4 修改 `Node` 结构体，将 `connection nio.IClient` 替换为 `grpcConn *grpc.ClientConn` 和 `client ClusterServiceClient`，更新 `setConnection`、`close` 方法
- [x] 2.5 实现 Node 的 gRPC 连接建立逻辑，替换 `Node.sendMessage` 为各 RPC 方法的直接调用

## 3. Cluster 结构体适配

- [x] 3.1 修改 `Cluster` 结构体，将 `server nio.IServer` 替换为 `grpcServer *grpc.Server`
- [x] 3.2 重写 `Cluster.listen()` 方法，启动 gRPC Server 并注册 `ClusterService`
- [x] 3.3 重写 `Cluster.reconnect()` 方法，使用 `grpc.Dial`/`grpc.NewClient` 替代 `nio.NewClient`
- [x] 3.4 修改 `Cluster.Close()` 方法，调用 `grpcServer.GracefulStop()` 替代 `server.Close()`
- [x] 3.5 重写 `Cluster.sendMsgWhitTimeout()` 方法，使用 gRPC Unary RPC 调用替代 nio 消息发送，保持响应收集逻辑
- [x] 3.6 重写 `Cluster.callRemoteFunc()` 方法，使用 gRPC `RemoteCall` RPC 替代 nio 消息发送

## 4. 心跳双向流实现

- [x] 4.1 在 `grpc_server.go` 中实现 `Heartbeat` 双向流 RPC 的服务端逻辑，处理 Follower 收到心跳请求后的响应
- [x] 4.2 在 `grpc_client.go` 中实现心跳双向流的客户端逻辑，Leader 向 Follower 建立流并发送心跳
- [x] 4.3 实现心跳流的生命周期管理：Leader 签署后建立流，退位时关闭流
- [x] 4.4 实现心跳流断开检测：流关闭时更新节点存活状态，触发重连或重新选举
- [x] 4.5 修改 `Cluster.heartbeat()` 方法，使用双向流发送心跳替代当前的 `sendMsgWithBackoffTimeout` 调用
- [x] 4.6 确保心跳流与现有 `Node.updateHeartbeat()`、`updateHeartbeatFailed()`、`resetHeartbeatOnLeaderChange()` 方法兼容

## 5. cluster_contact.go 重构

- [x] 5.1 重写 `getClientHandler()` 中的 `OnSessionConnected` 逻辑，适配 gRPC 连接建立后的节点存活标记
- [x] 5.2 重写 `getClientHandler()` 中的 `OnSessionClosed` 逻辑，适配 gRPC 连接断开后的节点失联处理
- [x] 5.3 重写 `getClientHandler()` 中的 `OnMessageReceived` 逻辑，将 nio 消息分发替换为 gRPC RPC 响应处理
- [x] 5.4 重写 `getServerHandler()` 中的所有消息处理逻辑，已迁移到 `grpc_server.go` 的 RPC 实现中
- [x] 5.5 移除 `cluster_contact.go` 对 `nio` 包的依赖，或删除该文件将逻辑分散到 grpc_server/grpc_client

## 6. 远程调用适配

- [x] 6.1 修改 `remoteCallMessage` 结构体或创建 Protobuf 消息到 `FuncSpec` 的转换逻辑
- [x] 6.2 实现 `RemoteCallRequest` 中 `param` 字段的 `google.protobuf.Any` 序列化/反序列化（内部使用 JSON）
- [x] 6.3 实现 `RemoteCallResponse` 中 `result` 和 `err` 字段的转换逻辑
- [x] 6.4 确保远程调用的 `callCache` 匹配逻辑在 gRPC 同步调用模式下仍然正确工作

## 7. 测试适配与验证

- [x] 7.1 适配 `cluster_test.go`，确保现有测试用例在 gRPC 模式下通过
- [x] 7.2 适配 `remote_call_test.go`，验证远程调用在 gRPC 模式下功能正确
- [x] 7.3 适配 `election_performance_test.go`，添加 gRPC 模式的选举性能基准测试
- [x] 7.4 编写 gRPC 双向流心跳的集成测试，验证流建立、消息收发、流断开检测
- [x] 7.5 编写 gRPC 连接管理的测试，验证连接建立、失败重试、优雅关闭
- [x] 7.6 对比 gRPC 和 nio 模式下的选举耗时和心跳延迟基准数据
- [x] 7.7 创建 `cluster/remote_call_benchmark_test.go`，编写远程调用性能 benchmark 测试，覆盖同步/异步、小载荷(string)/中等载荷([]byte)/大载荷(struct)、本地调用/远程调用等维度
- [ ] 7.8 在 benchmark 中对比 gRPC 和 nio 两种传输层下远程调用的吞吐量（QPS）和延迟（P50/P95/P99），输出对比报告

## 8. 清理与文档

- [x] 8.1 移除 `cluster` 包中所有对 `pkg/nio` 的 import 引用
- [x] 8.2 移除 `cluster` 结构体中不再使用的 nio 相关字段（`server nio.IServer`）
- [x] 8.3 移除 `message.go` 中不再使用的消息类型常量（`messageAskLeaderReq` 等），保留 `Message` 结构体供内部使用
- [x] 8.4 更新 `cluster_contact.go` 或删除该文件，将逻辑整合到新的 gRPC 实现文件中
- [x] 8.5 添加 `proto` 代码生成的 Makefile target 和 README 说明
