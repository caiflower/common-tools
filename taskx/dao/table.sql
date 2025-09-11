-- 创建 Task 表
CREATE TABLE IF NOT EXISTS task (
      id BIGINT AUTO_INCREMENT PRIMARY KEY,
      request_id VARCHAR(255),
      task_id VARCHAR(50),
      task_name VARCHAR(255),
      input TEXT,
      output TEXT,
      worker VARCHAR(128),
      retry TINYINT,
      retry_interval INT,
      urgent TINYINT,
      task_state VARCHAR(20),
      description VARCHAR(512),
      create_time DATETIME,
      update_time DATETIME,
      status TINYINT,
      INDEX idx_request_id (request_id),
      UNIQUE INDEX idx_task_id (task_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8 COLLATE=utf8_bin;

-- 创建 SubTask 表
CREATE TABLE IF NOT EXISTS subtask (
     id BIGINT AUTO_INCREMENT PRIMARY KEY,
     task_id VARCHAR(50),
     subtask_id VARCHAR(50),
     pre_subtask_id TEXT,
     task_name VARCHAR(255),
     input TEXT,
     output TEXT,
     task_state VARCHAR(20),
     worker VARCHAR(128),
     retry TINYINT,
     retry_interval INT,
     rollback VARCHAR(20),
     update_time DATETIME,
     status TINYINT,
     UNIQUE INDEX idx_subtask_id (subtask_id),
     INDEX idx_task_id (task_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8 COLLATE=utf8_bin;

-- 创建 Task 表
CREATE TABLE IF NOT EXISTS task_bak (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    request_id VARCHAR(255),
    task_id VARCHAR(50),
    task_name VARCHAR(255),
    input TEXT,
    output TEXT,
    worker VARCHAR(128),
    retry TINYINT,
    retry_interval INT,
    urgent TINYINT,
    task_state VARCHAR(20),
    description VARCHAR(512),
    create_time DATETIME,
    update_time DATETIME,
    status TINYINT,
    INDEX idx_request_id (request_id),
    UNIQUE INDEX idx_task_id (task_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8 COLLATE=utf8_bin;

-- 创建 SubTask 表
CREATE TABLE IF NOT EXISTS subtask_bak (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    task_id VARCHAR(50),
    subtask_id VARCHAR(50),
    pre_subtask_id TEXT,
    task_name VARCHAR(255),
    input TEXT,
    output TEXT,
    task_state VARCHAR(20),
    worker VARCHAR(128),
    retry TINYINT,
    retry_interval INT,
    rollback VARCHAR(20),
    update_time DATETIME,
    status TINYINT,
    UNIQUE INDEX idx_subtask_id (subtask_id),
    INDEX idx_task_id (task_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8 COLLATE=utf8_bin;