## Context

当前 `cluster` 包基于自研 `pkg/nio` TCP 框架实现节点间通信。`pkg/nio` 提供了 Server、Client、Handler、Session、Codec 五层抽象，使用自定义二进制协议（4字节长度 + 1字节flag + gzip压缩body）进行消息传输。集群通信包含两类协议：

1. **Raft 选举协议**：AskLeader、AskVote、BroadcastLeader、Heartbeat（各有 Req/Res），采用请求-响应模式，通过 `sendMsgWhitTimeout` 广播并收集响应
2. **远程调用协议**：RemoteCall（Req/Res），支持同步/异步调用，通过 `callCache` 匹配请求与响应

当前架构痛点：
- 手动粘包/断包处理，Codec 实现复杂且脆弱
- 每个节点对之间仅一条 TCP 连接，无法多路复用
- 无连接池，重连逻辑分散在 `reconnect()` 中
- 错误处理和重试逻辑需手动实现
- 心跳和选举使用独立的请求-响应模式，网络往返开销大

项目 `go.mod` 已包含 `google.golang.org/grpc v1.77.0` 和 `google.golang.org/protobuf v1.36.11`。

## Goals / Non-Goals

**Goals:**
- 将 `modeCluster` 模式下的网络层从 `pkg/nio` 迁移到 gRPC，提升性能和可维护性
- 定义清晰的 Protobuf 服务契约，替代自定义二进制协议
- 利用 gRPC 双向流优化心跳和选举消息交互，减少网络往返
- 保持 `ICluster` 接口不变，对外部调用方透明
- 保持 `modeRedis` 和 `modeSingle` 模式不受影响
- 保持与现有配置格式兼容（节点地址 `ip:port` 格式不变）

**Non-Goals:**
- 不删除 `pkg/nio` 包（其他模块可能仍在使用）
- 不修改 Raft 选举算法逻辑本身（任期、投票、法定人数等规则不变）
- 不修改 `modeRedis` 模式的实现
- 不引入 gRPC 拦截器/中间件链（可后续迭代）
- 不修改 `JobTracker`、`Scheduler` 等上层调度逻辑
- 不引入 etcd/consul 等外部服务发现组件

## Decisions

### Decision 1: gRPC 服务模型选择 — Unary + Bidirectional Streaming 混合

**选择**: Raft 选举协议使用 Unary RPC（一元调用），心跳使用 Bidirectional Streaming RPC，远程调用使用 Unary RPC。

**理由**:
- Raft 选举消息（AskLeader、AskVote、BroadcastLeader）天然是请求-响应模式，Unary RPC 语义最匹配
- 心跳是周期性双向交互，使用双向流可以在一条流上持续交换心跳消息，避免每次心跳都建立新请求，减少网络开销
- 远程调用是典型的请求-响应模式，Unary RPC 最合适

**替代方案**:
- 全部使用 Unary RPC：简单但心跳性能不佳，每次心跳都需要完整的请求-响应周期
- 全部使用双向流：过于复杂，选举和远程调用不需要流式语义
- 使用 Server Streaming：单向流不适合需要双向交互的心跳场景

### Decision 2: Protobuf 服务定义 — 单一服务 vs 多服务

**选择**: 定义一个 `ClusterService`，包含所有 RPC 方法。

```protobuf
service ClusterService {
  // Raft 选举协议 - Unary RPC
  rpc AskLeader(AskLeaderRequest) returns (AskLeaderResponse);
  rpc AskVote(AskVoteRequest) returns (AskVoteResponse);
  rpc BroadcastLeader(BroadcastLeaderRequest) returns (BroadcastLeaderResponse);

  // 心跳协议 - Bidirectional Streaming
  rpc Heartbeat(stream HeartbeatRequest) returns (stream HeartbeatResponse);

  // 远程调用 - Unary RPC
  rpc RemoteCall(RemoteCallRequest) returns (RemoteCallResponse);
}
```

**理由**: 集群通信是一个内聚的领域，所有 RPC 都服务于同一个 Raft 协议，放在一个服务中更清晰。单一服务也简化了服务端注册和客户端存根管理。

**替代方案**:
- 拆分为 `ElectionService`、`HeartbeatService`、`RemoteCallService`：过度设计，增加连接管理复杂度
- 拆分为 `RaftService` 和 `RemoteCallService`：可考虑，但当前规模不需要

### Decision 3: 连接管理 — 每个 Node 持有一个 gRPC Client 连接

**选择**: 每个 `Node` 持有一个 `grpc.ClientConn` 和对应的 `ClusterServiceClient` 存根，心跳流在连接上建立。

**理由**:
- gRPC 的 `ClientConn` 本身支持多路复用和连接池，一个连接可以同时发起多个 RPC
- 心跳双向流复用同一连接，无需额外连接
- 与当前架构（每个 Node 持有一个 `nio.IClient`）对齐，迁移改动最小

