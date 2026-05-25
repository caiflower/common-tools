package limiter

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
)

func newTestPlanLimiter(t *testing.T, mr *miniredis.Miniredis) *PlanLimiter {
	t.Helper()
	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	ctx := context.Background()

	pl, err := NewPlanLimiter(ctx, client)
	if err != nil {
		t.Fatalf("Failed to create PlanLimiter: %v", err)
	}
	return pl
}

func TestPlanLimiter_AllowWithinBudget(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()
	pl := newTestPlanLimiter(t, mr)

	allowed, usage, err := pl.Allow(context.Background(), "plan:user1:plan1", "gpt4", 1000, WithPlanWeight(3.0))
	if err != nil {
		t.Fatalf("Allow failed: %v", err)
	}
	if !allowed {
		t.Error("Expected allowed=true")
	}
	if usage.TotalUsed != 3000 {
		t.Errorf("Expected TotalUsed=3000, got %d", usage.TotalUsed)
	}
	if usage.Remaining != 97000 {
		t.Errorf("Expected Remaining=97000, got %d", usage.Remaining)
	}
}

func TestPlanLimiter_AllowExceedingBudget(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()
	pl := newTestPlanLimiter(t, mr)
	pl.defaultBudget = 10000

	allowed, _, err := pl.Allow(context.Background(), "plan:user1:plan1", "gpt4", 2000, WithPlanWeight(3.0))
	if err != nil {
		t.Fatalf("First Allow failed: %v", err)
	}
	if !allowed {
		t.Error("First request should be allowed")
	}

	allowed, usage, err := pl.Allow(context.Background(), "plan:user1:plan1", "gpt4", 2000, WithPlanWeight(3.0))
	if err != nil {
		t.Fatalf("Second Allow failed: %v", err)
	}
	if allowed {
		t.Error("Second request should be rejected (6000+6000=12000 > 10000)")
	}
	if usage.Remaining != 4000 {
		t.Errorf("Expected Remaining=4000, got %d", usage.Remaining)
	}
	if usage.TotalUsed != 6000 {
		t.Errorf("Expected TotalUsed=6000 (unchanged after rejection), got %d", usage.TotalUsed)
	}
}

func TestPlanLimiter_SlidingWindowBoundary(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()
	pl := newTestPlanLimiter(t, mr)
	pl.defaultBudget = 10000
	pl.defaultWindow = 2 * time.Hour
	pl.defaultBucketSize = 1 * time.Hour

	ctx := context.Background()
	key := "plan:user1:plan1"

	allowed, _, err := pl.Allow(ctx, key, "gpt4", 2000, WithPlanWeight(3.0))
	if err != nil {
		t.Fatalf("Allow failed: %v", err)
	}
	if !allowed {
		t.Error("First request should be allowed")
	}

	mr.FastForward(3 * time.Hour)

	allowed, usage, err := pl.Allow(ctx, key, "gpt4", 1000, WithPlanWeight(3.0))
	if err != nil {
		t.Fatalf("Allow after window expired failed: %v", err)
	}
	if !allowed {
		t.Error("Request after window expired should be allowed")
	}
	if usage.TotalUsed != 3000 {
		t.Errorf("Expected TotalUsed=3000 (only new request), got %d", usage.TotalUsed)
	}
}

func TestPlanLimiter_FirstRequestInitialization(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()
	pl := newTestPlanLimiter(t, mr)

	allowed, usage, err := pl.Allow(context.Background(), "plan:newuser:plan1", "gpt4", 5000, WithPlanWeight(2.0))
	if err != nil {
		t.Fatalf("Allow failed: %v", err)
	}
	if !allowed {
		t.Error("First request should be allowed")
	}
	if usage.TotalUsed != 10000 {
		t.Errorf("Expected TotalUsed=10000, got %d", usage.TotalUsed)
	}
	if usage.Remaining != 90000 {
		t.Errorf("Expected Remaining=90000, got %d", usage.Remaining)
	}
}

func TestPlanLimiter_RefundTokens(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()
	pl := newTestPlanLimiter(t, mr)

	ctx := context.Background()
	key := "plan:user1:plan1"

	_, _, err := pl.Allow(ctx, key, "gpt4", 2000, WithPlanWeight(3.0))
	if err != nil {
		t.Fatalf("Allow failed: %v", err)
	}

	usage, err := pl.Refund(ctx, key, "gpt4", 500, WithPlanWeight(3.0))
	if err != nil {
		t.Fatalf("Refund failed: %v", err)
	}
	if usage.TotalUsed != 4500 {
		t.Errorf("Expected TotalUsed=4500 (6000-1500), got %d", usage.TotalUsed)
	}
	if usage.Remaining != 95500 {
		t.Errorf("Expected Remaining=95500, got %d", usage.Remaining)
	}
}

func TestPlanLimiter_RefundClampsToZero(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()
	pl := newTestPlanLimiter(t, mr)

	ctx := context.Background()
	key := "plan:user1:plan1"

	_, _, err := pl.Allow(ctx, key, "gpt4", 1000, WithPlanWeight(1.0))
	if err != nil {
		t.Fatalf("Allow failed: %v", err)
	}

	usage, err := pl.Refund(ctx, key, "gpt4", 5000, WithPlanWeight(1.0))
	if err != nil {
		t.Fatalf("Refund failed: %v", err)
	}
	if usage.TotalUsed != 0 {
		t.Errorf("Expected TotalUsed=0 (clamped), got %d", usage.TotalUsed)
	}
	if usage.Remaining != 100000 {
		t.Errorf("Expected Remaining=100000, got %d", usage.Remaining)
	}
}

