## ADDED Requirements

### Requirement: v9 Hook interface support
The Redis metrics system SHALL support both go-redis v8's `BeforeProcess`/`AfterProcess` Hook interface (via `redis/v1`) and go-redis v9's `ProcessHook`/`ProcessPipelineHook` middleware pattern (via `redis/v2`). Both implementations SHALL write to the same Prometheus metric vectors with identical names, labels, and bucket configurations.

#### Scenario: v1 and v2 coexistence
- **WHEN** both `redis/v1` and `redis/v2` clients are used in the same process with metrics enabled
- **THEN** both SHALL write to the same `redis_commands_total`, `redis_command_duration_seconds`, `redis_pipeline_commands_total`, `redis_pipeline_duration_seconds`, `redis_pool_idle_conns`, `redis_pool_total_conns`, and `redis_pool_stale_conns_total` metric vectors without conflicts

#### Scenario: Dashboard compatibility
- **WHEN** a service migrates from `redis/v1` to `redis/v2`
- **THEN** existing Grafana dashboards querying `redis_*` metrics SHALL continue to work without modification
