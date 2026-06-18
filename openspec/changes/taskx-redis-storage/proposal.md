## Why

taskx 任务调度框架当前仅支持 SQL 持久化存储（基于 bun ORM），在高并发场景下存在以下瓶颈：
1. SQL 事务开销大：`SubmitTask` 每次提交都需要开启数据库事务，写入 task + subtasks + edges 三张表
2. 状态更新延迟高：调度循环中频繁的 `SetState`、`CASWorkerAndState` 等 CAS 操作受限于 SQL 行锁竞争
3. 轮询成本高：`handleTask` 每个调度周期都执行全表扫描查询待执行任务

Redis 作为内存存储引擎，天然支持原子操作（WATCH/MULTI/Lua 脚本）和高效的数据结构（Hash、Sorted Set），可以显著提升高并发场景下的任务调度吞吐量。

## What Changes

- **重构 DAO 接口层**：去除接口方法中 `*bun.Tx` 参数，引入存储无关的事务抽象（`Store` 接口），使 DAO 接口与具体存储后端解耦
- **新增 Redis DAO 实现**：基于项目已有的 `redis/v2` 客户端，实现 5 个 DAO 接口的 Redis 版本（TaskDAO、SubtaskDAO、TaskEdgeDAO、TaskBakDAO、SubtaskBakDAO）
- **Redis 数据结构设计**：使用 Hash 存储任务/子任务详情，Sorted Set 按状态+时间索引待调度任务，Lua 脚本实现原子 CAS 操作
- **存储后端可配置**：通过 `Config` 新增 `StorageBackend` 字段（`sql` / `redis`），在 `InitTaskDispatcher` 时根据配置注入对应的 DAO 实现
- **SubmitTask 适配**：SQL 后端保持 BatchTx 事务，Redis 后端使用 Lua 脚本保证原子性批量写入
- **backupTask 适配**：SQL 后端保持现有逻辑，Redis 后端使用 SCAN + Pipeline 批量迁移到备份 key 空间

## Capabilities

### New Capabilities
- `taskx-store-interface`: 定义存储无关的 Store 接口和事务抽象，替代当前 DAO 中对 `*bun.Tx` 的直接依赖
- `taskx-redis-dao`: Redis 版 DAO 实现，包含 5 个 DAO 接口的完整 Redis 存储实现（Hash + Sorted Set + Lua 脚本）
- `taskx-storage-switch`: 存储后端切换机制，通过配置选择 SQL 或 Redis 作为存储引擎

### Modified Capabilities

## Impact

- **taskx/dao/**: DAO 接口签名变更（移除 `*bun.Tx` 参数），接口提取到独立文件，新增 `store.go` 定义存储抽象接口
- **taskx/dao/sqld/**: SQL DAO 实现迁移到 sqld 子包
- **taskx/dao/redisd/**: 新增目录，包含所有 Redis DAO 实现
- **taskx/dispatch.go**: `taskDispatcher` 结构体和 `InitTaskDispatcher` 需适配 Store 接口，移除对 `dbv1.DB` 的直接依赖
- **taskx/receiver.go**: `taskReceiver` 无需修改，仅依赖 DAO 接口
- **taskx/backup.go**: 备份逻辑需根据存储后端分支处理
- **taskx/dao/model/**: Model 层保持不变（bun 标签仅 SQL 使用，Redis 用 JSON 序列化）
- **向后兼容**: SQL DAO 实现已适配新接口（`dao/sqld/`），内部逻辑不变
