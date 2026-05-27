## ADDED Requirements

### Requirement: ClusterService Protobuf 服务定义
系统 SHALL 定义一个名为 `ClusterService` 的 gRPC 服务，包含以下 RPC 方法：
- `AskLeader(AskLeaderRequest) returns (AskLeaderResponse)` — Unary RPC
- `AskVote(AskVoteRequest) returns (AskVoteResponse)` — Unary RPC
- `BroadcastLeader(BroadcastLeaderRequest) returns (BroadcastLeaderResponse)` — Unary RPC
- `Heartbeat(stream HeartbeatRequest) returns (stream HeartbeatResponse)` — Bidirectional Streaming RPC
- `RemoteCall(RemoteCallRequest) returns (RemoteCallResponse)` — Unary RPC

#### Scenario: Protobuf 文件编译成功
- **WHEN** 执行 `protoc` 编译 `cluster.proto` 文件
- **THEN** 生成 `cluster.pb.go` 和 `cluster_grpc.pb.go` 文件，且 Go 编译通过无错误

### Requirement: Raft 选举消息类型定义
系统 SHALL 定义以下 Protobuf 消息类型，覆盖现有 Raft 选举协议的所有字段：

**AskLeader**:
- `AskLeaderRequest`: node_name (string), term (int32)
- `AskLeaderResponse`: node_name (string), term (int32), leader_node_name (string), success (bool)

**AskVote**:
- `AskVoteRequest`: node_name (string), term (int32)
- `AskVoteResponse`: node_name (string), term (int32), vote_node_name (string), success (bool)

**BroadcastLeader**:
- `BroadcastLeaderRequest`: node_name (string), term (int32), leader_node_name (string)
- `BroadcastLeaderResponse`: node_name (string), term (int32), leader_node_name (string), success (bool)

#### Scenario: AskLeader 消息字段完整性
- **WHEN** 构造一个 `AskLeaderRequest` 并设置 node_name="node1", term=3
- **THEN** 序列化后再反序列化，所有字段值与原始值一致

#### Scenario: AskVote 消息字段完整性
- **WHEN** 构造一个 `AskVoteResponse` 并设置 node_name="node2", term=3, vote_node_name="node1", success=true
- **THEN** 序列化后再反序列化，所有字段值与原始值一致

### Requirement: 心跳消息类型定义
系统 SHALL 定义以下 Protobuf 消息类型：

**Heartbeat**:
- `HeartbeatRequest`: node_name (string), term (int32)
- `HeartbeatResponse`: node_name (string), term (int32), leader_node_name (string), success (bool)

#### Scenario: 心跳消息双向传输
- **WHEN** Leader 通过双向流发送 `HeartbeatRequest`（node_name="leader", term=5）
- **THEN** Follower 收到后返回 `HeartbeatResponse`（node_name="follower", term=5, success=true）

### Requirement: 远程调用消息类型定义
系统 SHALL 定义以下 Protobuf 消息类型：

**RemoteCall**:
- `RemoteCallRequest`: trace_id (string), uuid (string), func_name (string), param (google.protobuf.Any), sync (bool)
- `RemoteCallResponse`: trace_id (string), uuid (string), func_name (string), result (google.protobuf.Any), err (string)

#### Scenario: 远程调用请求序列化
- **WHEN** 构造一个 `RemoteCallRequest`，param 使用 `Any` 包装 JSON 数据
- **THEN** 序列化后再反序列化，param 的类型 URL 和字节内容可正确还原

#### Scenario: 远程调用错误返回
- **WHEN** 远程函数执行返回错误
- **THEN** `RemoteCallResponse.err` 字段 SHALL 包含错误信息的字符串表示