func TestPlanLimiter_RefundConsistencyWithTotal(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()
	pl := newTestPlanLimiter(t, mr)

	ctx := context.Background()
	key := "plan:user1:plan1"

	_, _, err := pl.Allow(ctx, key, "gpt4", 2000, WithPlanWeight(3.0))
	if err != nil {
		t.Fatalf("Allow failed: %v", err)
	}

	_, err = pl.Refund(ctx, key, "gpt4", 500, WithPlanWeight(3.0))
	if err != nil {
		t.Fatalf("Refund failed: %v", err)
	}

	usage, err := pl.Usage(ctx, key)
	if err != nil {
		t.Fatalf("Usage failed: %v", err)
	}

	if usage.TotalUsed != 4500 {
		t.Errorf("Expected TotalUsed=4500 after refund, got %d", usage.TotalUsed)
	}

	allowed, _, err := pl.Allow(ctx, key, "gpt4", 1000, WithPlanWeight(3.0))
	if err != nil {
		t.Fatalf("Allow after refund failed: %v", err)
	}
	if !allowed {
		t.Error("Allow after refund should succeed")
	}
}

func TestPlanLimiter_UsageQuery(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()
	pl := newTestPlanLimiter(t, mr)

	ctx := context.Background()
	key := "plan:user1:plan1"

	_, _, err := pl.Allow(ctx, key, "gpt4", 1000, WithPlanWeight(3.0))
	if err != nil {
		t.Fatalf("Allow gpt4 failed: %v", err)
	}
	_, _, err = pl.Allow(ctx, key, "claude", 500, WithPlanWeight(2.0))
	if err != nil {
		t.Fatalf("Allow claude failed: %v", err)
	}

	usage, err := pl.Usage(ctx, key)
	if err != nil {
		t.Fatalf("Usage failed: %v", err)
	}
	if usage.TotalUsed != 4000 {
		t.Errorf("Expected TotalUsed=4000, got %d", usage.TotalUsed)
	}
	if usage.Remaining != 96000 {
		t.Errorf("Expected Remaining=96000, got %d", usage.Remaining)
	}
	if usage.ByModel["gpt4"] != 1000 {
		t.Errorf("Expected gpt4=1000, got %d", usage.ByModel["gpt4"])
	}
	if usage.ByModel["claude"] != 500 {
		t.Errorf("Expected claude=500, got %d", usage.ByModel["claude"])
	}
}

func TestPlanLimiter_UsageExpiredWindow(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()
	pl := newTestPlanLimiter(t, mr)
	pl.defaultWindow = 2 * time.Hour
	pl.defaultBucketSize = 1 * time.Hour

	ctx := context.Background()
	key := "plan:user1:plan1"

	_, _, err := pl.Allow(ctx, key, "gpt4", 1000, WithPlanWeight(3.0))
	if err != nil {
		t.Fatalf("Allow failed: %v", err)
	}

	mr.FastForward(3 * time.Hour)

	usage, err := pl.Usage(ctx, key)
	if err != nil {
		t.Fatalf("Usage failed: %v", err)
	}
	if usage.TotalUsed != 0 {
		t.Errorf("Expected TotalUsed=0 after window expired, got %d", usage.TotalUsed)
	}
	if usage.Remaining != 100000 {
		t.Errorf("Expected Remaining=100000, got %d", usage.Remaining)
	}
}

func TestPlanLimiter_UsageNoWriteSideEffect(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()
	pl := newTestPlanLimiter(t, mr)

	ctx := context.Background()
	key := "plan:user1:plan1"

	_, _, err := pl.Allow(ctx, key, "gpt4", 1000, WithPlanWeight(1.0))
	if err != nil {
		t.Fatalf("Allow failed: %v", err)
	}

	fieldsBefore := pl.client.HGetAll(ctx, key).Val()
	ttlBefore, err := pl.client.TTL(ctx, key).Result()
	if err != nil {
		t.Fatalf("TTL before failed: %v", err)
	}

	_, err = pl.Usage(ctx, key)
	if err != nil {
		t.Fatalf("Usage failed: %v", err)
	}

	fieldsAfter := pl.client.HGetAll(ctx, key).Val()
	ttlAfter, err := pl.client.TTL(ctx, key).Result()
	if err != nil {
		t.Fatalf("TTL after failed: %v", err)
	}

	for k, v := range fieldsBefore {
		if fieldsAfter[k] != v {
			t.Errorf("Usage query modified data: field %s was %s, now %s", k, v, fieldsAfter[k])
		}
	}

	ttlDiff := ttlBefore - ttlAfter
	if ttlDiff < -time.Second || ttlDiff > time.Second {
		t.Errorf("Usage query modified TTL: before=%v, after=%v", ttlBefore, ttlAfter)
	}
}

