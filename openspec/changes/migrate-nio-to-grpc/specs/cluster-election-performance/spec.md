## MODIFIED Requirements

### Requirement: 选举超时与重试策略适配 gRPC 语义
选举过程中的超时和重试策略 SHALL 适配 gRPC 的调用语义。当前的 `sendMsgWhitTimeout` 使用 `context.WithTimeout` 控制超时，迁移到 gRPC 后 SHALL 使用 gRPC 的 `CallOption`（如 `grpc.Timeout`）或 `context.Context` 超时来控制 RPC 超时。

#### Scenario: AskLeader RPC 超时控制
- **WHEN** 发起 AskLeader RPC 调用并设置 500ms 超时
- **THEN** 如果在 500ms 内未收到响应，RPC 返回 `DeadlineExceeded` 错误，该节点的响应不计入投票

#### Scenario: AskVote RPC 超时控制
- **WHEN** 发起 AskVote RPC 调用并设置 500ms 超时
- **THEN** 如果在 500ms 内未收到响应，RPC 返回 `DeadlineExceeded` 错误，该节点的投票不计入

#### Scenario: BroadcastLeader RPC 超时控制
- **WHEN** 发起 BroadcastLeader RPC 调用并设置 1000ms 超时
- **THEN** 如果在 1000ms 内未收到响应，RPC 返回 `DeadlineExceeded` 错误，该节点的确认不计入

### Requirement: 选举性能基准测试
系统 SHALL 提供选举性能基准测试，对比 gRPC 和 nio 两种实现下的选举耗时和资源消耗。

#### Scenario: gRPC 模式选举耗时基准
- **WHEN** 在 3 节点集群中触发选举
- **THEN** 选举完成时间 SHALL 不超过 nio 模式下的 1.5 倍（考虑到 gRPC 的额外开销）

#### Scenario: gRPC 模式心跳吞吐量基准
- **WHEN** Leader 向 2 个 Follower 发送心跳
- **THEN** 心跳往返延迟 SHALL 低于 nio 模式下的延迟（得益于 HTTP/2 多路复用和双向流）

## ADDED Requirements

### Requirement: 远程调用性能基准测试
系统 SHALL 提供远程调用性能 benchmark 测试（`BenchmarkRemoteCall*`），覆盖以下维度：
- 调用模式：同步调用（`CallFuncAs`）、异步调用（`NewAsyncFuncSpec`）
- 载荷大小：小载荷（string，约 10 字节）、中等载荷（[]byte，约 1KB）、大载荷（struct，约 10KB）
- 调用目标：本地调用（同节点）、远程调用（跨节点）

#### Scenario: 同步远程调用小载荷 benchmark
- **WHEN** 执行 `BenchmarkRemoteCallSyncSmallPayload` benchmark，使用 string 类型参数进行同步远程调用
- **THEN** 输出每次调用耗时（ns/op）和内存分配（B/op、allocs/op），可对比 gRPC 与 nio 模式

#### Scenario: 同步远程调用中等载荷 benchmark
- **WHEN** 执行 `BenchmarkRemoteCallSyncMediumPayload` benchmark，使用 1KB []byte 参数进行同步远程调用
- **THEN** 输出每次调用耗时和内存分配，可对比 gRPC 与 nio 模式

#### Scenario: 同步远程调用大载荷 benchmark
- **WHEN** 执行 `BenchmarkRemoteCallSyncLargePayload` benchmark，使用约 10KB struct 参数进行同步远程调用
- **THEN** 输出每次调用耗时和内存分配，可对比 gRPC 与 nio 模式

#### Scenario: 异步远程调用 benchmark
- **WHEN** 执行 `BenchmarkRemoteCallAsync` benchmark，使用异步模式进行远程调用
- **THEN** 输出每次调用耗时和内存分配，可对比 gRPC 与 nio 模式

#### Scenario: 本地调用 benchmark
- **WHEN** 执行 `BenchmarkLocalCall` benchmark，在同节点内调用已注册函数
- **THEN** 输出每次调用耗时和内存分配，作为基线参考

### Requirement: 远程调用 gRPC vs nio 性能对比
系统 SHALL 在 benchmark 测试中输出 gRPC 和 nio 两种传输层下远程调用的性能对比报告，包含吞吐量（QPS）和延迟分位数（P50/P95/P99）。

#### Scenario: gRPC 模式远程调用延迟不高于 nio 模式的 1.5 倍
- **WHEN** 在 3 节点集群中执行同步远程调用 benchmark
- **THEN** gRPC 模式下单次调用 P50 延迟 SHALL 不超过 nio 模式 P50 延迟的 1.5 倍

#### Scenario: gRPC 模式远程调用吞吐量不低于 nio 模式的 80%
- **WHEN** 在 3 节点集群中执行同步远程调用 benchmark
- **THEN** gRPC 模式下 QPS SHALL 不低于 nio 模式 QPS 的 80%

#### Scenario: 大载荷场景下 gRPC 优于 nio
- **WHEN** 在 3 节点集群中使用 10KB 以上载荷执行远程调用 benchmark
- **THEN** gRPC 模式（Protobuf 序列化）的序列化/反序列化耗时 SHALL 低于 nio 模式（JSON + Gzip）
