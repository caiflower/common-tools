## ADDED Requirements

### Requirement: Store interface abstraction
The system SHALL define a `Store` interface that abstracts transaction semantics from the underlying storage backend. The interface MUST provide a `RunInTx` method that executes a group of operations within a single transaction boundary.

#### Scenario: SQL backend RunInTx
- **WHEN** Store is backed by SQL (bun)
- **THEN** `RunInTx` SHALL open a `bun.Tx`, pass the transaction context to the callback, and commit on success or rollback on error

#### Scenario: Redis backend RunInTx
- **WHEN** Store is backed by Redis
- **THEN** `RunInTx` SHALL execute the callback using a Lua script or pipeline to ensure atomicity, with no explicit commit/rollback

### Requirement: DAO interfaces without bun.Tx dependency
All 5 DAO interfaces (TaskDAO, SubtaskDAO, TaskEdgeDAO, TaskBakDAO, SubtaskBakDAO) SHALL remove `*bun.Tx` variadic parameters from their method signatures. Transaction boundaries MUST be managed through the Store interface instead.

#### Scenario: TaskDAO Insert without bun.Tx
- **WHEN** `TaskDAO.Insert(ctx, data)` is called
- **THEN** the method SHALL persist the task using the DAO's underlying Store, without requiring a `*bun.Tx` argument

#### Scenario: SubtaskDAO BatchInsert with transaction
- **WHEN** multiple subtasks need to be inserted atomically
- **THEN** the caller SHALL use `Store.RunInTx` to wrap the batch insert, and the DAO SHALL use the transaction context provided by the callback

#### Scenario: Backward compatibility for SQL callers
- **WHEN** existing SQL DAO implementations are used with the new interface
- **THEN** all existing functionality SHALL continue to work without behavioral changes

### Requirement: GetClient returns Store instead of dbv1.DB
The `GetClient()` method on DAO interfaces SHALL return the `Store` interface instead of `dbv1.DB`, allowing storage-backend-agnostic access to the underlying store.

#### Scenario: GetClient on SQL DAO
- **WHEN** `GetClient()` is called on a SQL-backed DAO
- **THEN** it SHALL return a Store implementation that wraps the bun DB client

#### Scenario: GetClient on Redis DAO
- **WHEN** `GetClient()` is called on a Redis-backed DAO
- **THEN** it SHALL return a Store implementation that wraps the Redis client

### Requirement: QueryPage abstraction
The `QueryPage` method SHALL accept a storage-agnostic filter interface instead of relying on `bun.IDB` for query building. Each storage backend SHALL implement its own query translation.

#### Scenario: SQL QueryPage
- **WHEN** `QueryPage` is called on a SQL DAO
- **THEN** it SHALL build and execute a SQL query using bun's query builder as before

#### Scenario: Redis QueryPage
- **WHEN** `QueryPage` is called on a Redis DAO
- **THEN** it SHALL use SCAN + filter or Sorted Set range queries to return paginated results