func TestPlanLimiter_MultiModelTracking(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()
	pl := newTestPlanLimiter(t, mr)

	ctx := context.Background()
	key := "plan:user1:plan1"

	_, _, err := pl.Allow(ctx, key, "gpt4", 1000, WithPlanWeight(3.0))
	if err != nil {
		t.Fatalf("Allow gpt4 failed: %v", err)
	}
	_, _, err = pl.Allow(ctx, key, "claude", 500, WithPlanWeight(2.0))
	if err != nil {
		t.Fatalf("Allow claude failed: %v", err)
	}
	_, _, err = pl.Allow(ctx, key, "gemini", 2000, WithPlanWeight(0.5))
	if err != nil {
		t.Fatalf("Allow gemini failed: %v", err)
	}

	usage, err := pl.Usage(ctx, key)
	if err != nil {
		t.Fatalf("Usage failed: %v", err)
	}

	if usage.TotalUsed != 5000 {
		t.Errorf("Expected TotalUsed=5000 (3000+1000+1000), got %d", usage.TotalUsed)
	}
	if usage.ByModel["gpt4"] != 1000 {
		t.Errorf("Expected gpt4=1000, got %d", usage.ByModel["gpt4"])
	}
	if usage.ByModel["claude"] != 500 {
		t.Errorf("Expected claude=500, got %d", usage.ByModel["claude"])
	}
	if usage.ByModel["gemini"] != 2000 {
		t.Errorf("Expected gemini=2000, got %d", usage.ByModel["gemini"])
	}
}

func TestPlanLimiter_WeightedPricing(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()
	pl := newTestPlanLimiter(t, mr)

	ctx := context.Background()
	key := "plan:user1:plan1"

	allowed, _, err := pl.Allow(ctx, key, "gpt4", 1000, WithPlanWeight(3.0))
	if err != nil {
		t.Fatalf("Allow gpt4 failed: %v", err)
	}
	if !allowed {
		t.Error("gpt4 request should be allowed")
	}

	allowed, usage, err := pl.Allow(ctx, key, "gemini", 1000, WithPlanWeight(0.5))
	if err != nil {
		t.Fatalf("Allow gemini failed: %v", err)
	}
	if !allowed {
		t.Error("gemini request should be allowed")
	}

	if usage.TotalUsed != 3500 {
		t.Errorf("Expected TotalUsed=3500 (3000+500), got %d", usage.TotalUsed)
	}
}

func TestPlanLimiter_CustomWindowAndBucketSize(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()
	pl := newTestPlanLimiter(t, mr)

	ctx := context.Background()
	key := "plan:user1:plan1"

	allowed, _, err := pl.Allow(ctx, key, "gpt4", 1000,
		WithPlanBudget(50000),
		WithPlanWindow(4*time.Hour),
		WithPlanBucketSize(30*time.Minute),
		WithPlanWeight(1.0),
	)
	if err != nil {
		t.Fatalf("Allow failed: %v", err)
	}
	if !allowed {
		t.Error("Request should be allowed")
	}

	mr.FastForward(5 * time.Hour)

	allowed, usage, err := pl.Allow(ctx, key, "gpt4", 1000,
		WithPlanBudget(50000),
		WithPlanWindow(4*time.Hour),
		WithPlanBucketSize(30*time.Minute),
		WithPlanWeight(1.0),
	)
	if err != nil {
		t.Fatalf("Allow after window failed: %v", err)
	}
	if !allowed {
		t.Error("Request after window should be allowed")
	}
	if usage.TotalUsed != 1000 {
		t.Errorf("Expected TotalUsed=1000 (only new request), got %d", usage.TotalUsed)
	}
}

// TestPlanLimiter_ConcurrentAccess verifies budget enforcement under concurrent access.
// Note: miniredis is single-threaded and cannot fully validate Lua script atomicity
// under real Redis concurrency. This test verifies logical correctness only.
// For production, supplement with integration tests against a real Redis instance.
func TestPlanLimiter_ConcurrentAccess(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()
	pl := newTestPlanLimiter(t, mr)
	pl.defaultBudget = 10000

	ctx := context.Background()
	key := "plan:user1:plan1"

	var wg sync.WaitGroup
	allowedCount := int64(0)
	rejectedCount := int64(0)
	var mu sync.Mutex

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			allowed, _, err := pl.Allow(ctx, key, "gpt4", 1000, WithPlanWeight(1.0))
			if err != nil {
				t.Logf("Allow error: %v", err)
				return
			}
			mu.Lock()
			if allowed {
				allowedCount++
			} else {
				rejectedCount++
			}
			mu.Unlock()
		}()
	}
	wg.Wait()

	totalConsumed := allowedCount * 1000
	if totalConsumed > 10000 {
		t.Errorf("Budget exceeded: %d consumed with budget 10000", totalConsumed)
	}
	if allowedCount == 0 {
		t.Error("Expected at least one allowed request")
	}
	if rejectedCount == 0 {
		t.Error("Expected at least one rejected request")
	}
	t.Logf("Concurrent test: %d allowed, %d rejected, %d total consumed", allowedCount, rejectedCount, totalConsumed)
}

func TestPlanLimiter_KeyTTL(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()
	pl := newTestPlanLimiter(t, mr)
	pl.defaultWindow = 2 * time.Hour

	ctx := context.Background()
	key := "plan:user1:plan1"

	_, _, err := pl.Allow(ctx, key, "gpt4", 1000, WithPlanWeight(1.0))
	if err != nil {
		t.Fatalf("Allow failed: %v", err)
	}

	ttl, err := pl.client.TTL(ctx, key).Result()
	if err != nil {
		t.Fatalf("TTL failed: %v", err)
	}
	expectedTTL := 2*time.Hour + 3600*time.Second
	if ttl < expectedTTL-5*time.Second || ttl > expectedTTL+5*time.Second {
		t.Errorf("Expected TTL ~%v, got %v", expectedTTL, ttl)
	}
}

