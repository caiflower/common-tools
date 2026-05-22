## Context

`pkg/limiter` 包已提供令牌桶（`RedisLimiter`/`TokenBucket`）、固定窗口（`FixedWindowLimiter`）等限流实现，均基于请求速率（QPS）维度。大模型 Coding Plan 场景需要的是 Token 总量预算控制：用户在任意滑动时间窗口（如 8 小时）内的加权 Token 消耗不得超过 Plan 预算。现有实现无法满足此需求。

关键约束：
- 日活 1000w+ 用户，Redis 存储需控制在可接受范围
- 不同 LLM 模型 token 价值不同，需加权计价
- LLM 场景需支持预扣减和退款（估算→实际修正）
- 需支持分模型统计 token 消耗

## Goals / Non-Goals

**Goals:**
- 实现基于时间桶滑动窗口的 Plan Token 预算限流器
- 保证任意滑动时间窗口内 token 消耗不超过预算
- 支持多模型分别统计和加权计价
- 支持 Allow（扣减）、Refund（退款）、Usage（查询）三个核心操作
- 单次 Redis 调用完成所有操作（EVALSHA）
- 10M DAU 下 Redis 内存占用可控（1h 桶粒度约 5-8 GB 峰值）
- 完整的集成测试覆盖（使用 miniredis）

**Non-Goals:**
- 不实现精确到每个请求的滑动窗口日志（存储成本不可接受）
- 不实现分布式限流协调（单 Redis Cluster 即可）
- 不实现 Plan 的创建/管理/计费（只负责限流判断）
- 不实现本地缓存降级（可作为后续优化）

## Decisions

### Decision 1: 时间桶滑动窗口 vs 固定窗口 vs Sorted Set 滑动日志

**选择：时间桶滑动窗口**

| 方案 | 任意窗口精确 | 存储/10M DAU | 复杂度 |
|------|------------|-------------|--------|
| 固定窗口 | ❌ 边界双花 | ~1.2 GB | 低 |
| 时间桶滑动窗口 | ✅ 桶粒度内精确 | ~1.5-2 GB | 中 |
| Sorted Set 滑动日志 | ✅ 完全精确 | ~40 GB | 高 |

理由：固定窗口无法保证任意滑动窗口不超限；Sorted Set 存储不可接受；时间桶是精确度和存储的最优平衡。

### Decision 2: 单 Hash Key + 多 field vs 多 Key

**选择：单 Hash Key**

一个 `plan:{userId}:{planId}` Hash Key 内存储所有桶数据和模型统计数据。

理由：
- 单次 EVALSHA 原子完成预算判断 + 扣减，无需跨 Key 事务
- Hash listpack 编码下内存开销最小
- 模型统计用 `_model` 前缀字段与桶时间戳字段区分

### Decision 3: 桶粒度默认 1 小时

8 小时窗口 / 1 小时桶 = 最多 8 个桶字段。边界误差 ≤ 1 小时内的消耗量，对 Plan 场景可接受。用户可自定义桶粒度。

### Decision 4: 加权计价模型

`total = Σ(各桶原始 token × 模型权重)`，预算以加权单位设定。模型权重通过 Option 传入，默认 1.0。

### Decision 5: Lua 脚本设计

三个 Lua 脚本：
- `plan_sliding_window.lua`：Allow 操作，求和窗口内桶 → 判断预算 → 扣减 → 清理过期桶
- `plan_refund.lua`：Refund 操作，退回指定模型的 token → 更新桶和模型统计
- `plan_usage.lua`：Usage 操作，只读查询当前窗口消耗和剩余额度

### Decision 6: Redis Key 过期策略

TTL = window + 3600 秒（窗口时长 + 1 小时缓冲），确保窗口滑动后 Key 自动清理。

## Risks / Trade-offs

- [桶粒度精度] → 1 小时桶粒度下，滑动窗口求和与真实值最大偏差为 1 个桶内的消耗量。Plan 场景下可接受，如需更精确可减小桶粒度（代价是更多 field 和略高内存）
- [HGETALL 性能] → 每次操作需 HGETALL 读取所有 field。1h 桶粒度下最多 8+10=18 个 field，listpack 编码下极快。如模型数暴增导致 field 过多，可拆分为独立 Key
- [Lua 脚本 HDEL 过期桶] → 每次 Allow 调用会清理过期桶，但 HDEL 是 O(n) 操作。实际 n 很小（最多清理 1-2 个过期桶），不影响性能
- [退款超限] → Refund 不检查是否退回超过实际消耗，由调用方保证正确性。Lua 脚本中做简单保护（total 和 modelUsed 不低于 0）
