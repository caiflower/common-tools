## 1. Store 接口抽象层 (taskx-store-interface) ✅ 已完成

- [x] 1.1 创建 `dao/store.go`，定义 `Store` 接口（`RunInTx`）和 context 事务传播（`WithTxContext`/`TxFromContext`）
- [x] 1.2 提取 5 个 DAO 接口到 `dao/` 级别（task_dao.go、subtask_dao.go、task_edge_dao.go、task_bak_dao.go、subtask_bak_dao.go）
- [x] 1.3 SQL 实现迁移到 `dao/sqld/`，通过 `db(ctx)` helper + `TxFromContext` 实现 context-based 事务传播
- [x] 1.4 创建 `dao/sqld/store.go`，SQL Store 实现（封装 `bun.Tx`，支持嵌套事务复用）
- [x] 1.5 更新 `dispatch.go`：`SubmitTask` 使用 `Store.RunInTx`，`SubmitTaskWithTx` 使用 `WithTxContext`
- [x] 1.6 CAS 方法重命名：`CASWorkerAndState`、`CASWorkerAndRollback`
- [x] 1.7 编译通过 + 所有现有测试通过
- [x] 1.8 **接口精简**：移除 23 个调度层未使用的方法（QueryPage、DeleteByID、SoftDeleteByID 等），仅保留 22 个实际调用方法

## 2. Redis DAO 实现 (taskx-redis-dao) ✅ 已完成

### 2.1 基础设施

- [x] 2.1.1 创建 `dao/redisd/common.go`：key 生成、JSON↔Hash 序列化（`toHash`/`fromHash`）、`KeyConfig`
- [x] 2.1.2 创建 `dao/redisd/store.go`：Redis Store（Pipeline-based `RunInTx`），context 传播支持嵌套复用
- [x] 2.1.3 创建 `dao/redisd/scripts.go`：Lua 脚本（`casWorkerAndState`、`casWorkerAndRollback`）

### 2.2 核心 DAO 实现

- [x] 2.2.1 实现 `dao/redisd/task_dao.go`（7 方法）：GetStore、Insert、GetByID、GetByIDs、GetTodoTask、CASWorkerAndState、SetState
- [x] 2.2.2 实现 `dao/redisd/subtask_dao.go`（10 方法）：BatchInsert、GetByID、GetByTaskID、CASWorkerAndState、CASWorkerAndRollback、GetByIDs、SetOutputAndState、SetRollbackAndState、SetInput、SetRetry
- [x] 2.2.3 实现 `dao/redisd/task_edge_dao.go`（2 方法）：BatchInsert、GetByTaskID

### 2.3 备份 DAO 实现

- [x] 2.3.1 实现 `dao/redisd/task_bak_dao.go`（1 方法）：GetByID
- [x] 2.3.2 实现 `dao/redisd/subtask_bak_dao.go`（1 方法）：GetByTaskID

### 2.4 构造函数与初始化

- [x] 2.4.1 每个 Redis DAO 提供 `NewXxxDAOWithConfig(client v2.RedisClient, keyCfg *KeyConfig) dao.XxxDAO` 构造函数
- [x] 2.4.2 `KeyConfig` 集成在 `common.go` 中（默认前缀 `taskx`）

### 2.5 测试

- [x] 2.5.1 编写 Redis DAO 单元测试：使用 `miniredis` 模拟 Redis，覆盖所有 DAO 方法（30 个测试，common_test.go + dao_test.go）
- [x] 2.5.2 编写 Lua 脚本专项测试：验证 CAS 成功/失败场景、BatchInsert 原子性、SubmitTask 原子写入
- [x] 2.5.3 编写 Redis Cluster hash tag 测试：验证所有 key 包含正确的 `{taskID}` hash tag

## 3. 存储后端切换机制 (taskx-storage-switch) ✅ 已完成

- [x] 3.1 `Config` 新增 `StorageBackend string`（默认 `"sql"`，可选 `"redis"`）
- [x] 3.2 `Config` 新增 `RedisClient v2.RedisClient` + `RedisKeys *redisd.KeyConfig`
- [x] 3.3 `InitTaskDispatcher` 根据 `StorageBackend` 分支创建 sqld 或 redisd DAO
- [x] 3.4 `DBClient dbv1.DB` 保留在结构体中（backup.go 仍依赖原始 SQL，待 Phase 4 重构）
- [x] 3.5 `SubmitTaskWithTx` Redis 路径使用 `Store.RunInTx`，忽略外部 `*bun.Tx`
- [x] 3.6 `backupTask` 保持现有 SQL 逻辑（Redis 路径需独立重构）
- [x] 3.7 `StorageBackend` 配置校验：无效值 panic，Redis 模式缺 RedisClient panic

## 4. 集成测试与验证 ✅ 基本完成

- [x] 4.1 编写 SQL 后端回归测试：验证 SubmitTask → handleTask → execSubtask 全链路（已有 dispatch_test.go 覆盖）
- [x] 4.2 编写 Redis 后端集成测试：使用 miniredis 验证完整任务生命周期
- [x] 4.3 编写存储切换测试：同一 Config 框架下切换 `sql` → `redis`，验证 DAO 正确注入
- [ ] 4.4 编写高并发压测：对比 SQL 和 Redis 后端在 1000+ 并发任务下的吞吐量和延迟（手动测试）
