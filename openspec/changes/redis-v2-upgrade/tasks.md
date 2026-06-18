## 1. Setup & Shared Infrastructure

- [x] 1.1 Create `redis/v2/` directory, add go-redis v9 dependency (`github.com/redis/go-redis/v9`)
- [x] 1.2 Extract shared `Config` to `redis/interface.go` (package `redis`), v1/v2 use type alias `type Config = xredis.Config`
- [x] 1.3 Extract shared metrics to `redis/metric.go`: `PoolStats`, `InitMetrics()`, `RegisterOrReuse()`, `RecordCommand()`, `RecordPipeline()`, `UpdatePoolStats()`, `DurationBuckets`

## 2. Core Client Implementation (Minimal Wrapper)

- [x] 2.1 Implement `NewRedisClient` factory: standalone mode (v9 `redis.Options`, `ConnMaxIdleTime`/`ConnMaxLifetime`), cluster mode (v9 `redis.ClusterOptions`), Ping, metrics hook registration
- [x] 2.2 Implement `Cmdable` interface: embeds `redis.Cmdable` + `Key(string) string` for key prefix
- [x] 2.3 Implement `RedisClient` interface: `Cmd() Cmdable`, `AddHook(redis.Hook)`, `Close()`
- [x] 2.4 Verify `go build ./redis/v2/` compiles without errors

## 3. Metrics Implementation

- [x] 3.1 Create `redis/v2/metric.go`: `init()` calls shared `xredis.InitMetrics()`
- [x] 3.2 Implement `MetricsHook` with v9's `DialHook`/`ProcessHook`/`ProcessPipelineHook` middleware pattern
- [x] 3.3 Implement `startPoolMetrics` + `collectPoolStats` with cluster `ForEachShard` aggregation
- [x] 3.4 Verify `go build ./redis/v2/` compiles without errors

## 4. ScriptManager

- [x] 4.1 Create `redis/v2/redis_script.go`: port v1 `ScriptManager` to v9 (`redis.Cmdable`, `redis.Cmd`, `redis.NewCmd`)
- [x] 4.2 Create `redis/v2/redis_script_test.go`: port v1 tests using miniredis + v9 client

## 5. Tests

- [x] 5.1 Create `redis/v2/metric_test.go`: unit tests (ProcessHook/Pipeline/DialHook) + integration tests with miniredis (Set/Get/Del/Pipeline/KeyPrefix/pool stats)
- [x] 5.2 Run `go test ./redis/v2/...` and verify all tests pass

## 6. Dependency & Cleanup

- [x] 6.1 Run `go mod tidy` to ensure `go.mod` and `go.sum` are clean with v9 dependency added
- [x] 6.2 Run `go vet ./redis/v2/...` and fix any warnings
- [x] 6.3 Verify v1 is untouched: `go test ./redis/v1/...` still passes

## 7. Non-v1 Package v8→v9 Upgrade

> `redis/v1/` 保留 v8（遗留封装），其余直接引用 v8 的包全部升级到 v9。

### 7.1 cluster
- [x] 7.1.1 `cluster/cluster_redis.go`: import `github.com/go-redis/redis/v8` → `github.com/redis/go-redis/v9`（仅用于 `redis.Nil`）

### 7.2 telemetry
- [x] 7.2.1 `telemetry/redisotel.go`: 重写 TracingHook 为 v9 的 `ProcessHook`/`ProcessPipelineHook` wrapper 模式，移除 `rediscmd/v8` 依赖，用 `cmd.String()` 替代

### 7.3 pkg/limiter
- [x] 7.3.1 `pkg/limiter/limiter.go`: import v8→v9，`redisv1.ScriptManager` → `redisv2.ScriptManager`
- [x] 7.3.2 `pkg/limiter/key_fixed_window.go`: import v8→v9，`redisv1.ScriptManager` → `redisv2.ScriptManager`
- [x] 7.3.3 `pkg/limiter/plan_limiter.go`: import v8→v9，`redisv1.ScriptManager` → `redisv2.ScriptManager`
- [x] 7.3.4 `pkg/limiter/key_fixed_window_test.go`: import v8→v9，`*redis.Client` 类型更新
- [x] 7.3.5 `pkg/limiter/limit_test.go`: import v8→v9，`*redis.Client` 类型更新
- [x] 7.3.6 `pkg/limiter/plan_limiter_test.go`: import v8→v9，`*redis.Client` 类型更新
- [x] 7.3.7 `pkg/limiter/miniredis_time_test.go`: import v8→v9，`*redis.Client` 类型更新

### 7.4 Verification
- [x] 7.4.1 `go mod tidy` 清理 `rediscmd/v8` 依赖
- [x] 7.4.2 `go vet ./...` 全量检查
- [x] 7.4.3 `go test ./pkg/limiter/... ./telemetry/...` 通过；`go build ./cluster/` 通过（cluster 测试超旹为 gRPC 已有问题，与本次 v8→v9 无关）
- [x] 7.4.4 `go test ./redis/v1/... ./redis/v2/...` 不受影响

## 8. Cluster redisv1 → redisv2 Migration

> cluster 包从 redisv1.RedisClient 接口迁移到 redisv2.RedisClient，使用 v9 原生命令。

- [x] 8.1 `redis/v2/client.go`: 新增 `GetRedis() redis.Cmdable` 方法，暴露原始客户端用于 SCAN 场景
- [x] 8.2 `cluster/cluster.go`: import `redisv1` → `redisv2`，字段类型 `redisv1.RedisClient` → `redisv2.RedisClient`
- [x] 8.3 `cluster/cluster_redis.go`: 移除 redisv1 import，所有 v1 API 调用替换为 v2 `Cmd()` + v9 原生命令
  - `SetExPeriod` → `Cmd().Set(ctx, Key(), v, ttl).Err()`
  - `GetString` → `Cmd().Get(ctx, Key()).Result()`
  - `SetNXPeriod` → `Cmd().SetNX(ctx, Key(), v, ttl).Result()`
  - `SetPeriod` → `Cmd().Set(ctx, Key(), v, ttl).Err()`
  - `Del` → `Cmd().Del(ctx, Key()).Err()`
  - `GetRedis()` SCAN 保持使用原始 redis.Cmdable（避免双重前缀）
  - `redisv1.ErrNil` 移除，仅检查 `redis.Nil`
- [x] 8.4 `cluster/cluster_redis_test.go`: redisv1 → redisv2，测试辅助函数和 GetString 调用迁移
- [x] 8.5 `cluster/cluster_test.go`: redisv1 → redisv2 import + 构造函数调用
- [x] 8.6 Verification: `go vet ./cluster/` 通过，`go build ./cluster/` 通过，Redis 相关测试通过
