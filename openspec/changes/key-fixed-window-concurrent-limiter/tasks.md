## 1. Lua 脚本

- [x] 1.1 创建 `pkg/limiter/lua/key_fixed_window_acquire.lua`：原子性获取并发槽位（读取当前计数，判断是否超限，未超限则 INCR 并设置 EXPIRE）
- [x] 1.2 创建 `pkg/limiter/lua/key_fixed_window_release.lua`：原子性释放并发槽位（读取当前计数，大于 0 则 DECR，不低于 0）

## 2. Go 核心实现

- [x] 2.1 创建 `pkg/limiter/key_fixed_window.go`：定义 `KeyFixedWindowConfig`、`KeyFixedWindowOption`、`KeyFixedWindowCallOption` 配置结构
- [x] 2.2 实现 `KeyFixedWindowLimiterInterface` 接口定义（Acquire、Release、CurrentConcurrent）
- [x] 2.3 实现 `NewKeyFixedWindowLimiter` 构造函数（初始化 Redis 客户端、注册 Lua 脚本、加载脚本、校验配置）
- [x] 2.4 实现 `Acquire` 方法（调用 acquire Lua 脚本，解析返回结果）
- [x] 2.5 实现 `Release` 方法（调用 release Lua 脚本）
- [x] 2.6 实现 `CurrentConcurrent` 方法（读取 Redis key 的当前值）
- [x] 2.7 实现配置校验函数 `validateKeyFixedWindowConfig`

## 3. 单元测试

- [x] 3.1 创建 `pkg/limiter/key_fixed_window_test.go`：测试基本 Acquire 成功（新 key、未达上限）
- [x] 3.2 测试 Acquire 超限拒绝（达到最大并发数后返回 false）
- [x] 3.3 测试 Release 正常释放（计数器递减）
- [x] 3.4 测试 Release 下溢保护（计数器为 0 时 Release 不变负）
- [x] 3.5 测试 Release 不存在的 key（无错误返回）
- [x] 3.6 测试 CurrentConcurrent 查询（存在 key 返回正确值、不存在 key 返回 0）
- [x] 3.7 测试多 key 隔离（不同 key 的计数器互不影响）
- [x] 3.8 测试并发安全（多 goroutine 同时 Acquire/Release 同一 key，计数器不超限）
- [x] 3.9 测试构造配置校验（零/负 maxConcurrent、负 expiration 返回错误）
- [x] 3.10 测试接口合规性（`var _ KeyFixedWindowLimiterInterface = (*KeyFixedWindowLimiter)(nil)`）
- [x] 3.11 测试 TTL 设置和刷新（Acquire 后 key 有 TTL，再次 Acquire 后 TTL 被刷新）
- [x] 3.12 测试 per-request 覆盖 maxConcurrent
