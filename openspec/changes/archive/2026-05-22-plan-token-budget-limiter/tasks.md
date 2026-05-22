## 1. Lua Scripts

- [x] 1.1 Create `pkg/limiter/lua/plan_sliding_window.lua` — Allow operation: HGETALL → sum valid buckets → check budget → HSET current bucket + model field → HDEL expired buckets → return {allowed, remaining, totalUsed}
- [x] 1.2 Create `pkg/limiter/lua/plan_refund.lua` — Refund operation: HGETALL → find current bucket → subtract tokens → update model field → clamp to 0 → return {remaining, totalUsed}
- [x] 1.3 Create `pkg/limiter/lua/plan_usage.lua` — Usage operation: HGETALL → sum valid buckets → collect model stats → return {remaining, totalUsed, modelBreakdown}

## 2. Go Core Implementation

- [x] 2.1 Create `pkg/limiter/plan_limiter.go` — Define `PlanLimiter` struct with redis.Client, three script SHA strings, and sync.Once for initialization
- [x] 2.2 Implement `NewPlanLimiter(ctx, *redis.Client)` — Constructor that pre-loads all three Lua scripts via ScriptLoad
- [x] 2.3 Implement `PlanConfig` and `PlanOption` — Config struct with Budget, Window, BucketSize, Weight fields; Option functions WithBudget, WithWindow, WithBucketSize, WithWeight
- [x] 2.4 Implement `PlanUsage` result struct — Fields: TotalUsed, Budget, Remaining, WindowEnd, ByModel map[string]int64
- [x] 2.5 Implement `Allow(ctx, key, model string, tokens int64, opts ...PlanOption) (bool, *PlanUsage, error)` — Execute plan_sliding_window.lua via EvalSHA, parse result
- [x] 2.6 Implement `Refund(ctx, key, model string, tokens int64, opts ...PlanOption) (*PlanUsage, error)` — Execute plan_refund.lua via EvalSHA, parse result
- [x] 2.7 Implement `Usage(ctx, key string, opts ...PlanOption) (*PlanUsage, error)` — Execute plan_usage.lua via EvalSHA, parse result

## 3. Integration Tests

- [x] 3.1 Add `github.com/alicebob/miniredis/v2` test dependency
- [x] 3.2 Create `pkg/limiter/plan_limiter_test.go` — Test helper: setup miniredis + redis.Client + PlanLimiter
- [x] 3.3 Test Allow within budget — Verify allowed=true, correct remaining
- [x] 3.4 Test Allow exceeding budget — Verify allowed=false, no deduction
- [x] 3.5 Test sliding window boundary — Advance miniredis clock, verify expired buckets excluded and cleaned
- [x] 3.6 Test first request initialization — No prior key, verify correct creation
- [x] 3.7 Test Refund tokens — Verify total and model field decrease correctly
- [x] 3.8 Test Refund clamps to zero — Refund more than consumed, verify no negative values
- [x] 3.9 Test Usage query — Verify totalUsed, remaining, ByModel breakdown
- [x] 3.10 Test Usage with expired window — All buckets outside window, verify zero usage
- [x] 3.11 Test multi-model tracking — Consume tokens for gpt4 and claude, verify per-model stats
- [x] 3.12 Test weighted pricing — Different weights for different models, verify weighted total
- [x] 3.13 Test custom window and bucket size — WithWindow(4h) + WithBucketSize(30m), verify behavior
- [x] 3.14 Test concurrent access — Parallel goroutines calling Allow, verify budget not exceeded
- [x] 3.15 Test Redis key TTL — Verify key expires after window + 3600s
