## Context

taskx 是一个 DAG 任务调度框架，当前所有持久化操作通过 bun ORM 直接依赖 SQL 数据库。Phase 1 已完成 Store 接口抽象和 DAO 层对称分层重构：

- **接口层** `dao/` — 定义 5 个存储无关的 DAO 接口（TaskDAO、SubtaskDAO、TaskEdgeDAO、TaskBakDAO、SubtaskBakDAO），方法签名不含任何存储后端类型
- **SQL 实现** `dao/sqld/` — 所有 SQL DAO 实现，通过 `db(ctx)` helper 和 `dao.TxFromContext` 实现 context-based 事务传播
- **Redis 实现** `dao/redisd/`（待实现）— 计划实现 Redis DAO
- **Store 抽象** `dao/store.go` — `RunInTx(ctx, fn)` 封装事务语义，SQL 用 `bun.Tx`，Redis 用 Lua 脚本

项目已有成熟的 `redis/v2` 客户端（基于 go-redis v9），支持 standalone 和 cluster 模式，提供 `Cmdable` 接口封装和 KeyPrefix 管理。

## Goals / Non-Goals

**Goals:**
- 定义存储无关的 DAO 接口，消除 `*bun.Tx` 依赖，使接口与存储后端解耦
- 实现完整的 Redis DAO，使用 Hash + Sorted Set + Lua 脚本优化高并发读写
- 通过配置一键切换存储后端（`sql` / `redis`），零代码变更
- 保持 SQL 实现完全向后兼容

**Non-Goals:**
- 不实现 SQL → Redis 的在线数据迁移工具
- 不支持同时使用两种存储后端（双写）
- 不改变 Model 层结构（`dao/model/` 保持不变）
- 不重构 DAG 编译和执行引擎（仅改存储层）

## Decisions

### Decision 1: DAO 接口去 bun.Tx 化 — 引入 Store 抽象

**选择**: 在 DAO 接口中移除 `*bun.Tx` 参数，引入 `Store` 接口封装事务语义：

```go
// Store 是存储后端的统一抽象
type Store interface {
    // RunInTx 在事务中执行一组操作
    // SQL 后端使用 bun.Tx，Redis 后端使用 Lua 脚本或 WATCH/MULTI
    RunInTx(ctx context.Context, fn func(ctx context.Context) error) error
}
```

**理由**: 
- `*bun.Tx` 是 SQL 专有概念，Redis 没有等价物
- Store 接口让调用方不关心底层事务实现
- SQL DAO 内部仍使用 bun.Tx，但对上层透明

**替代方案**: 使用 `context.Context` 传递事务 — 过于隐式，且无法保证事务边界清晰。

### Decision 2: Redis 数据结构设计

**选择**: 采用以下 Redis 数据结构：

| 数据 | Key 模式 | 类型 | 说明 |
|------|----------|------|------|
| Task 详情 | `taskx:task:{id}` | Hash | 存储 Task 所有字段 |
| Subtask 详情 | `taskx:subtask:{id}` | Hash | 存储 Subtask 所有字段 |
| TaskEdge 详情 | `taskx:edge:{id}` | Hash | 存储 TaskEdge 所有字段 |
| Task 子任务索引 | `taskx:task:{taskID}:subtasks` | Set | 子任务 ID 集合 |
| Task 边索引 | `taskx:task:{taskID}:edges` | Set | 边 ID 集合 |
| 待调度任务队列 | `taskx:todo:{state}` | Sorted Set | score=execute_time，用于 GetTodoTask |
| 备份 Task | `taskx:bak:task:{id}` | Hash | 同 Task 结构 |
| 备份 Subtask | `taskx:bak:subtask:{id}` | Hash | 同 Subtask 结构 |

**理由**:
- Hash 天然适合存储结构化对象，支持 HGET 单字段读取和 HGETALL 全量读取
- Sorted Set 按 execute_time 排序，`ZRANGEBYSCORE` 高效查询待调度任务，替代 SQL 全表扫描
- Set 用于索引子任务和边，`SMEMBERS` 一次获取所有关联 ID

**替代方案**: 使用 JSON String 存储整个对象 — 无法单字段更新，每次 SetOutputAndState 都需要全量读写。

### Decision 3: CAS 操作使用 Lua 脚本实现

**选择**: `CASWorkerAndState`、`CASWorkerAndRollback` 等 CAS 操作用 Lua 脚本实现原子性：