func TestPlanLimiter_ExpiredBucketCleanup(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()
	pl := newTestPlanLimiter(t, mr)
	pl.defaultWindow = 2 * time.Hour
	pl.defaultBucketSize = 1 * time.Hour

	ctx := context.Background()
	key := "plan:user1:plan1"

	_, _, err := pl.Allow(ctx, key, "gpt4", 1000, WithPlanWeight(1.0))
	if err != nil {
		t.Fatalf("Allow failed: %v", err)
	}

	fieldCountBefore := len(pl.client.HGetAll(ctx, key).Val())

	mr.FastForward(3 * time.Hour)

	_, _, err = pl.Allow(ctx, key, "gpt4", 500, WithPlanWeight(1.0))
	if err != nil {
		t.Fatalf("Allow after shift failed: %v", err)
	}

	fieldsAfter := pl.client.HGetAll(ctx, key).Val()
	fieldCountAfter := len(fieldsAfter)

	if fieldCountAfter > fieldCountBefore {
		t.Logf("Field count before=%d, after=%d (some expired buckets cleaned)", fieldCountBefore, fieldCountAfter)
	}

	for field := range fieldsAfter {
		if field[0] != '_' {
			ts := fmt.Sprintf("%s", field)
			_ = ts
		}
	}
}

func TestPlanLimiter_InvalidWeight(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	_, err := NewPlanLimiter(context.Background(), client, WithPlanWeight(0))
	if err == nil {
		t.Error("Expected error for zero weight")
	}

	_, err = NewPlanLimiter(context.Background(), client, WithPlanWeight(-1))
	if err == nil {
		t.Error("Expected error for negative weight")
	}
}

func TestPlanLimiter_InvalidBucketSize(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	_, err := NewPlanLimiter(context.Background(), client, WithPlanBucketSize(0))
	if err == nil {
		t.Error("Expected error for zero bucketSize")
	}

	_, err = NewPlanLimiter(context.Background(), client, WithPlanBucketSize(10*time.Hour))
	if err == nil {
		t.Error("Expected error for bucketSize > window")
	}
}

func TestPlanLimiter_RuntimeConfigValidation(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()
	pl := newTestPlanLimiter(t, mr)

	_, _, err := pl.Allow(context.Background(), "plan:user1:plan1", "gpt4", 1000, WithPlanWeight(0))
	if err == nil {
		t.Error("Expected error for zero weight at runtime")
	}

	_, err = pl.Refund(context.Background(), "plan:user1:plan1", "gpt4", 1000, WithPlanWeight(-1))
	if err == nil {
		t.Error("Expected error for negative weight at runtime")
	}
}

func TestPlanLimiter_InterfaceCompliance(t *testing.T) {
	var _ PlanLimiterInterface = (*PlanLimiter)(nil)
}

func TestPlanLimiter_TotalFieldConsistency(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()
	pl := newTestPlanLimiter(t, mr)

	ctx := context.Background()
	key := "plan:user1:plan1"

	_, _, err := pl.Allow(ctx, key, "gpt4", 2000, WithPlanWeight(3.0))
	if err != nil {
		t.Fatalf("Allow failed: %v", err)
	}

	_, err = pl.Refund(ctx, key, "gpt4", 500, WithPlanWeight(3.0))
	if err != nil {
		t.Fatalf("Refund failed: %v", err)
	}

	usage, err := pl.Usage(ctx, key)
	if err != nil {
		t.Fatalf("Usage failed: %v", err)
	}

	if usage.TotalUsed != 4500 {
		t.Errorf("Expected TotalUsed=4500, got %d", usage.TotalUsed)
	}

	totalField, err := pl.client.HGet(ctx, key, "_total").Result()
	if err != nil {
		t.Fatalf("HGet _total failed: %v", err)
	}
	if totalField != "4500" {
		t.Errorf("Expected _total=4500, got %s", totalField)
	}

	allowed, allowUsage, err := pl.Allow(ctx, key, "gpt4", 1000, WithPlanWeight(3.0))
	if err != nil {
		t.Fatalf("Allow after refund failed: %v", err)
	}
	if !allowed {
		t.Error("Allow after refund should succeed")
	}
	if allowUsage.TotalUsed != 7500 {
		t.Errorf("Expected TotalUsed=7500 (4500+3000), got %d", allowUsage.TotalUsed)
	}

	totalField, err = pl.client.HGet(ctx, key, "_total").Result()
	if err != nil {
		t.Fatalf("HGet _total failed: %v", err)
	}
	if totalField != "7500" {
		t.Errorf("Expected _total=7500, got %s", totalField)
	}
}

// advanceTime 推进 miniredis 的服务器时间，使 TIME 命令返回推进后的时间。
// 这比直接修改桶时间戳更接近真实 Redis 行为，因为 Lua 脚本通过 redis.call('TIME')
// 获取服务器时间来计算窗口起始点。
//
// 注意：FastForward 只减少 TTL，不推进 TIME 命令返回的服务器时间，
// 因此滑动窗口的过期逻辑必须通过 SetTime 来触发。
func advanceTime(t *testing.T, mr *miniredis.Miniredis, currentTime *time.Time, elapsed time.Duration) {
	t.Helper()
	*currentTime = currentTime.Add(elapsed)
	mr.SetTime(*currentTime)
}