**替代方案**:
- 连接池（多个 `ClientConn`）：gRPC 内部已有 HTTP/2 多路复用，无需应用层连接池
- 全局共享连接管理器：增加复杂度，当前节点规模不大

### Decision 4: 心跳双向流的生命周期管理

**选择**: Leader 节点向每个 Follower 发起双向流，流的生命周期与 Leader 任期绑定。Leader 退位时关闭所有心跳流，Follower 检测到流关闭后触发重新选举。

**理由**:
- Leader 发起流可以避免 Follower 需要知道 Leader 地址的问题
- 流与任期绑定，确保旧 Leader 不会继续发送心跳
- 流关闭自然触发 Follower 的"心跳超时"检测，替代当前的 `updateHeartbeatFailed` 逻辑

**替代方案**:
- Follower 发起流：需要 Follower 知道 Leader 地址，增加复杂度
- 独立的心跳连接：浪费资源，不如复用 gRPC 连接

### Decision 5: 消息序列化 — Protobuf 替代 JSON + Gzip

**选择**: 使用 Protobuf 作为消息序列化格式，替代当前的 JSON + Gzip 方案。

**理由**:
- Protobuf 二进制编码比 JSON 更紧凑，无需额外的 Gzip 压缩层
- Protobuf 有强类型定义，编译时检查，减少运行时序列化错误
- gRPC 原生支持 Protobuf，无需额外编解码逻辑

**替代方案**:
- 继续使用 JSON：失去 gRPC 的类型安全和性能优势
- 使用 MessagePack：不如 Protobuf 与 gRPC 的集成度高

### Decision 6: 远程调用的参数和结果序列化

**选择**: 远程调用的 `param` 和 `result` 使用 `google.protobuf.Any` 包装，内部仍然使用 JSON 序列化以保持与现有 `RegisterFunc` 接口的兼容性。

**理由**:
- 现有 `RegisterFunc(funcName string, fn func(data interface{}) (interface{}, error))` 接口使用 `interface{}`，无法映射到 Protobuf 强类型
- 使用 `Any` 类型可以在 Protobuf 层面保持灵活性，内部 JSON 序列化保持向后兼容
- 后续可以逐步迁移到强类型的 Protobuf 消息

**替代方案**:
- 使用 `bytes` 字段直接传输 JSON：功能等价，但 `Any` 提供了类型 URL 元信息，便于后续迁移
- 为每个远程调用定义独立的 Protobuf 消息：改动太大，破坏现有 API

## Risks / Trade-offs

- **[风险] gRPC 双向流调试困难** → 使用 gRPC 内置的 tracing 和日志拦截器辅助调试；保留关键日志输出
- **[风险] 心跳流断开可能导致误判节点失联** → 实现流自动重连机制，区分"流断开"和"节点失联"；流断开后先尝试重连，重连失败再标记失联
- **[风险] Protobuf 代码生成增加构建复杂度** → 将生成的代码提交到仓库（常见做法），避免 CI 环境需要安装 protoc；提供 Makefile target 自动生成
- **[风险] 远程调用使用 `Any` + JSON 的性能不如纯 Protobuf** → 这是过渡方案，后续可逐步迁移到强类型；当前远程调用不是性能瓶颈
- **[权衡] 单一 gRPC 服务可能导致服务定义文件过大** → 当前 RPC 方法数量有限（5个），可维护性可接受；如果后续扩展可拆分
- **[风险] gRPC 与现有 nio 端口冲突** → 迁移期间可以使用不同端口，确认 nio 完全移除后再复用原端口；或通过配置项指定 gRPC 端口

## Migration Plan

1. **Phase 1 - Protobuf 定义与代码生成**: 创建 `cluster/proto/cluster.proto`，生成 Go 代码，确保编译通过
2. **Phase 2 - gRPC 传输层实现**: 实现 `ClusterServiceServer` 和客户端存根封装，不替换现有逻辑
3. **Phase 3 - 集成替换**: 修改 `Cluster` 结构体，用 gRPC Server/Client 替换 nio Server/Client，重写 `cluster_contact.go`
4. **Phase 4 - 心跳流优化**: 实现双向流心跳，替换当前的 Unary 心跳模式
5. **Phase 5 - 测试与验证**: 适配现有测试，增加 gRPC 特有的集成测试，性能基准测试对比
6. **Phase 6 - 清理**: 移除 `cluster` 包对 `pkg/nio` 的 import，更新文档

**回滚策略**: 保留 nio 相关代码直到 Phase 5 验证通过，通过 feature flag（配置项）控制使用 gRPC 还是 nio，出问题时可快速回退。

## Open Questions

- 心跳双向流是否需要支持多个并发流（同一对节点间），还是严格一个流？
- gRPC 服务端是否需要启用 TLS，还是先使用 insecure 连接（当前 nio 也是明文 TCP）？
- 是否需要支持 gRPC 健康检查协议（`grpc.health.v1.Health`）？
