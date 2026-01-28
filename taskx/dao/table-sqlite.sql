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
    status TINYINT /* 任务状态(0:禁用, 1:启用) */
    );

-- 为task表创建索引
CREATE INDEX IF NOT EXISTS idx_task_request_id ON task(request_id);
CREATE INDEX IF NOT EXISTS idx_task_state ON task(state);

-- 2. 创建subtask表（无内联索引）
CREATE TABLE IF NOT EXISTS subtask (
    id VARCHAR(50) PRIMARY KEY /* 子任务ID */,
    task_id VARCHAR(50) /* 所属任务ID */,
    pre_subtask_id TEXT /* 前置子任务ID列表 */,
    task_name VARCHAR(255) /* 子任务名称 */,
    input TEXT /* 子任务输入参数 */,
    output TEXT /* 子任务输出结果 */,
    state VARCHAR(20) /* 子任务状态 */,
    worker VARCHAR(128) /* 执行子任务的工作节点 */,
    retry TINYINT /* 剩余重试次数 */,
    retry_interval INT /* 重试间隔(秒) */,
    rollback VARCHAR(20) /* 回滚策略 */,
    last_run_time TIMESTAMP(3) /* 更新时间，保留3位毫秒 */,
    status TINYINT /* 子任务状态(0:禁用, 1:启用) */
    );

-- 为subtask表创建索引
CREATE INDEX IF NOT EXISTS idx_subtask_task_id ON subtask(task_id);
CREATE INDEX IF NOT EXISTS idx_subtask_state ON subtask(state);

-- 3. 创建task_bak表（无内联索引）
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
    status TINYINT /* 任务状态(0:禁用, 1:启用) */
    );

-- 为task_bak表创建索引
CREATE INDEX IF NOT EXISTS idx_task_bak_request_id ON task_bak(request_id);
CREATE INDEX IF NOT EXISTS idx_task_bak_state ON task_bak(state);

-- 4. 创建subtask_bak表（无内联索引）
CREATE TABLE IF NOT EXISTS subtask_bak (
    id VARCHAR(50) PRIMARY KEY /* 子任务ID */,
    task_id VARCHAR(50) /* 所属任务ID */,
    pre_subtask_id TEXT /* 前置子任务ID列表 */,
    task_name VARCHAR(255) /* 子任务名称 */,
    input TEXT /* 子任务输入参数 */,
    output TEXT /* 子任务输出结果 */,
    state VARCHAR(20) /* 子任务状态 */,
    worker VARCHAR(128) /* 执行子任务的工作节点 */,
    retry TINYINT /* 剩余重试次数 */,
    retry_interval INT /* 重试间隔(秒) */,
    rollback VARCHAR(20) /* 回滚策略 */,
    last_run_time TIMESTAMP(3) /* 更新时间，保留3位毫秒 */,
    status TINYINT /* 子任务状态(0:禁用, 1:启用) */
    );

-- 为subtask_bak表创建索引
CREATE INDEX IF NOT EXISTS idx_subtask_bak_task_id ON subtask_bak(task_id);
CREATE INDEX IF NOT EXISTS idx_subtask_bak_state ON subtask_bak(state);