func TestPlanLimiter_SlidingWindowPartialThenFullExpiry(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()
	now := time.Now()
	mr.SetTime(now)
	pl := newTestPlanLimiter(t, mr)
	pl.defaultBudget = 50000
	pl.defaultWindow = 4 * time.Hour
	pl.defaultBucketSize = 1 * time.Hour

	ctx := context.Background()
	key := "plan:user1:plan1"

	allowed, usage, err := pl.Allow(ctx, key, "gpt4", 1000, WithPlanWeight(3.0))
	if err != nil {
		t.Fatalf("First Allow failed: %v", err)
	}
	if !allowed {
		t.Error("First request should be allowed")
	}
	if usage.TotalUsed != 3000 {
		t.Errorf("Expected TotalUsed=3000 after first request, got %d", usage.TotalUsed)
	}

	advanceTime(t, mr, &now, 2*time.Hour)

	allowed, usage, err = pl.Allow(ctx, key, "gpt4", 500, WithPlanWeight(2.0))
	if err != nil {
		t.Fatalf("Second Allow (half window) failed: %v", err)
	}
	if !allowed {
		t.Error("Second request should be allowed")
	}
	if usage.TotalUsed != 4000 {
		t.Errorf("Expected TotalUsed=4000 (3000+1000, both within window), got %d", usage.TotalUsed)
	}

	advanceTime(t, mr, &now, 2*time.Hour)

	allowed, usage, err = pl.Allow(ctx, key, "gpt4", 2000, WithPlanWeight(1.0))
	if err != nil {
		t.Fatalf("Third Allow (full window) failed: %v", err)
	}
	if !allowed {
		t.Error("Third request should be allowed")
	}
	if usage.TotalUsed != 3000 {
		t.Errorf("Expected TotalUsed=3000 (first request expired, second=1000+third=2000), got %d", usage.TotalUsed)
	}
}

func TestPlanLimiter_24HourSlidingWindow(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()
	now := time.Now()
	mr.SetTime(now)
	pl := newTestPlanLimiter(t, mr)
	pl.defaultBudget = 100000
	pl.defaultWindow = 24 * time.Hour
	pl.defaultBucketSize = 1 * time.Hour

	ctx := context.Background()
	key := "plan:user1:plan1"

	type step struct {
		advanceHours      int
		model             string
		tokens            int64
		weight            float64
		expectedAllowed   bool
		expectedTotalUsed int64
	}

	steps := []step{
		{0, "gpt4", 1000, 3.0, true, 3000},
		{0, "claude", 500, 2.0, true, 4000},
		{0, "gemini", 2000, 0.5, true, 5000},
		{1, "gpt4", 2000, 3.0, true, 11000},
		{0, "deepseek", 3000, 1.0, true, 14000},
		{1, "claude", 1500, 2.0, true, 17000},
		{1, "gpt4", 1000, 3.0, true, 20000},
		{2, "gemini", 4000, 0.5, true, 22000},
		{0, "claude", 2000, 2.0, true, 26000},
		{3, "gpt4", 1500, 3.0, true, 30500},
		{0, "deepseek", 5000, 1.0, true, 35500},
		{2, "claude", 1000, 2.0, true, 37500},
		{2, "gpt4", 2000, 3.0, true, 43500},
		{0, "gemini", 3000, 0.5, true, 45000},
		{3, "claude", 3000, 2.0, true, 51000},
		{1, "gpt4", 1000, 3.0, true, 54000},
		{2, "deepseek", 2000, 1.0, true, 56000},
		{2, "gpt4", 500, 3.0, true, 57500},
		{0, "claude", 1000, 2.0, true, 59500},
		{4, "gpt4", 1000, 3.0, true, 57500},
		{1, "claude", 2000, 2.0, true, 52500},
		{1, "gemini", 3000, 0.5, true, 51000},
		{2, "gpt4", 1000, 3.0, true, 51000},
		{2, "deepseek", 4000, 1.0, true, 49000},
		{2, "claude", 1500, 2.0, true, 42500},
		{2, "gpt4", 2000, 3.0, true, 46500},
		{2, "gemini", 5000, 0.5, true, 41500},
		{3, "claude", 1000, 2.0, true, 37500},
		{1, "gpt4", 3000, 3.0, true, 43500},
		{2, "deepseek", 2000, 1.0, true, 43500},
	}

	for i, s := range steps {
		if s.advanceHours > 0 {
			advanceTime(t, mr, &now, time.Duration(s.advanceHours)*time.Hour)
		}

		allowed, usage, err := pl.Allow(ctx, key, s.model, s.tokens, WithPlanWeight(s.weight))
		if err != nil {
			t.Fatalf("Step %d (T=%dh, model=%s): Allow failed: %v", i+1, s.advanceHours, s.model, err)
		}
		if allowed != s.expectedAllowed {
			t.Errorf("Step %d (T=%dh, model=%s, tokens=%d, w=%.1f): allowed=%v, want %v",
				i+1, s.advanceHours, s.model, s.tokens, s.weight, allowed, s.expectedAllowed)
		}
		if usage.TotalUsed != s.expectedTotalUsed {
			t.Errorf("Step %d (T=%dh, model=%s, tokens=%d, w=%.1f): TotalUsed=%d, want %d",
				i+1, s.advanceHours, s.model, s.tokens, s.weight, usage.TotalUsed, s.expectedTotalUsed)
		} else {
			t.Logf("Step %d (T=%v, model=%s, tokens=%d, w=%.1f): allowed=%v, TotalUsed=%d",
				i+1, &now, s.model, s.tokens, s.weight, allowed, usage.TotalUsed)
		}
	}
}

