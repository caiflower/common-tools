## 1. Store 接口抽象层 (taskx-store-interface)

- [x] 1.1 创建 `taskx/dao/store.go`，定义 `Store` 接口（`RunInTx` 方法）和 `GetStore` 方法签名
- [x] 1.2 创建 `taskx/dao/sql_store.go`，实现 SQL 版 Store（封装 `dbv1.Client` + `bun.Tx`）
- [x] 1.3 重构 `TaskDAO` 接口：移除所有方法中的 `tx ...*bun.Tx` 参数，`GetClient()` 返回 `Store`
- [x] 1.4 重构 `SubtaskDAO` 接口：移除 `tx ...*bun.Tx` 参数，`GetClient()` 返回 `Store`
- [x] 1.5 重构 `TaskEdgeDAO` 接口：移除 `tx ...*bun.Tx` 参数，`GetClient()` 返回 `Store`
- [x] 1.6 重构 `TaskBakDAO` 接口：移除 `tx ...*bun.Tx` 参数，`GetClient()` 返回 `Store`
- [x] 1.7 重构 `SubtaskBakDAO` 接口：移除 `tx ...*bun.Tx` 参数，`GetClient()` 返回 `Store`
- [x] 1.8 适配 SQL 实现 `taskDAO`：内部使用 Store.RunInTx 管理事务，保持原有 SQL 逻辑不变
- [x] 1.9 适配 SQL 实现 `subtaskDAO`：内部使用 Store.RunInTx 管理事务
- [x] 1.10 适配 SQL 实现 `taskEdgeDAO`、`taskBakDAO`、`subtaskBakDAO`
- [x] 1.11 更新 `taskx/dispatch.go` 中 `SubmitTask` 和 `SubmitTaskWithTx`：使用 `Store.RunInTx` 替代 `dbv1.NewBatchTx`
- [x] 1.12 更新 `taskx/backup.go` 中 `backupTask`：使用 DAO 接口替代直接 SQL 操作
- [x] 1.13 运行现有单元测试，确保 SQL 后端在接口重构后功能完全正常

## 2. Redis DAO 实现 (taskx-redis-dao)

- [ ] 2.1 创建 `taskx/dao/redisd/` 目录结构和公共工具文件 `redisd/common.go`（key 生成、JSON 序列化/反序列化、hash tag 工具）
- [ ] 2.2 创建 `taskx/dao/redisd/store.go`，实现 Redis 版 Store（`RunInTx` 使用 Lua 脚本或 Pipeline）
- [ ] 2.3 创建 `taskx/dao/redisd/scripts.go`，集中定义所有 Lua 脚本（CAS 操作、批量写入、原子更新）
- [ ] 2.4 实现 `taskx/dao/redisd/task_dao.go`：Redis 版 TaskDAO（Hash 存储 + Sorted Set 索引 + Lua CAS）
- [ ] 2.5 实现 `taskx/dao/redisd/subtask_dao.go`：Redis 版 SubtaskDAO（Hash 存储 + Set 索引 + Lua CAS）
- [ ] 2.6 实现 `taskx/dao/redisd/task_edge_dao.go`：Redis 版 TaskEdgeDAO（Hash 存储 + Set 索引）
- [ ] 2.7 实现 `taskx/dao/redisd/task_bak_dao.go`：Redis 版 TaskBakDAO（Hash 存储，`taskx:bak:` 前缀）
- [ ] 2.8 实现 `taskx/dao/redisd/subtask_bak_dao.go`：Redis 版 SubtaskBakDAO（Hash 存储 + Set 索引）
- [ ] 2.9 编写 Redis DAO 单元测试：使用 miniredis 或 gomock 模拟 Redis，覆盖所有 DAO 方法
- [ ] 2.10 编写 Lua 脚本专项测试：验证 CAS 成功/失败场景、批量写入原子性

## 3. 存储后端切换机制 (taskx-storage-switch)

- [ ] 3.1 在 `taskx/dispatch.go` 的 `Config` 结构体中新增 `StorageBackend string` 字段（默认值 `"sql"`）
- [ ] 3.2 修改 `InitTaskDispatcher`：根据 `StorageBackend` 值创建对应的 DAO 实现并注册到 bean
- [ ] 3.3 修改 `taskDispatcher` 结构体：移除 `DBClient dbv1.DB` 硬依赖，改为通过 Store 接口访问
- [ ] 3.4 修改 `taskDispatcher.SubmitTask`：SQL 路径使用 BatchTx，Redis 路径使用 `Store.RunInTx` + Lua 原子写入
- [ ] 3.5 修改 `taskDispatcher.SubmitTaskWithTx`：Redis 路径忽略外部 tx 参数（使用 Store.RunInTx）
- [ ] 3.6 修改 `taskDispatcher.backupTask`：Redis 路径使用 SCAN + Lua 脚本迁移备份
- [ ] 3.7 确保 `taskReceiver` 无需修改即可适配两种存储后端（仅依赖 DAO 接口）
- [ ] 3.8 添加 `StorageBackend` 配置校验：无效值返回明确错误信息

## 4. 集成测试与验证

- [ ] 4.1 编写 SQL 后端回归测试：验证接口重构后 SubmitTask → handleTask → execSubtask 全链路正常
- [ ] 4.2 编写 Redis 后端集成测试：使用真实 Redis（或 miniredis）验证完整任务生命周期
- [ ] 4.3 编写存储切换测试：同一 Config 框架下切换 `sql` → `redis`，验证 DAO 正确注入
- [ ] 4.4 编写高并发压测：对比 SQL 和 Redis 后端在 1000+ 并发任务下的吞吐量和延迟
