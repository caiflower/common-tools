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

### Requirement: GetStore returns Store interface
The `GetStore()` method on DAO interfaces SHALL return the `Store` interface, allowing storage-backend-agnostic access to the underlying store.

#### Scenario: GetStore on SQL DAO
- **WHEN** `GetStore()` is called on a SQL-backed DAO
- **THEN** it SHALL return a Store implementation that wraps the bun DB client

#### Scenario: GetStore on Redis DAO
- **WHEN** `GetStore()` is called on a Redis-backed DAO
- **THEN** it SHALL return a Store implementation that wraps the Redis client

### Requirement: Context-based transaction propagation
When a caller needs to execute multiple DAO operations within a single transaction, the system SHALL use context-based transaction propagation via `dao.WithTxContext(ctx, tx)` and `dao.TxFromContext(ctx)`. The DAO's internal `db(ctx)` helper SHALL automatically detect the transaction from context.

#### Scenario: Multi-DAO transaction via Store.RunInTx
- **WHEN** `Store.RunInTx(ctx, fn)` is called
- **THEN** the system SHALL begin a transaction, store it in the context via `WithTxContext`, and pass the enriched context to `fn`
- **AND** all DAO operations within `fn` SHALL automatically use the same transaction via `TxFromContext`

#### Scenario: SQL backend tx extraction
- **WHEN** a SQL DAO's `db(ctx)` is called with a context containing a `*bun.Tx`
- **THEN** it SHALL return the `*bun.Tx` as `bun.IDB` instead of the default `db.GetDB()`

#### Scenario: Redis backend tx extraction
- **WHEN** a Redis DAO's operation is called within a `Store.RunInTx` context
- **THEN** it SHALL use the pipeline or Lua script context stored in the context for atomic execution

### Requirement: QueryPage abstraction
The `QueryPage` method SHALL accept a storage-agnostic filter interface instead of relying on `bun.IDB` for query building. Each storage backend SHALL implement its own query translation.

#### Scenario: SQL QueryPage
- **WHEN** `QueryPage` is called on a SQL DAO
- **THEN** it SHALL build and execute a SQL query using bun's query builder as before

#### Scenario: Redis QueryPage
- **WHEN** `QueryPage` is called on a Redis DAO
- **THEN** it SHALL use SCAN + filter or Sorted Set range queries to return paginated results