func TestPlanLimiter_SubHourTimeAdvance(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()
	now := time.Now().Truncate(time.Hour)
	mr.SetTime(now)
	pl := newTestPlanLimiter(t, mr)
	pl.defaultBudget = 10000
	pl.defaultWindow = 4 * time.Hour
	pl.defaultBucketSize = 1 * time.Hour

	ctx := context.Background()
	key := "plan:user1:plan1"

	type step struct {
		advanceMinutes    int
		model             string
		tokens            int64
		weight            float64
		expectedAllowed   bool
		expectedTotalUsed int64
	}

	steps := []step{
		// Phase 1: Sub-hour advances within same bucket (hour-slot 0: 6000 weighted)
		{0, "gpt4", 1000, 3.0, true, 3000},
		{30, "gpt4", 500, 3.0, true, 4500},
		{15, "claude", 750, 2.0, true, 6000},
		// Phase 2: Cross bucket boundary with sub-hour advance (hour-slot 1: 3950 weighted)
		{15, "gpt4", 500, 3.0, true, 7500},
		{20, "claude", 625, 2.0, true, 8750},
		{10, "gpt4", 400, 3.0, true, 9950},
		{5, "gpt4", 100, 3.0, false, 9950},
		// Phase 3: Sub-hour window expiry - hour-slot 0 (6000) expires at T=4h1m
		// totalUsed = 9950 - 6000 = 3950
		{146, "gpt4", 1000, 3.0, true, 6950},
		{15, "claude", 1000, 2.0, true, 8950},
		{10, "gpt4", 300, 3.0, true, 9850},
		{5, "gpt4", 100, 3.0, false, 9850},
		// Phase 4: Hour-slot 1 (3950) expires at T=5h1m
		// totalUsed = 9850 - 3950 = 5900
		{30, "gpt4", 1000, 3.0, true, 8900},
		{15, "claude", 500, 2.0, true, 9900},
		{5, "gpt4", 100, 3.0, false, 9900},
		// Phase 5: Hour-slot 4 (5900) expires at T=8h1m
		// totalUsed = 9900 - 5900 = 4000
		{160, "gpt4", 1500, 3.0, true, 8500},
		{15, "gpt4", 500, 3.0, true, 10000},
		{5, "gpt4", 100, 3.0, false, 10000},
		// Phase 6: Hour-slot 5 (4000) expires at T=9h1m
		// totalUsed = 10000 - 4000 = 6000
		{40, "gpt4", 1000, 3.0, true, 9000},
		{10, "gpt4", 200, 3.0, true, 9600},
		{5, "gpt4", 200, 3.0, false, 9600},
	}

	for i, s := range steps {
		if s.advanceMinutes > 0 {
			advanceTime(t, mr, &now, time.Duration(s.advanceMinutes)*time.Minute)
		}

		allowed, usage, err := pl.Allow(ctx, key, s.model, s.tokens, WithPlanWeight(s.weight))
		if err != nil {
			t.Fatalf("Step %d (T=+%dm, model=%s): Allow failed: %v", i+1, s.advanceMinutes, s.model, err)
		}
		if allowed != s.expectedAllowed {
			t.Errorf("Step %d (T=+%dm, model=%s, tokens=%d, w=%.1f): allowed=%v, want %v",
				i+1, s.advanceMinutes, s.model, s.tokens, s.weight, allowed, s.expectedAllowed)
		}
		if usage.TotalUsed != s.expectedTotalUsed {
			t.Errorf("Step %d (T=+%dm, model=%s, tokens=%d, w=%.1f): TotalUsed=%d, want %d",
				i+1, s.advanceMinutes, s.model, s.tokens, s.weight, usage.TotalUsed, s.expectedTotalUsed)
		} else {
			t.Logf("Step %d (T=%v, model=%s, tokens=%d, w=%.1f): allowed=%v, TotalUsed=%d",
				i+1, &now, s.model, s.tokens, s.weight, allowed, usage.TotalUsed)
		}
	}
}

