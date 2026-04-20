# Redis Client Prometheus Metrics — Spec

## Overview

通过 `redis.Hook` 拦截器机制，在 Redis 客户端命令执行的前后注入 Prometheus 指标采集逻辑，支持单机模式（`redis.Client`）和集群模式（`redis.ClusterClient`）。

新文件：`redis/v1/metric.go`，实现 `redis.Hook` 接口。

---

## Goals

- 通过 Hook（拦截器）采集 Redis 命令级别的 Prometheus 指标，无需修改业务代码
- 支持按命令类型（command）、地址（addr）、状态（status: ok / error）分维度统计
- 与现有 `kafka/metrics.go`、`web/common/metric/metric.go` 保持风格一致
- 在 `NewRedisClient` 中自动注册 Hook，无需用户手动调用

---

## Non-Goals

- 不采集 Redis Server 侧指标（INFO 命令）
- 不支持 OpenTelemetry Metrics（已有 `telemetry/redisotel.go` 处理 Tracing）
- 不做指标开关配置，默认始终开启

---

## 主要指标采集项

### 1. `redis_commands_total` — Counter

| 字段 | 值 |
|---|---|
| 类型 | CounterVec |
| 说明 | Redis 命令执行总次数 |
| Labels | `addr`, `command`, `status` (ok \| error) |
| 触发时机 | `AfterProcess` / `AfterProcessPipeline` |

```
redis_commands_total{addr="127.0.0.1:6379", command="get", status="ok"} 1024
redis_commands_total{addr="127.0.0.1:6379", command="set", status="error"} 3
```

---

### 2. `redis_command_duration_seconds` — Histogram

| 字段 | 值 |
|---|---|
| 类型 | HistogramVec |
| 说明 | Redis 命令执行耗时（秒） |
| Labels | `addr`, `command` |
| Buckets | `0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1` |
| 触发时机 | `BeforeProcess` 记录开始时间，`AfterProcess` 计算差值 |

```
redis_command_duration_seconds_bucket{addr="127.0.0.1:6379", command="get", le="0.01"} 900
redis_command_duration_seconds_sum{addr="127.0.0.1:6379", command="get"} 8.5
redis_command_duration_seconds_count{addr="127.0.0.1:6379", command="get"} 1024
```

---

### 3. `redis_pipeline_commands_total` — Counter

| 字段 | 值 |
|---|---|
| 类型 | CounterVec |
| 说明 | Pipeline 中命令执行总次数（批量命令数之和） |
| Labels | `addr`, `status` (ok \| error) |
| 触发时机 | `AfterProcessPipeline`，`Add(len(cmds))` |

```
redis_pipeline_commands_total{addr="127.0.0.1:6379", status="ok"} 256
```

---

### 4. `redis_pipeline_duration_seconds` — Histogram

| 字段 | 值 |
|---|---|
| 类型 | HistogramVec |
| 说明 | Pipeline 整批命令执行耗时（秒） |
| Labels | `addr` |
| Buckets | 同 `redis_command_duration_seconds` |
| 触发时机 | `BeforeProcessPipeline` 记录开始，`AfterProcessPipeline` 记录差值 |

```
redis_pipeline_duration_seconds_bucket{addr="127.0.0.1:6379", le="0.05"} 120
```

---

### 5. `redis_pool_idle_conns` — Gauge

| 字段 | 值 |
|---|---|
| 类型 | GaugeVec |
| 说明 | 连接池当前空闲连接数 |
| Labels | `addr` |
| 触发时机 | 后台 goroutine 定期（10s）轮询 `PoolStats()` |

---

### 6. `redis_pool_total_conns` — Gauge

| 字段 | 值 |
|---|---|
| 类型 | GaugeVec |
| 说明 | 连接池总连接数（含空闲 + 使用中） |
| Labels | `addr` |
| 触发时机 | 后台 goroutine 定期（10s）轮询 `PoolStats()` |

---

### 7. `redis_pool_stale_conns_total` — Counter

| 字段 | 值 |
|---|---|
| 类型 | CounterVec |
| 说明 | 因超时或失效被淘汰的连接总数（累计增量） |
| Labels | `addr` |
| 触发时机 | 后台 goroutine 定期（10s）轮询 `PoolStats().StaleConns`，上报增量 |

---

## ConstLabels

所有指标均附带以下 ConstLabels（与项目其他 metrics 文件一致）：

```go
prometheus.Labels{
    "ip":        env.GetLocalHostIP(),
    "namespace": env.GetNamespace(), // 非空时才加
    "app":       env.GetApp(),       // 非空时才加
}
```

---

## 实现设计

### Hook 结构

```go
// redis/v1/metric.go
type MetricsHook struct {
    addr string // 单机: config.Addrs[0]; 集群: "cluster"
}

func (h *MetricsHook) BeforeProcess(ctx context.Context, cmd redis.Cmder) (context.Context, error)
func (h *MetricsHook) AfterProcess(ctx context.Context, cmd redis.Cmder) error
func (h *MetricsHook) BeforeProcessPipeline(ctx context.Context, cmds []redis.Cmder) (context.Context, error)
func (h *MetricsHook) AfterProcessPipeline(ctx context.Context, cmds []redis.Cmder) error
```

开始时间通过 `context.WithValue` 传递，key 用包内私有类型避免碰撞。

### 注册时机

在 `NewRedisClient` 末尾、Ping 之后，自动调用：

```go
hook := newMetricsHook(config)
c.AddHook(hook)
startPoolMetrics(ctx, c, config) // 启动后台连接池轮询
```

### 连接池轮询（单机 vs 集群）

- 单机：`c.client.PoolStats()` 直接获取
- 集群：遍历 `c.clusterClient.ForEachShard(...)` 汇总各 shard 的 `PoolStats()`

---

## 文件清单

| 文件 | 说明 |
|---|---|
| `redis/v1/metric.go` | Hook 实现 + 指标定义 + 连接池轮询 |

无需新增其他文件，`client.go` 仅在 `NewRedisClient` 末尾增加 Hook 注册。