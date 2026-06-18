## ADDED Requirements

### Requirement: MetricsHook implements v9 ProcessHook interface
The `redis/v2` package SHALL provide a `MetricsHook` struct that implements the go-redis v9 `redis.Hook` interface. The hook SHALL implement `ProcessHook` to wrap the `next` function, measuring elapsed time and recording command-level Prometheus metrics (counter + histogram).

#### Scenario: Successful command recording
- **WHEN** a Redis command is executed through a client with `MetricsHook` registered
- **THEN** `redis_commands_total` SHALL increment with labels `{addr, command, status="ok"}` and `redis_command_duration_seconds` SHALL record the elapsed time

#### Scenario: Failed command recording
- **WHEN** a Redis command returns an error (other than `redis.Nil`)
- **THEN** `redis_commands_total` SHALL increment with `status="error"`

#### Scenario: redis.Nil treated as success
- **WHEN** a Redis command returns `redis.Nil` (key not found)
- **THEN** `redis_commands_total` SHALL increment with `status="ok"` (matching v1 behavior)

### Requirement: MetricsHook implements v9 ProcessPipelineHook interface
The `MetricsHook` SHALL implement `ProcessPipelineHook` to wrap the pipeline `next` function, measuring batch elapsed time and recording pipeline-level Prometheus metrics.

#### Scenario: Successful pipeline recording
- **WHEN** a Redis pipeline is executed with N commands
- **THEN** `redis_pipeline_commands_total` SHALL increment by N with `status="ok"` and `redis_pipeline_duration_seconds` SHALL record the batch elapsed time

#### Scenario: Pipeline with errors
- **WHEN** any command in the pipeline returns a non-nil, non-redis.Nil error
- **THEN** `redis_pipeline_commands_total` SHALL increment by N with `status="error"`

### Requirement: Hook registration in NewRedisClient
When `config.EnableMetrics` is true, `NewRedisClient` SHALL register the `MetricsHook` on the client via `AddHook()` and start the background pool metrics collector. When `EnableMetrics` is false, no hook SHALL be registered.

#### Scenario: Metrics enabled
- **WHEN** `NewRedisClient` is called with `EnableMetrics: true`
- **THEN** the `MetricsHook` SHALL be registered and pool metrics goroutine SHALL start

#### Scenario: Metrics disabled
- **WHEN** `NewRedisClient` is called with `EnableMetrics: false`
- **THEN** no hook SHALL be registered and no pool metrics goroutine SHALL start

### Requirement: Pool metrics background collector
The package SHALL start a background goroutine (when metrics enabled) that polls `PoolStats()` every 10 seconds (or `config.MetricInterval` if set) and updates `redis_pool_total_conns`, `redis_pool_idle_conns` gauges, and `redis_pool_stale_conns_total` counter. For cluster mode, it SHALL aggregate across all shards via `ForEachShard`.

#### Scenario: Standalone pool metrics
- **WHEN** the pool metrics ticker fires
- **THEN** `redis_pool_total_conns{addr}` and `redis_pool_idle_conns{addr}` SHALL be set from `PoolStats()`, and `redis_pool_stale_conns_total{addr}` SHALL increment by the delta of `StaleConns`

#### Scenario: Cluster pool metrics
- **WHEN** the pool metrics ticker fires in cluster mode
- **THEN** metrics SHALL be aggregated across all shards with `addr="cluster"`

#### Scenario: Metrics goroutine stops on context cancellation
- **WHEN** the context passed to `NewRedisClient` is cancelled
- **THEN** the pool metrics goroutine SHALL exit cleanly

### Requirement: Prometheus metric registration with conflict handling
The package SHALL register all 7 Prometheus metric vectors in `init()`. If a metric is already registered (e.g., by v1 in the same process), it SHALL reuse the existing metric vector instead of failing.

#### Scenario: First registration
- **WHEN** `redis/v2` is the first to register metrics
- **THEN** all 7 metric vectors SHALL be registered successfully

#### Scenario: v1 already registered metrics
- **WHEN** `redis/v1` has already registered the same metric names
- **THEN** `redis/v2` SHALL reuse the existing metric vectors and both v1 and v2 SHALL write to the same metrics