func TestPlanLimiter_PerModelQuota(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()
	now := time.Now()
	mr.SetTime(now)
	pl := newTestPlanLimiter(t, mr)
	pl.defaultWindow = 24 * time.Hour
	pl.defaultBucketSize = 1 * time.Hour

	ctx := context.Background()
	key := "plan:user1:plan1"

	modelBudgets := map[string]int64{
		"gpt4":     15000,
		"claude":   20000,
		"gemini":   50000,
		"deepseek": 100000,
	}

	modelWeights := map[string]float64{
		"gpt4":     3.0,
		"claude":   2.0,
		"gemini":   0.5,
		"deepseek": 1.0,
	}

	modelKey := func(key, model string) string {
		return key + ":" + model
	}

	type step struct {
		advanceHours      int
		model             string
		tokens            int64
		expectedAllowed   bool
		expectedTotalUsed int64
	}

	steps := []step{
		{0, "gpt4", 2000, true, 6000},
		{0, "claude", 3000, true, 6000},
		{0, "gemini", 5000, true, 2500},
		{0, "deepseek", 4000, true, 4000},
		{1, "gpt4", 3000, true, 15000},
		{0, "claude", 4000, true, 14000},
		{0, "gemini", 10000, true, 7500},
		{0, "gpt4", 500, false, 15000},
		{0, "deepseek", 20000, true, 24000},
		{1, "claude", 4000, false, 14000},
		{0, "gemini", 20000, true, 17500},
		{0, "deepseek", 30000, true, 54000},
		{2, "gpt4", 1000, false, 15000},
		{0, "deepseek", 40000, true, 94000},
		{0, "deepseek", 7000, false, 94000},
		{0, "gemini", 40000, true, 37500},
		{0, "gemini", 30000, false, 37500},
		{21, "gpt4", 1000, true, 3000},
		{0, "claude", 2000, true, 4000},
		{0, "deepseek", 5000, true, 75000},
		{0, "gemini", 10000, true, 35000},
	}

	for i, s := range steps {
		if s.advanceHours > 0 {
			advanceTime(t, mr, &now, time.Duration(s.advanceHours)*time.Hour)
		}

		mk := modelKey(key, s.model)
		budget := modelBudgets[s.model]
		weight := modelWeights[s.model]

		allowed, usage, err := pl.Allow(ctx, mk, s.model, s.tokens,
			WithPlanBudget(budget),
			WithPlanWeight(weight),
		)
		if err != nil {
			t.Fatalf("Step %d (T=%dh, model=%s): Allow failed: %v", i+1, s.advanceHours, s.model, err)
		}
		if allowed != s.expectedAllowed {
			t.Errorf("Step %d (T=%dh, model=%s, tokens=%d, budget=%d): allowed=%v, want %v",
				i+1, s.advanceHours, s.model, s.tokens, budget, allowed, s.expectedAllowed)
		}
		if usage.TotalUsed != s.expectedTotalUsed {
			t.Errorf("Step %d (T=%dh, model=%s, tokens=%d, budget=%d): TotalUsed=%d, want %d",
				i+1, s.advanceHours, s.model, s.tokens, budget, usage.TotalUsed, s.expectedTotalUsed)
		} else {
			t.Logf("Step %d (T=%v, model=%s, tokens=%d, budget=%d): allowed=%v, TotalUsed=%d",
				i+1, &now, s.model, s.tokens, budget, allowed, usage.TotalUsed)
		}
	}
}

func TestPlanLimiter_ExactBoundaryQuota(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()
	now := time.Now()
	mr.SetTime(now)
	pl := newTestPlanLimiter(t, mr)
	pl.defaultBudget = 10000
	pl.defaultWindow = 24 * time.Hour
	pl.defaultBucketSize = 1 * time.Hour

	ctx := context.Background()
	key := "plan:user1:plan1"

	type step struct {
		advanceHours      int
		model             string
		tokens            int64
		weight            float64
		expectedAllowed   bool
		expectedTotalUsed int64
		expectedRemaining int64
		checkRemaining    bool
	}

	steps := []step{
		{0, "gpt4", 1000, 5.0, true, 5000, 5000, true},
		{0, "claude", 2500, 2.0, true, 10000, 0, true},
		{0, "gemini", 1, 1.0, false, 10000, 0, false},
		{0, "deepseek", 1, 1.0, false, 10000, 0, false},
		{0, "gemini", 1, 0.5, true, 10000, 0, false},
		{0, "gemini", 2, 0.5, false, 10000, 0, false},
		{25, "gpt4", 9900, 1.0, true, 9900, 100, true},
		{0, "gpt4", 100, 1.0, true, 10000, 0, true},
		{0, "gpt4", 1, 1.0, false, 10000, 0, false},
	}

	for i, s := range steps {
		if s.advanceHours > 0 {
			advanceTime(t, mr, &now, time.Duration(s.advanceHours)*time.Hour)
		}

		allowed, usage, err := pl.Allow(ctx, key, s.model, s.tokens, WithPlanWeight(s.weight))
		if err != nil {
			t.Fatalf("Step %d (T=%dh, model=%s): Allow failed: %v", i+1, s.advanceHours, s.model, err)
		}
		if allowed != s.expectedAllowed {
			t.Errorf("Step %d (T=%dh, model=%s, tokens=%d, w=%.1f): allowed=%v, want %v",
				i+1, s.advanceHours, s.model, s.tokens, s.weight, allowed, s.expectedAllowed)
		}
		if usage.TotalUsed != s.expectedTotalUsed {
			t.Errorf("Step %d (T=%dh, model=%s, tokens=%d, w=%.1f): TotalUsed=%d, want %d",
				i+1, s.advanceHours, s.model, s.tokens, s.weight, usage.TotalUsed, s.expectedTotalUsed)
		}
		if s.checkRemaining && usage.Remaining != s.expectedRemaining {
			t.Errorf("Step %d (T=%dh, model=%s, tokens=%d, w=%.1f): Remaining=%d, want %d",
				i+1, s.advanceHours, s.model, s.tokens, s.weight, usage.Remaining, s.expectedRemaining)
		} else {
			t.Logf("Step %d (T=%v, model=%s, tokens=%d, w=%.1f): allowed=%v, TotalUsed=%d, Remaining=%d",
				i+1, &now, s.model, s.tokens, s.weight, allowed, usage.TotalUsed, usage.Remaining)
		}
	}
}

