# Cluster Election Performance Optimization — Spec

## Overview

当前 Raft 选举在 3/10/30/100 节点规模下，首次选主耗时均约 5~6s，故障转移约 8~10s。根本原因是：选举流程中的超时等待、随机延迟、心跳周期均过长，且 follower 检测 leader 失联依赖被动等待。

本 spec 描述优化目标、瓶颈定位及改动范围。

---

## 当前性能基线

| 规模  | 首个 leader (ms) | 全员共识 (ms) | 故障转移 (ms) |
|------|-----------------|------------|------------|
| 3    | 6159            | 0          | 9891       |
| 10   | 6051            | 0          | 9801       |
| 30   | 5963            | 0          | 10286      |
| 100  | 4796            | 0          | 7635       |

---

## Goals

- 首次选主耗时 < 1500ms（3/10/30 节点），100 节点 < 3000ms
- 故障转移耗时 < 3000ms（3/10/30 节点），100 节点 < 5000ms
- 不改变 Raft 选举的正确性（多数派原则、任期保证、一票制）
- 不引入新的外部依赖

## Non-Goals

- 不优化 Redis 模式 / Single 模式的选举路径
- 不改变 `sendMsgWhitTimeout` 的底层 nio 传输机制
- 不做 pre-vote 轮次优化

---

## 瓶颈定位

### 瓶颈 1：`sendMsgWhitTimeout` 硬等 2s 超时

```
// cluster.go:759, 862, 852
messages := c.sendMsgWhitTimeout(2*time.Second, ...)
```

`sendMsgWhitTimeout` 在收满所有存活节点响应前最多等 2s。但在本地测试环境中，所有响应都在几十毫秒内到达，导致每次 RPC 轮次都浪费约 2s。

**选主一次完整路径含 3 次 RPC 轮次：**
1. `messageAskLeaderReq`（2s）
2. `messageAskVoteReq`（2s）
3. `messageBroadcastLeaderReq`（2s）

即使所有响应瞬时返回，总耗时仍最少 ≈ 2s（因为第一次必须等满，当节点未就绪时响应 success=false，循环继续）。

### 瓶颈 2：`fighting()` 随机延迟过大

```go
// cluster.go:808
time.Sleep(time.Duration(rand.Intn(200)) * time.Millisecond)
```

在开始拉票前等待 0~200ms，作用是防止分票（split vote）。此随机范围过大，应缩小。

### 瓶颈 3：查询 leader 阶段的轮询延迟

```go
// cluster.go:801
time.Sleep(time.Duration(rand.Intn(100)) * time.Millisecond)
```

在查询是否已有 leader 的循环中，每轮等 0~100ms，而且该阶段在节点均未就绪时会多次重试才 `break`。

### 瓶颈 4：`sendMsgWhitTimeout` 响应收集使用 200ms 轮询 ticker

```go
// cluster.go:1075
ticker := time.NewTicker(200 * time.Millisecond)
```

收到所有响应后仍在等下一个 ticker 触发才返回，增加不必要的延迟。

### 瓶颈 5：follower 心跳检测周期过长导致故障转移慢

```go
// cluster.go:900
ticker := time.NewTicker(c.calculateHeartbeatInterval())
// calculateHeartbeatInterval = config.Timeout / 3 = 5s / 3 ≈ 1.67s
```

follower 通过心跳 ticker 检测 leader 是否失联。`config.Timeout` 默认 5s，心跳间隔 ≈ 1.67s。
leader 宕机后，follower 最多等 1.67s 才开始 fighting，加上 fighting 中 2s 的 RPC 超时，总计 ≈ 4~10s。

---

## 改动方案

### 改动 1：`sendMsgWhitTimeout` 提前退出 + 缩短 ticker 间隔

**文件：`cluster.go`，函数 `sendMsgWhitTimeout`**

- 收满所有 aliveNode 响应后立即返回，无需等 ticker
- ticker 间隔从 200ms 缩短至 10ms（仅作为超时检查 fallback）

```go
// 改前
ticker := time.NewTicker(200 * time.Millisecond)

// 改后
ticker := time.NewTicker(10 * time.Millisecond)
```

同时在 `case m := <-c.msgChan` 中收满后立即 `return`（当前逻辑已有，但 ticker case 中也要检查）。

### 改动 2：RPC 超时从 2s 缩短至 500ms

**文件：`cluster.go`，`fightingWithRetry` 中三处 `sendMsgWhitTimeout` 调用**

```go
// 改前
messages := c.sendMsgWhitTimeout(2*time.Second, ...)

// 改后
messages := c.sendMsgWhitTimeout(500*time.Millisecond, ...)
```

理由：本地 TCP 通信延迟 < 1ms，节点无响应即视为不可达，500ms 足够等待慢节点。心跳 RPC 超时同理。

### 改动 3：缩小 fighting 随机延迟范围

**文件：`cluster.go`，`fightingWithRetry`**

```go
// 改前（防止 split vote 的随机延迟）
time.Sleep(time.Duration(rand.Intn(200)) * time.Millisecond)

// 改后
time.Sleep(time.Duration(rand.Intn(50)) * time.Millisecond)
```

50ms 的随机范围足以防止 split vote（概率与 200ms 类似，因为竞争窗口相同）。

### 改动 4：缩短查询 leader 阶段的轮询间隔

**文件：`cluster.go`，`fightingWithRetry` askLeader 循环**

```go
// 改前
time.Sleep(time.Duration(rand.Intn(100)) * time.Millisecond)

// 改后
time.Sleep(time.Duration(rand.Intn(20)) * time.Millisecond)
```

### 改动 5：加快 follower 心跳检测周期

**文件：`cluster.go`，`calculateHeartbeatInterval`**

```go
// 改前
baseInterval := c.config.Timeout / 3  // 5s/3 ≈ 1.67s

// 改后：leader 心跳间隔固定 300ms；follower 检测间隔 500ms
```

具体方案：将 leader 发送心跳间隔（`calculateHeartbeatInterval`）改为固定 300ms，follower 超时检测间隔改为 500ms。这样 leader 宕机后 follower 最迟 500ms 感知，加上 500ms RPC，故障转移总耗时可降至 1~2s。

**注意**：`config.Timeout` 同时控制节点心跳有效期（`node.isReady`），保持默认 5s 不变，仅调整主动发送/检测的间隔。

---

## 改动文件清单

| 文件 | 改动说明 |
|------|---------|
| `cluster/cluster.go` | 5 处参数/逻辑修改，不新增函数 |

---

## 预期性能

| 规模 | 首个 leader (目标) | 故障转移 (目标) |
|------|-----------------|--------------|
| 3    | < 500ms         | < 1500ms     |
| 10   | < 800ms         | < 2000ms     |
| 30   | < 1200ms        | < 2500ms     |
| 100  | < 2000ms        | < 4000ms     |

---

## 风险与兼容性

- **split vote 概率**：随机延迟从 200ms 缩短到 50ms 不增加 split vote 风险，因为 Raft 的 split vote 概率取决于多个节点是否在同一时间窗口启动竞选，而非绝对延迟大小。实际测试为验证指标。
- **网络抖动容忍**：500ms RPC 超时对局域网足够；跨 AZ 或高延迟网络建议通过 `Config.Timeout` 动态调整，此处不做修改。
- **向后兼容**：只修改内部常量与计算逻辑，Config 结构体和公共接口不变。