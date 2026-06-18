## ADDED Requirements

### Requirement: StorageBackend configuration field
The taskx `Config` struct SHALL include a `StorageBackend` field of type string with valid values `"sql"` and `"redis"`. The default value SHALL be `"sql"` to maintain backward compatibility.

#### Scenario: Default configuration uses SQL
- **WHEN** `Config.StorageBackend` is not set or is empty
- **THEN** the system SHALL use SQL as the storage backend, behaving identically to the current implementation

#### Scenario: Explicit SQL configuration
- **WHEN** `Config.StorageBackend` is set to `"sql"`
- **THEN** the system SHALL initialize SQL DAOs (TaskDAO, SubtaskDAO, etc.) and register them via bean

#### Scenario: Redis configuration
- **WHEN** `Config.StorageBackend` is set to `"redis"`
- **THEN** the system SHALL initialize Redis DAOs using the configured `redis/v2.RedisClient` and register them via bean

#### Scenario: Invalid configuration value
- **WHEN** `Config.StorageBackend` is set to an unrecognized value
- **THEN** the system SHALL return an error during `InitTaskDispatcher` with a clear message listing valid values

### Requirement: Redis client injection via configuration
When `StorageBackend` is `"redis"`, the system SHALL accept a `redis/v2.RedisClient` through configuration or bean injection for use by all Redis DAO implementations.

#### Scenario: Redis client from bean
- **WHEN** `StorageBackend` is `"redis"` and a `redis/v2.RedisClient` is registered in the bean container
- **THEN** all Redis DAOs SHALL use this shared client instance

#### Scenario: Missing Redis client
- **WHEN** `StorageBackend` is `"redis"` but no `redis/v2.RedisClient` is available
- **THEN** the system SHALL return an error during initialization indicating that a Redis client is required

### Requirement: Dispatcher adapts to storage backend
The `taskDispatcher` SHALL not directly depend on `dbv1.DB` when using Redis storage. All database operations SHALL go through the DAO interfaces.

#### Scenario: SubmitTask with Redis backend
- **WHEN** `SubmitTask` is called with Redis storage backend
- **THEN** the system SHALL use `Store.RunInTx` for atomic writes instead of `dbv1.NewBatchTx`

#### Scenario: SubmitTask with SQL backend
- **WHEN** `SubmitTask` is called with SQL storage backend
- **THEN** the system SHALL continue to use `dbv1.NewBatchTx` as before

#### Scenario: handleTaskImmediately with Redis backend
- **WHEN** `handleTaskImmediately` queries tasks with Redis storage backend
- **THEN** the system SHALL use `TaskDAO.GetByIDs` and `SubtaskDAO.GetByTaskID` to load data from Redis

### Requirement: Receiver adapts to storage backend
The `taskReceiver` SHALL work with both SQL and Redis DAO implementations without code changes, relying solely on the DAO interface contract.

#### Scenario: execSubtask with Redis backend
- **WHEN** a subtask is executed with Redis storage backend
- **THEN** all outcome persistence operations (`SetOutputAndState`, `SetRetry`, `SetRollbackAndState`) SHALL write to Redis

#### Scenario: execSubtaskRollback with Redis backend
- **WHEN** a subtask rollback is executed with Redis storage backend
- **THEN** all rollback persistence operations SHALL write to Redis

### Requirement: Backup task adapts to storage backend
The `backupTask` method SHALL support both SQL and Redis storage backends. For Redis, it SHALL migrate completed task data from the active key space to the backup key space.

#### Scenario: backupTask with SQL backend
- **WHEN** backupTask runs with SQL storage backend
- **THEN** it SHALL continue to use SQL transactions as currently implemented

#### Scenario: backupTask with Redis backend
- **WHEN** backupTask runs with Redis storage backend
- **THEN** it SHALL scan for completed tasks older than `BackupTaskAge`, copy them to backup keys (`taskx:bak:`), and delete from the active key space using a Lua script for atomicity
