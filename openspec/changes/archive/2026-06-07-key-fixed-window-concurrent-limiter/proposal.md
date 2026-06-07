## Why

当前 `pkg/limiter` 包中的 `FixedWindowLimiter` 仅支持全局并发控制（基于 channel），无法按 key（如用户 ID）维度进行独立的并发限制。在大模型对话场景中，需要限制每个用户同时进行的对话连接数（最大并发数），现有实现无法满足这一需求。需要一个基于 Redis 的按 key 固定窗口并发限流器，支持分布式环境下的原子操作，并提供 Acquire/Release 语义来管理并发连接的生命周期。

## What Changes

- 新增 `KeyFixedWindowLimiter` 结构体，基于 Redis 实现按 key 的固定窗口并发限流
- 新增 Lua 脚本 `key_fixed_window_acquire.lua`：原子性地获取一个并发槽位（当前并发数 +1，不超过上限则允许）
- 新增 Lua 脚本 `key_fixed_window_release.lua`：原子性地释放一个并发槽位（当前并发数 -1）
- 新增 `KeyFixedWindowLimiterInterface` 接口，定义 `Acquire`、`Release`、`CurrentConcurrent` 方法
- 新增 `KeyFixedWindowConfig` 配置结构及对应的 Option 模式
- 新增完整的单元测试，覆盖基本获取/释放、并发安全、多 key 隔离、超限拒绝等场景

## Capabilities

### New Capabilities
- `key-fixed-window-concurrent`: 按 key 固定窗口并发限流能力，支持基于 Redis 的原子 Acquire/Release 操作，用于控制每个 key 的最大并发连接数

### Modified Capabilities

## Impact

- 新增文件：`pkg/limiter/key_fixed_window.go`、`pkg/limiter/lua/key_fixed_window_acquire.lua`、`pkg/limiter/lua/key_fixed_window_release.lua`、`pkg/limiter/key_fixed_window_test.go`
- 依赖：`github.com/go-redis/redis/v8`、`github.com/caiflower/common-tools/redis/v1`（已有依赖）
- 不影响现有 `FixedWindowLimiter`、`RedisLimiter`、`PlanLimiter` 等代码
