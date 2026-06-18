## Why

The current `redis/v1` package wraps `go-redis v8` (`github.com/go-redis/redis/v8`), which lacks built-in connection health checks. This causes persistent `EOF` errors in production when idle connections in the pool are silently closed by intermediary proxies or the Redis server itself, and the client picks up these dead connections without detection. go-redis v9 (`github.com/redis/go-redis/v9`) introduces `ConnMaxIdleTime` for proactive stale connection eviction and has a redesigned Hook API. Creating a `redis/v2` package allows a clean upgrade path without breaking existing v1 consumers.

## What Changes

- **New package `redis/v2`** wrapping `go-redis v9` with the same `RedisClient` interface contract as v1
- **Updated Hook API**: v9 replaces `BeforeProcess`/`AfterProcess` with `ProcessHook` (wraps the `next` function), and `BeforeProcessPipeline`/`AfterProcessPipeline` with `ProcessPipelineHook`
- **Built-in stale connection health check**: leverage v9's `ConnMaxIdleTime` option to proactively evict idle connections before they are closed by external proxies, eliminating the root cause of EOF errors
- **Same Prometheus metrics**: all 7 existing metrics (`redis_commands_total`, `redis_command_duration_seconds`, `redis_pipeline_commands_total`, `redis_pipeline_duration_seconds`, `redis_pool_idle_conns`, `redis_pool_total_conns`, `redis_pool_stale_conns_total`) preserved with identical labels and semantics
- **Same Config structure**: `Config` fields remain compatible; add `ConnMaxIdleTime` as a new optional field
- **BREAKING**: v2 is a separate import path (`redis/v2`); callers must update import statements when migrating. v1 remains available for gradual migration.

## Capabilities

### New Capabilities
- `redis-v2-client`: Redis client wrapper for go-redis v9, providing the same `RedisClient` interface, Config, encoding helpers, and cluster/standalone support as v1, with v9's improved connection pool management.
- `redis-v2-metrics`: Prometheus metrics hook adapted for go-redis v9's `ProcessHook`/`ProcessPipelineHook` API, collecting the same command-level and pool-level metrics as v1.

### Modified Capabilities
- `redis-prometheus-metrics`: v2 implementation uses the same metric names, labels, and buckets as v1. The spec's metric definitions remain unchanged; only the Hook interface implementation differs (v9 API vs v8 API).

## Impact

- **New dependency**: `github.com/redis/go-redis/v9` added to `go.mod`
- **Existing dependency**: `github.com/go-redis/redis/v8` retained (v1 still uses it)
- **New files**: `redis/v2/client.go`, `redis/v2/metric.go`, `redis/v2/client_test.go`, `redis/v2/metric_test.go`
- **No changes to v1**: `redis/v1/*` remains untouched for backward compatibility
- **Grafana dashboards**: no changes needed; v2 emits the same metric names
- **Callers**: migration is opt-in; each service can independently choose to upgrade from `redis/v1` to `redis/v2`
