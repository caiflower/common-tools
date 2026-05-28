## Context

`pkg/limiter` 包已包含多种限流实现：
- `FixedWindowLimiter`：基于 Go channel 的全局固定窗口并发控制，不支持按 key 隔离
- `RedisLimiter`：基于 Redis + Lua 的令牌桶限流，支持按 key 但语义是速率限制（rate limiting），非并发控制
- `PlanLimiter`：基于 Redis + Lua 的滑动窗口预算限流，面向 token 消耗配额管理
- `XTokenBucket`：基于 `golang.org/x/time/rate` 的本地令牌桶

大模型对话场景需要：每个用户（key）同时进行的对话连接数有上限，连接开始时 Acquire，连接结束时 Release。这是一个**并发数控制**问题，而非速率限制问题。现有的 `FixedWindowLimiter` 只能做全局并发控制，`RedisLimiter` 的令牌桶语义不适合并发生命周期管理。

需要一个新的基于 Redis 的按 key 固定窗口并发限流器，核心语义是 Acquire/Release 而非 TakeToken。

## Goals / Non-Goals

**Goals:**
- 实现按 key 的固定窗口并发限流，支持分布式环境下的原子操作
- 提供 Acquire（获取并发槽位）和 Release（释放并发槽位）语义
- 支持查询当前 key 的并发数
- 遵循包内已有的代码模式（RedisLimiter、PlanLimiter 的 Option 模式、Lua 脚本管理）
- 保证并发安全，防止超限

**Non-Goals:**
- 不实现滑动窗口或令牌桶算法（已有 RedisLimiter 和 PlanLimiter）
- 不替换现有的 FixedWindowLimiter
- 不实现等待/阻塞获取（仅提供非阻塞的 Acquire）
- 不实现 TTL 自动过期窗口（并发槽位由业务方显式 Release，但提供兜底过期防止泄漏）

## Decisions

### 1. 数据结构：Redis Hash + 计数器

**选择**：使用 Redis String 存储每个 key 的当前并发数（`key_fixed_window:{userKey}` -> count）

**理由**：
- 并发计数器只需一个整数值，无需 Hash 的多字段
- INCR/DECR 操作天然原子性
- 简单直观，性能最优

**备选方案**：
- Redis Hash（类似 PlanLimiter）：过度设计，并发计数器无需多字段
- Redis Set（存储每个连接 ID）：更精确但复杂度高，需要生成唯一连接 ID，且 Set 操作不如 INCR 高效

### 2. Acquire 语义：Lua 脚本原子操作

**选择**：使用 Lua 脚本实现 Acquire，先读取当前计数，判断是否超限，未超限则 INCR

**理由**：
- 读取 + 判断 + 写入必须在同一原子操作中完成
- 虽然单次 INCR 是原子的，但"判断是否超限"需要先 GET，两步操作之间可能被其他请求插入
- Lua 脚本保证原子性，与包内已有模式一致

### 3. Release 语义：Lua 脚本防止下溢

**选择**：使用 Lua 脚本实现 Release，DECR 但不低于 0

**理由**：
- 直接 DECR 可能导致计数器变为负数（如网络重传导致重复 Release）
- Lua 脚本中判断当前值 > 0 才 DECR，否则保持 0

### 4. 兜底过期：EXPIRE 防止泄漏

**选择**：每次 Acquire 时设置 EXPIRE，默认 24 小时

**理由**：
- 如果业务方因异常未调用 Release（进程崩溃、网络断开），计数器不会永久占用
- 过期时间应远大于正常对话连接的持续时间
- 可通过 Option 配置

### 5. 接口设计

**选择**：定义 `KeyFixedWindowLimiterInterface` 接口

```go
type KeyFixedWindowLimiterInterface interface {
    Acquire(ctx context.Context, key string, opts ...KeyFixedWindowOption) (bool, error)
    Release(ctx context.Context, key string, opts ...KeyFixedWindowOption) error
    CurrentConcurrent(ctx context.Context, key string, opts ...KeyFixedWindowOption) (int64, error)
}
```

**理由**：
- 与 `RedisLimiterInterface` 和 `PlanLimiterInterface` 模式一致
- Acquire/Release 语义清晰，区别于 TakeToken
- CurrentConcurrent 提供可观测性

### 6. Option 模式

**选择**：两层 Option（构造时默认值 + 调用时覆盖）

- `KeyFixedWindowOption`：构造时配置（默认最大并发数、默认过期时间）
- `KeyFixedWindowCallOption`：单次调用覆盖（覆盖最大并发数）

**理由**：与 `LimiterOption`/`Option`、`PlanOption` 模式完全一致

## Risks / Trade-offs

- **[连接泄漏]** → 如果业务方忘记 Release，并发槽位会被永久占用。通过 EXPIRE 兜底缓解，但过期前窗口内该 key 的并发数会被错误占用。建议业务方使用 defer Release 模式。
- **[重复 Release]** → 网络重传或业务逻辑错误可能导致同一连接被 Release 多次。Lua 脚本中已做下溢保护（不低于 0），但会导致并发计数比实际少 1。建议业务方确保 Acquire/Release 一一对应。
- **[时钟偏移]** → Redis 单节点无时钟问题；集群模式下各节点时钟可能有微小偏差，影响 EXPIRE 精度，但对并发控制影响可忽略。
- **[miniredis 测试局限]** → miniredis 是单线程模拟，无法完全验证高并发下的原子性，生产环境需补充真实 Redis 集成测试。
