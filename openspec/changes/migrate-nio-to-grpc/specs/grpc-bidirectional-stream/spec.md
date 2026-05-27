## ADDED Requirements

### Requirement: 心跳双向流建立
Leader 节点 SHALL 向每个 Follower 节点发起 `Heartbeat` 双向流 RPC。流的建立 SHALL 在 Leader 签署后自动触发，替代当前的心跳定时器发送逻辑。

#### Scenario: Leader 上任后建立心跳流
- **WHEN** 节点成为 Leader
- **THEN** 向所有存活 Follower 节点发起 `Heartbeat` 双向流，每个 Follower 一条流

#### Scenario: 新节点加入后建立心跳流
- **WHEN** Leader 检测到新的存活节点
- **THEN** 向该节点发起 `Heartbeat` 双向流

### Requirement: 心跳双向流消息发送
Leader SHALL 通过已建立的双向流定期发送 `HeartbeatRequest`，Follower 收到后返回 `HeartbeatResponse`。发送间隔 SHALL 与当前配置的 `leaderHeartbeatInterval` 一致。

#### Scenario: Leader 定期发送心跳
- **WHEN** Leader 的心跳定时器触发
- **THEN** 通过双向流向所有 Follower 发送 `HeartbeatRequest`（包含 node_name 和 term）

#### Scenario: Follower 响应心跳
- **WHEN** Follower 通过双向流收到 `HeartbeatRequest`
- **THEN** 返回 `HeartbeatResponse`（包含 node_name、term、leader_node_name、success）

#### Scenario: Follower 拒绝非 Leader 心跳
- **WHEN** Follower 收到的心跳请求中 node_name 与当前 Leader 名称不匹配
- **THEN** 返回 `HeartbeatResponse`，success=false

### Requirement: 心跳双向流断开检测
系统 SHALL 检测双向流的断开，并据此更新节点存活状态。

#### Scenario: 流正常关闭时标记节点失联
- **WHEN** Leader 到某 Follower 的心跳流被正常关闭
- **THEN** 将该 Follower 从 `aliveNodes` 中移除

#### Scenario: 流异常断开时触发重连
- **WHEN** 心跳流因网络错误断开
- **THEN** 记录警告日志，尝试重新建立流；如果重连失败则标记节点失联

#### Scenario: Follower 检测到 Leader 流断开
- **WHEN** Follower 检测到来自 Leader 的心跳流断开
- **THEN** 更新心跳失败状态，如果连续失败超过阈值则触发重新选举

### Requirement: 心跳双向流与 Leader 任期绑定
心跳流 SHALL 与 Leader 的任期绑定。Leader 退位时 SHALL 关闭所有心跳流。

#### Scenario: Leader 退位关闭所有流
- **WHEN** Leader 释放 leader 身份（`releaseLeader`）
- **THEN** 关闭所有到 Follower 的心跳双向流

#### Scenario: 旧 Leader 的流被 Follower 拒绝
- **WHEN** Follower 收到 term 小于自身 term 的心跳请求
- **THEN** 返回 `HeartbeatResponse`，success=false，term 设置为自身 term

### Requirement: 心跳流与现有心跳超时逻辑兼容
双向流的心跳检测 SHALL 与现有的 `Node.updateHeartbeat()`、`Node.updateHeartbeatFailed()`、`Node.isReady()` 等方法兼容，确保上层逻辑无需修改。

#### Scenario: 心跳成功时更新节点状态
- **WHEN** Leader 收到 Follower 的成功心跳响应
- **THEN** 调用该 Follower 节点的 `updateHeartbeat()` 方法

#### Scenario: 心跳失败时更新节点状态
- **WHEN** Leader 收到 Follower 的失败心跳响应
- **THEN** 调用该 Follower 节点的 `updateHeartbeatFailed()` 方法

#### Scenario: Leader 变更时重置心跳状态
- **WHEN** Follower 收到来自新 Leader 的心跳（leader_node_name 与之前不同）
- **THEN** 调用 `resetHeartbeatOnLeaderChange()` 方法