```lua
-- KEYS[1] = task hash key, ARGV[1] = oldWorker, ARGV[2] = newWorker, ARGV[3] = newState
if redis.call('HGET', KEYS[1], 'worker') == ARGV[1] then
    redis.call('HSET', KEYS[1], 'worker', ARGV[2], 'state', ARGV[3])
    return 1
end
return 0
```

**理由**: 
- Redis Lua 脚本在服务端原子执行，无网络往返开销
- 等效于 SQL 的 `WHERE worker = ? ... UPDATE` CAS 语义
- 返回 affected rows（0 或 1），与现有 DAO 接口签名兼容

### Decision 4: SubmitTask 原子性 — Lua 脚本批量写入

**选择**: Redis 后端的 `SubmitTask` 使用单个 Lua 脚本原子写入 task + subtasks + edges：

**理由**:
- 替代 SQL 的 BatchTx 事务语义
- 避免部分写入成功导致数据不一致
- Lua 脚本内可使用 HSET、SADD、ZADD 等命令

**替代方案**: WATCH + MULTI/EXEC — 需要乐观重试，在高并发下冲突率高。

### Decision 5: 序列化方案 — JSON

**选择**: Redis Hash 的值使用 JSON 序列化（复用项目已有的 `tools.ToByte` / `tools.DeByte`）

**理由**:
- 与 Model 层的 JSON tag 兼容，无需额外适配
- 便于调试和人工查看 Redis 数据
- bun 标签仅 SQL 使用，不影响 Redis 序列化

## Risks / Trade-offs

- **[内存占用]** Redis 内存存储比 SQL 磁盘存储成本高 → 适用于活跃任务数据，备份数据仍建议使用 SQL；通过 `backupTask` 定期清理已完成任务控制内存增长
- **[数据持久性]** Redis RDB/AOF 策略影响数据安全性 → 建议生产环境配置 AOF always 或 everysec；关键业务场景继续使用 SQL 后端
- **[Lua 脚本复杂度]** 大量 Lua 脚本增加维护成本 → 集中管理在 `redisd/scripts/` 目录，每个脚本附带单元测试
- **[Redis Cluster 限制]** Lua 脚本要求所有 key 在同一 slot → 使用 hash tag `{taskID}` 确保同一任务的 task/subtask/edge key 分布到同一 slot

## Migration Plan

1. **Phase 1**: ~~重构 DAO 接口（去 bun.Tx），SQL 实现适配新接口~~ **已完成**
   - 提取 5 个 DAO 接口到 `dao/` 级别，SQL 实现迁移到 `dao/sqld/`
   - Store 接口 + context-based 事务传播（`WithTxContext`/`TxFromContext`）
   - CAS 方法重命名（`CASWorkerAndState`、`CASWorkerAndRollback`）
2. **Phase 2**: 实现 Redis DAO（`dao/redisd/`），新增 Redis 存储后端
3. **Phase 3**: 添加配置切换机制，集成测试
4. **回滚策略**: 配置 `storageBackend: sql` 即可回退到 SQL 后端，零停机

## Open Questions

- 是否需要支持 Redis 数据 TTL（自动过期清理已完成任务）？
- Redis Cluster 模式下 hash tag 策略是否满足所有查询模式？

## Decision 6: DAO 接口最小化 — 仅保留实际调用方法

**选择**: 移除调度层（dispatch.go、receiver.go、receiver_internal.go）未实际调用的 DAO 方法，保留 22 个核心方法。

**移除的方法** (23 个)：
- TaskDAO: QueryPage、DeleteByID、SoftDeleteByID、SetOutputAndState、SetRetry
- SubtaskDAO: GetStore、Insert、QueryPage、DeleteByID、SoftDeleteByID
- TaskEdgeDAO: GetStore、Insert、QueryPage、DeleteByID、DeleteByTaskID
- TaskBakDAO: GetStore、Insert、QueryPage、DeleteByID、SoftDeleteByID
- SubtaskBakDAO: GetStore、Insert、GetByID、QueryPage、DeleteByID、SoftDeleteByID

**理由**:
- 减少 Redis DAO 实现工作量（从 45 个方法降至 22 个）
- QueryPage 等管理接口可通过独立服务层实现，不属调度核心路径
- backup.go 直接走原始 SQL，未使用 DAO 接口
- 后续如有需求可重新添加，不影响架构
