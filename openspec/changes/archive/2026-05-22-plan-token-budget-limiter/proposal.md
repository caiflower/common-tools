## Why

现有的 `pkg/limiter` 包提供了令牌桶、固定窗口等限流算法，但都是基于请求速率（QPS）的限流，无法满足大模型 Coding Plan 场景的需求：限制用户在任意滑动时间窗口（如 8 小时）内的 Token 消耗总量。大模型场景下，每次请求消耗的 token 数不同，需要的是"预算制"而非"速率制"，且需要支持多模型分别统计、加权计价、预扣减/退款等 LLM 特有需求。系统需支撑日活 1000w+ 用户。

## What Changes

- 新增 `PlanLimiter` 类型，基于时间桶滑动窗口（Time-Bucketed Sliding Window）算法实现 Token 预算限流
- 新增 Lua 脚本 `plan_sliding_window.lua`，原子完成窗口内桶求和、预算判断、扣减、过期桶清理
- 支持多模型分别统计 token 消耗（Hash field 以模型名为 key）
- 支持模型加权计价（不同模型 token 价值不同，统一换算为加权单位后与预算比较）
- 支持 `Allow`（扣减）、`Refund`（退款）、`Usage`（查询剩余额度）三个核心操作
- 支持可配置桶粒度（默认 1 小时）和窗口时长（默认 8 小时）
- 新增集成测试，使用 `github.com/alicebob/miniredis/v2` 作为 Redis 模拟

## Capabilities

### New Capabilities
- `plan-token-budget`: 大模型 Plan Token 预算限流器，基于时间桶滑动窗口算法，支持多模型统计、加权计价、预扣减/退款

### Modified Capabilities

## Impact

- 新增文件：`pkg/limiter/plan_limiter.go`、`pkg/limiter/plan_limiter_test.go`、`pkg/limiter/lua/plan_sliding_window.lua`
- 新增依赖：`github.com/alicebob/miniredis/v2`（仅测试依赖）
- 不影响现有 `RedisLimiter`、`TokenBucket`、`FixedWindowLimiter` 等类型
- Redis 存储结构：每用户一个 Hash Key，1h 桶粒度下约 600 bytes/user，10M DAU 峰值约 5-8 GB
