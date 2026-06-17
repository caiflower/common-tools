-- 1. 创建task表（无内联索引）
CREATE TABLE IF NOT EXISTS task (
    id VARCHAR(50) PRIMARY KEY /* 任务ID */,
    request_id VARCHAR(255) /* 请求ID */,
    task_name VARCHAR(255) /* 任务名称 */,
    input TEXT /* 任务输入参数 */,
    output TEXT /* 任务输出结果 */,
    worker VARCHAR(128) /* 执行任务的工作节点 */,
    retry TINYINT /* 剩余重试次数 */,
    retry_interval INT /* 重试间隔(秒) */,
    urgent TINYINT /* 是否紧急任务(0:否, 1:是) */,
    state VARCHAR(20) /* 任务状态 */,
    description VARCHAR(512) /* 任务描述 */,
    create_time TIMESTAMP(3) /* 创建时间，保留3位毫秒 */,
    last_run_time TIMESTAMP(3) /* 更新时间，保留3位毫秒 */,
    execute_time TIMESTAMP(3) /* 定时执行时间，保留3位毫秒 */,
    status TINYINT /* 任务状态(0:禁用, 1:启用) */,
    affinity_type VARCHAR(20) /* 亲和性类型(SameNode, ForceSameNode, Random) */,
    primary_worker VARCHAR(128) /* 主要执行节点 */,
    rollback_strategy VARCHAR(20) NOT NULL DEFAULT 'rollback_all' /* 回滚策略(rollback_all, rollback_failed, rollback_custom) */
    );

-- 为task表创建索引
CREATE INDEX IF NOT EXISTS idx_task_request_id ON task(request_id);
CREATE INDEX IF NOT EXISTS idx_task_state ON task(state);
CREATE INDEX IF NOT EXISTS idx_task_execute_time ON task(execute_time);
CREATE INDEX IF NOT EXISTS idx_task_affinity_type ON task(affinity_type);
CREATE INDEX IF NOT EXISTS idx_task_primary_worker ON task(primary_worker);

-- 2. 创建subtask表（无内联索引）
CREATE TABLE IF NOT EXISTS subtask (
    id VARCHAR(50) PRIMARY KEY /* 子任务ID */,
    task_id VARCHAR(50) /* 所属任务ID */,
    pre_subtask_id TEXT /* 前置子任务ID列表 */,
    task_name VARCHAR(255) /* 子任务名称 */,
    trigger_mode VARCHAR(20) NOT NULL DEFAULT 'all_predecessor' /* 触发模式(all_predecessor, any_predecessor) */,
    priority INT NOT NULL DEFAULT 0 /* 优先级 */,
    timeout INT NOT NULL DEFAULT 0 /* 超时时间(秒) */,
    input TEXT /* 子任务输入参数 */,
    output TEXT /* 子任务输出结果 */,
    state VARCHAR(20) /* 子任务状态 */,
    worker VARCHAR(128) /* 执行子任务的工作节点 */,
    retry TINYINT /* 剩余重试次数 */,
    retry_interval INT /* 重试间隔(秒) */,
    rollback VARCHAR(20) /* 回滚策略 */,
    last_run_time TIMESTAMP(3) /* 更新时间，保留3位毫秒 */,
    status TINYINT /* 子任务状态(0:禁用, 1:启用) */,
    settings VARCHAR(4096) NOT NULL DEFAULT '' /* extensible JSON config (branch_config, etc.) */
    );

-- 为subtask表创建索引
CREATE INDEX IF NOT EXISTS idx_subtask_task_id ON subtask(task_id);
CREATE INDEX IF NOT EXISTS idx_subtask_state ON subtask(state);

-- 3. 创建task_edge表
CREATE TABLE IF NOT EXISTS task_edge (
    id VARCHAR(50) PRIMARY KEY /* 边ID */,
    task_id VARCHAR(50) NOT NULL /* 所属任务ID */,
    from_subtask_id VARCHAR(50) NOT NULL /* 源子任务ID */,
    to_subtask_id VARCHAR(50) NOT NULL /* 目标子任务ID */,
    edge_type VARCHAR(20) NOT NULL DEFAULT 'control+data' /* 边类型(control, data, control+data) */,
    field_mappings TEXT /* 字段映射JSON */,
    create_time TIMESTAMP(3) /* 创建时间 */
);

