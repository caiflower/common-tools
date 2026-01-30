-- 创建 Task 表
CREATE TABLE IF NOT EXISTS `task` (
      `id` VARCHAR(50)  PRIMARY KEY COMMENT '任务ID',
      `request_id` VARCHAR(255) COMMENT '请求ID',
      `task_name` VARCHAR(255) COMMENT '任务名称',
      `input` TEXT COMMENT '任务输入参数',
      `output` TEXT COMMENT '任务输出结果',
      `worker` VARCHAR(128) COMMENT '执行任务的工作节点',
      `retry` TINYINT COMMENT '剩余重试次数',
      `retry_interval` INT COMMENT '重试间隔(秒)',
      `urgent` TINYINT(1) COMMENT '是否紧急任务(0:否, 1:是)',
      `state` VARCHAR(20) COMMENT '任务状态',
      `description` VARCHAR(512) COMMENT '任务描述',
      `create_time` TIMESTAMP(3) COMMENT '创建时间',
      `last_run_time` TIMESTAMP(3) COMMENT '最后执行时间',
      `execute_time` TIMESTAMP(3) COMMENT '定时执行时间',
      `status` TINYINT COMMENT '任务状态(0:禁用, 1:启用)',
      `affinity_type` VARCHAR(20) COMMENT '亲和性类型(SameNode, ForceSameNode, Random)',
      `primary_worker` VARCHAR(128) COMMENT '主要执行节点',
      INDEX idx_request_id (`request_id`),
      INDEX idx_state (`state`),
      INDEX idx_execute_time (`execute_time`),
      INDEX idx_affinity_type (`affinity_type`),
      INDEX idx_primary_worker (`primary_worker`),
      UNIQUE INDEX idx_id (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8 COLLATE=utf8_bin;

-- 创建 SubTask 表
CREATE TABLE IF NOT EXISTS `subtask` (
     `id` VARCHAR(50) PRIMARY KEY COMMENT '子任务ID',
     `task_id` VARCHAR(50) COMMENT '所属任务ID',
     `pre_subtask_id` TEXT COMMENT '前置子任务ID列表',
     `task_name` VARCHAR(255) COMMENT '子任务名称',
     `input` TEXT COMMENT '子任务输入参数',
     `output` TEXT COMMENT '子任务输出结果',
     `state` VARCHAR(20) COMMENT '子任务状态',
     `worker` VARCHAR(128) COMMENT '执行子任务的工作节点',
     `retry` TINYINT COMMENT '剩余重试次数',
     `retry_interval` INT COMMENT '重试间隔(秒)',
     `rollback` VARCHAR(20) COMMENT '回滚策略',
     `last_run_time` TIMESTAMP(3) COMMENT '最后执行时间',
     `status` TINYINT COMMENT '子任务状态(0:禁用, 1:启用)',
     UNIQUE INDEX idx_id (`id`),
     INDEX idx_task_id (`task_id`),
     INDEX idx_state (`state`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8 COLLATE=utf8_bin;

-- 创建 Task 表
CREATE TABLE IF NOT EXISTS `task_bak` (
    `id` VARCHAR(50)  PRIMARY KEY COMMENT '任务ID',
    `request_id` VARCHAR(255) COMMENT '请求ID',
    `task_name` VARCHAR(255) COMMENT '任务名称',
    `input` TEXT COMMENT '任务输入参数',
    `output` TEXT COMMENT '任务输出结果',
    `worker` VARCHAR(128) COMMENT '执行任务的工作节点',
    `retry` TINYINT COMMENT '剩余重试次数',
    `retry_interval` INT COMMENT '重试间隔(秒)',
    `urgent` TINYINT(1) COMMENT '是否紧急任务(0:否, 1:是)',
    `state` VARCHAR(20) COMMENT '任务状态',
    `description` VARCHAR(512) COMMENT '任务描述',
    `create_time` TIMESTAMP(3) COMMENT '创建时间',
    `last_run_time` TIMESTAMP(3) COMMENT '最后执行时间',
    `execute_time` TIMESTAMP(3) COMMENT '定时执行时间',
    `status` TINYINT COMMENT '任务状态(0:禁用, 1:启用)',
    `affinity_type` VARCHAR(20) COMMENT '亲和性类型(SameNode, ForceSameNode, Random)',
    `primary_worker` VARCHAR(128) COMMENT '主要执行节点',
    INDEX idx_request_id (`request_id`),
    INDEX idx_state (`state`),
    INDEX idx_execute_time (`execute_time`),
    INDEX idx_affinity_type (`affinity_type`),
    INDEX idx_primary_worker (`primary_worker`),
    UNIQUE INDEX idx_id (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8 COLLATE=utf8_bin;

-- 创建 SubTask 表
CREATE TABLE IF NOT EXISTS `subtask_bak` (
    `id` VARCHAR(50) PRIMARY KEY COMMENT '子任务ID',
    `task_id` VARCHAR(50) COMMENT '所属任务ID',
    `pre_subtask_id` TEXT COMMENT '前置子任务ID列表',
    `task_name` VARCHAR(255) COMMENT '子任务名称',
    `input` TEXT COMMENT '子任务输入参数',
    `output` TEXT COMMENT '子任务输出结果',
    `state` VARCHAR(20) COMMENT '子任务状态',
    `worker` VARCHAR(128) COMMENT '执行子任务的工作节点',
    `retry` TINYINT COMMENT '剩余重试次数',
    `retry_interval` INT COMMENT '重试间隔(秒)',
    `rollback` VARCHAR(20) COMMENT '回滚策略',
    `last_run_time` TIMESTAMP(3) COMMENT '最后执行时间',
    `status` TINYINT COMMENT '子任务状态(0:禁用, 1:启用)',
    UNIQUE INDEX idx_id (`id`),
    INDEX idx_task_id (`task_id`),
    INDEX idx_state (`state`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8 COLLATE=utf8_bin;