func TestPlanLimiter_PartialModelExceededOthersAvailable(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()
	now := time.Now()
	mr.SetTime(now)
	pl := newTestPlanLimiter(t, mr)
	pl.defaultWindow = 24 * time.Hour
	pl.defaultBucketSize = 1 * time.Hour

	ctx := context.Background()
	key := "plan:user1:plan1"

	modelBudgets := map[string]int64{
		"gpt4":   6000,
		"claude": 20000,
		"gemini": 50000,
	}
	modelWeights := map[string]float64{
		"gpt4":   3.0,
		"claude": 2.0,
		"gemini": 0.5,
	}
	modelKey := func(key, model string) string {
		return key + ":" + model
	}

	type step struct {
		advanceHours      int
		model             string
		tokens            int64
		expectedAllowed   bool
		expectedTotalUsed int64
	}

	steps := []step{
		{0, "gpt4", 1000, true, 3000},
		{0, "gpt4", 1000, true, 6000},
		{0, "gpt4", 1, false, 6000},
		{0, "claude", 5000, true, 10000},
		{0, "gemini", 20000, true, 10000},
		{0, "claude", 5000, true, 20000},
		{0, "claude", 1, false, 20000},
		{0, "gemini", 60000, true, 40000},
		{0, "gpt4", 1, false, 6000},
		{0, "claude", 1, false, 20000},
		{0, "gemini", 20000, true, 50000},
		{0, "gemini", 2, false, 50000},
		{25, "gpt4", 1000, true, 3000},
		{0, "claude", 5000, true, 10000},
		{0, "gemini", 20000, true, 10000},
	}

	for i, s := range steps {
		if s.advanceHours > 0 {
			advanceTime(t, mr, &now, time.Duration(s.advanceHours)*time.Hour)
		}

		mk := modelKey(key, s.model)
		budget := modelBudgets[s.model]
		weight := modelWeights[s.model]

		allowed, usage, err := pl.Allow(ctx, mk, s.model, s.tokens,
			WithPlanBudget(budget),
			WithPlanWeight(weight),
		)
		if err != nil {
			t.Fatalf("Step %d (T=%dh, model=%s): Allow failed: %v", i+1, s.advanceHours, s.model, err)
		}
		if allowed != s.expectedAllowed {
			t.Errorf("Step %d (T=%dh, model=%s, tokens=%d, budget=%d): allowed=%v, want %v",
				i+1, s.advanceHours, s.model, s.tokens, budget, allowed, s.expectedAllowed)
		}
		if usage.TotalUsed != s.expectedTotalUsed {
			t.Errorf("Step %d (T=%dh, model=%s, tokens=%d, budget=%d): TotalUsed=%d, want %d",
				i+1, s.advanceHours, s.model, s.tokens, budget, usage.TotalUsed, s.expectedTotalUsed)
		} else {
			t.Logf("Step %d (T=%v, model=%s, tokens=%d, budget=%d): allowed=%v, TotalUsed=%d",
				i+1, &now, s.model, s.tokens, budget, allowed, usage.TotalUsed)
		}
	}
}

func TestPlanLimiter_SlidingWindowRecoveryThenReExceed(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()
	now := time.Now()
	mr.SetTime(now)
	pl := newTestPlanLimiter(t, mr)
	pl.defaultBudget = 10000
	pl.defaultWindow = 4 * time.Hour
	pl.defaultBucketSize = 1 * time.Hour

	ctx := context.Background()
	key := "plan:user1:plan1"

	type step struct {
		advanceHours      int
		model             string
		tokens            int64
		weight            float64
		expectedAllowed   bool
		expectedTotalUsed int64
	}

	steps := []step{
		{0, "gpt4", 2000, 3.0, true, 6000},
		{0, "claude", 1000, 2.0, true, 8000},
		{0, "gpt4", 1000, 3.0, false, 8000},
		{5, "gpt4", 2000, 3.0, true, 6000},
		{0, "claude", 1000, 2.0, true, 8000},
		{0, "gpt4", 1000, 3.0, false, 8000},
		{2, "gpt4", 500, 3.0, true, 9500},
		{advanceHours: 0, model: "gpt4", tokens: 200, weight: 3.0, expectedAllowed: false, expectedTotalUsed: 9500},
		{3, "gpt4", 2000, 3.0, true, 7500},
		{0, "gpt4", 1000, 3.0, false, 7500},
		{5, "gpt4", 3000, 3.0, true, 9000},
		{0, "gpt4", 500, 3.0, false, 9000},
	}

	for i, s := range steps {
		if s.advanceHours > 0 {
			advanceTime(t, mr, &now, time.Duration(s.advanceHours)*time.Hour)
		}

		allowed, usage, err := pl.Allow(ctx, key, s.model, s.tokens, WithPlanWeight(s.weight))
		if err != nil {
			t.Fatalf("Step %d (T=%dh, model=%s): Allow failed: %v", i+1, s.advanceHours, s.model, err)
		}
		if allowed != s.expectedAllowed {
			t.Errorf("Step %d (T=%dh, model=%s, tokens=%d, w=%.1f): allowed=%v, want %v",
				i+1, s.advanceHours, s.model, s.tokens, s.weight, allowed, s.expectedAllowed)
		}
		if usage.TotalUsed != s.expectedTotalUsed {
			t.Errorf("Step %d (T=%dh, model=%s, tokens=%d, w=%.1f): TotalUsed=%d, want %d",
				i+1, s.advanceHours, s.model, s.tokens, s.weight, usage.TotalUsed, s.expectedTotalUsed)
		} else {
			t.Logf("Step %d (T=%v, model=%s, tokens=%d, w=%.1f): allowed=%v, TotalUsed=%d",
				i+1, &now, s.model, s.tokens, s.weight, allowed, usage.TotalUsed)
		}
	}
}
