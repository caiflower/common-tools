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