-- 为task_edge表创建索引
CREATE INDEX IF NOT EXISTS idx_task_edge_task_id ON task_edge(task_id);
CREATE INDEX IF NOT EXISTS idx_task_edge_from ON task_edge(from_subtask_id);
CREATE INDEX IF NOT EXISTS idx_task_edge_to ON task_edge(to_subtask_id);

-- 4. 创建task_bak表（无内联索引）
CREATE TABLE IF NOT EXISTS task_bak (
    id VARCHAR(50) PRIMARY KEY /* 任务ID */,
    request_id VARCHAR(255) /* 请求ID */,
    task_name VARCHAR(255) /* 任务名称 */,
    input TEXT /* 任务输入参数 */,
    output TEXT /* 任务输出结果 */,
    worker VARCHAR(128) /* 执行任务的工作节点 */,
    retry TINYINT /* 剩余重试次数 */,
    retry_interval INT /* 重试间隔(秒) */,
    urgent TINYINT /* 是否紧急任务(0:否, 1:是) */,
    state VARCHAR(20) /* 任务状态 */,
    description VARCHAR(512) /* 任务描述 */,
    create_time TIMESTAMP(3) /* 创建时间，保留3位毫秒 */,
    last_run_time TIMESTAMP(3) /* 更新时间，保留3位毫秒 */,
    execute_time TIMESTAMP(3) /* 定时执行时间，保留3位毫秒 */,
    status TINYINT /* 任务状态(0:禁用, 1:启用) */,
    affinity_type VARCHAR(20) /* 亲和性类型(SameNode, ForceSameNode, Random) */,
    primary_worker VARCHAR(128) /* 主要执行节点 */,
    rollback_strategy VARCHAR(20) NOT NULL DEFAULT 'rollback_all' /* 回滚策略(rollback_all, rollback_failed, rollback_custom) */
    );

-- 为task_bak表创建索引
CREATE INDEX IF NOT EXISTS idx_task_bak_request_id ON task_bak(request_id);
CREATE INDEX IF NOT EXISTS idx_task_bak_state ON task_bak(state);
CREATE INDEX IF NOT EXISTS idx_task_bak_execute_time ON task_bak(execute_time);
CREATE INDEX IF NOT EXISTS idx_task_bak_affinity_type ON task_bak(affinity_type);
CREATE INDEX IF NOT EXISTS idx_task_bak_primary_worker ON task_bak(primary_worker);

-- 5. 创建subtask_bak表（无内联索引）
CREATE TABLE IF NOT EXISTS subtask_bak (
    id VARCHAR(50) PRIMARY KEY /* 子任务ID */,
    task_id VARCHAR(50) /* 所属任务ID */,
    pre_subtask_id TEXT /* 前置子任务ID列表 */,
    task_name VARCHAR(255) /* 子任务名称 */,
    trigger_mode VARCHAR(20) NOT NULL DEFAULT 'all_predecessor' /* 触发模式(all_predecessor, any_predecessor) */,
    priority INT NOT NULL DEFAULT 0 /* 优先级 */,
    timeout INT NOT NULL DEFAULT 0 /* 超时时间(秒) */,
    input TEXT /* 子任务输入参数 */,
    output TEXT /* 子任务输出结果 */,
    state VARCHAR(20) /* 子任务状态 */,
    worker VARCHAR(128) /* 执行子任务的工作节点 */,
    retry TINYINT /* 剩余重试次数 */,
    retry_interval INT /* 重试间隔(秒) */,
    rollback VARCHAR(20) /* 回滚策略 */,
    last_run_time TIMESTAMP(3) /* 更新时间，保留3位毫秒 */,
    status TINYINT /* 子任务状态(0:禁用, 1:启用) */,
    settings VARCHAR(4096) NOT NULL DEFAULT '' /* extensible JSON config (branch_config, etc.) */
    );

-- 为subtask_bak表创建索引
CREATE INDEX IF NOT EXISTS idx_subtask_bak_task_id ON subtask_bak(task_id);
CREATE INDEX IF NOT EXISTS idx_subtask_bak_state ON subtask_bak(state);
