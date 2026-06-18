## Context

The `redis/v1` package wraps `go-redis v8` (`github.com/go-redis/redis/v8`) and provides:
- A `RedisClient` interface with String, Hash, Key, Counter, List, Set, Sorted Set operations
- A `Config` struct with connection pool settings (PoolSize, MinIdleConns, IdleTimeout, MaxConnAge)
- Prometheus metrics via a `redis.Hook` implementation (BeforeProcess/AfterProcess pattern)
- Background pool metrics collection (total/idle/stale connections via periodic PoolStats polling)
- Support for both standalone (`redis.Client`) and cluster (`redis.ClusterClient`) modes
- JSON encoding helpers for complex types

**Current problem**: v8's connection pool has no health check on checkout. Idle connections get silently closed by proxies, causing `io.EOF` errors when the client attempts to use them. This manifests as intermittent `Failed to update task progress in Redis: EOF` errors in production.

**go-redis v9 key differences**:
- Module path: `github.com/redis/go-redis/v9` (moved from `github.com/go-redis/redis/v8`)
- Hook API: `ProcessHook(next ProcessHookFunc) ProcessHookFunc` replaces `BeforeProcess`/`AfterProcess`
- Pool: `ConnMaxIdleTime` option for automatic idle connection eviction before use
- Context is first-class on all operations (already was in v8)

## Goals / Non-Goals

**Goals:**
- Provide `redis/v2` package with identical `RedisClient` interface contract as v1
- Leverage v9's `ConnMaxIdleTime` to eliminate stale connection EOF errors
- Preserve all 7 Prometheus metrics with identical names, labels, and semantics
- Support both standalone and cluster modes
- Maintain backward compatibility — v1 remains untouched

**Non-Goals:**
- Migrate existing callers from v1 to v2 (opt-in per service)
- Add new Redis commands not present in v1
- Change Grafana dashboards (v2 emits same metric names)
- Upgrade telemetry/OpenTelemetry integration (handled separately in `telemetry/redisotel.go`)

## Decisions

### Decision 1: New package path `redis/v2` instead of in-place upgrade

**Choice**: Create `redis/v2/` as a separate package.

**Rationale**: v8 and v9 have incompatible Hook APIs. An in-place upgrade would force all callers to change simultaneously. A separate package allows gradual, per-service migration.

**Alternatives considered**:
- In-place upgrade in `redis/v1/` → rejected: breaks all callers at once, no rollback path
- `redis/v9/` naming → rejected: Go convention uses `v2`, `v3` etc. for major version paths

### Decision 2: Adapt Hook to v9's ProcessHook pattern

**Choice**: Implement `ProcessHook` and `ProcessPipelineHook` that wrap the `next` function, measuring elapsed time around the call.

**v8 pattern** (current):
```go
func (h *MetricsHook) BeforeProcess(ctx, cmd) (ctx, error) { ... }
func (h *MetricsHook) AfterProcess(ctx, cmd) error { ... }
```

**v9 pattern** (new):
```go
func (h *MetricsHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
    return func(ctx context.Context, cmd redis.Cmder) error {
        start := time.Now()
        err := next(ctx, cmd)
        elapsed := time.Since(start).Seconds()
        // record metrics
        return err
    }
}
```

**Rationale**: v9's hook is a middleware/wrapper pattern, simpler and avoids context-based time passing.

### Decision 3: Add `ConnMaxIdleTime` to Config with sensible default

**Choice**: Add `ConnMaxIdleTime time.Duration` to Config, default to `60s`.

**Rationale**: 60s is shorter than most TCP proxy idle timeouts (typically 120-300s), ensuring the client proactively evicts idle connections before the proxy closes them. This directly addresses the EOF root cause.

**Alternatives considered**:
- Default `0` (disabled) → rejected: defeats the purpose of upgrading
- Default `300s` → rejected: too long, proxy may close connections at 120s

### Decision 4: Keep MaxConnAge for backward compatibility

**Choice**: Retain `MaxConnAge` field in Config, map to v9's `ConnMaxLifetime`.

**Rationale**: `MaxConnAge` was in v1's Config. v9 renamed it to `ConnMaxLifetime` but serves the same purpose. Keeping the field name avoids Config changes for callers.

### Decision 5: Reuse same Prometheus metric names

**Choice**: v2 registers to the same global metric vectors (`redis_commands_total`, etc.) as v1.

**Rationale**: Since both packages share the same process, they share the same Prometheus registry. Using identical metric names means Grafana dashboards work without modification. The `AlreadyRegisteredError` handling in `init()` already covers this.

**Risk**: If v1 and v2 are both used in the same process, they share metrics. This is intentional — same Redis, same dashboard.

### Decision 6: Pool metrics via PoolStats polling (same as v1)

**Choice**: Continue using a background goroutine with 10s ticker to poll `PoolStats()`.

**Rationale**: v9's `PoolStats()` API is identical to v8's. The `Hits`, `Misses`, `Timeouts` fields exist but we only track `TotalConns`, `IdleConns`, `StaleConns` (consistent with v1).

## Risks / Trade-offs

| Risk | Mitigation |
|------|-----------|
| v1 and v2 coexistence increases binary size | Both v8 and v9 are small (~2MB); acceptable trade-off for gradual migration |
| Same metric names from two packages could confuse | This is intentional — same metric, same dashboard. Callers migrating from v1→v2 see no metric gap |
| `ConnMaxIdleTime=60s` may cause excessive connection churn under bursty low-traffic | `MinIdleConns` (default 20) ensures a warm pool. Only connections idle > 60s AND above MinIdleConns get evicted |
| v9 breaking changes in edge cases (e.g., `redis.Nil` behavior) | v9 maintains `redis.Nil` sentinel error. Test suite covers nil-key scenarios |

## Migration Plan

1. **Phase 1 (this change)**: Create `redis/v2` package. No caller changes.
2. **Phase 2 (follow-up)**: Each service team updates their import from `redis/v1` to `redis/v2` independently.
3. **Phase 3 (future)**: Once all services migrated, deprecate `redis/v1`.

**Rollback**: If v2 causes issues in a service, revert that service's import back to `redis/v1`. No shared state.

## Open Questions

None. The design is straightforward — a wrapper upgrade with well-understood API changes.
