package limiter

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
)

func setupKeyFixedWindow(t *testing.T, opts ...KeyFixedWindowOption) (*miniredis.Miniredis, *redis.Client, *KeyFixedWindowLimiter) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	ctx := context.Background()

	kl, err := NewKeyFixedWindowLimiter(ctx, client, opts...)
	if err != nil {
		t.Fatalf("Failed to create KeyFixedWindowLimiter: %v", err)
	}
	return mr, client, kl
}

func TestKeyFixedWindow_AcquireSuccess(t *testing.T) {
	mr, _, kl := setupKeyFixedWindow(t, WithDefaultMaxConcurrent(5))
	defer mr.Close()

	ctx := context.Background()
	key := "user:1"

	allowed, current, err := kl.Acquire(ctx, key)
	if err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}
	if !allowed {
		t.Fatal("First Acquire should be allowed")
	}
	if current != 1 {
		t.Fatalf("Expected current=1, got %d", current)
	}

	allowed, current, err = kl.Acquire(ctx, key)
	if err != nil {
		t.Fatalf("Second Acquire failed: %v", err)
	}
	if !allowed {
		t.Fatal("Second Acquire should be allowed (under limit)")
	}
	if current != 2 {
		t.Fatalf("Expected current=2, got %d", current)
	}
}

func TestKeyFixedWindow_AcquireAtLimit(t *testing.T) {
	mr, _, kl := setupKeyFixedWindow(t, WithDefaultMaxConcurrent(3))
	defer mr.Close()

	ctx := context.Background()
	key := "user:2"

	for i := 0; i < 3; i++ {
		allowed, _, err := kl.Acquire(ctx, key)
		if err != nil {
			t.Fatalf("Acquire %d failed: %v", i+1, err)
		}
		if !allowed {
			t.Fatalf("Acquire %d should be allowed", i+1)
		}
	}

	allowed, current, err := kl.Acquire(ctx, key)
	if err != nil {
		t.Fatalf("Acquire at limit failed: %v", err)
	}
	if allowed {
		t.Fatal("Acquire at limit should be rejected")
	}
	if current != 3 {
		t.Fatalf("Expected current=3, got %d", current)
	}
}

func TestKeyFixedWindow_ReleaseDecrements(t *testing.T) {
	mr, _, kl := setupKeyFixedWindow(t, WithDefaultMaxConcurrent(5))
	defer mr.Close()

	ctx := context.Background()
	key := "user:3"

	_, _, _ = kl.Acquire(ctx, key)
	_, _, _ = kl.Acquire(ctx, key)

	current, err := kl.CurrentConcurrent(ctx, key)
	if err != nil {
		t.Fatalf("CurrentConcurrent failed: %v", err)
	}
	if current != 2 {
		t.Fatalf("Expected current=2 before release, got %d", current)
	}

	newCount, err := kl.Release(ctx, key)
	if err != nil {
		t.Fatalf("Release failed: %v", err)
	}
	if newCount != 1 {
		t.Fatalf("Expected newCount=1 after release, got %d", newCount)
	}

	current, err = kl.CurrentConcurrent(ctx, key)
	if err != nil {
		t.Fatalf("CurrentConcurrent after release failed: %v", err)
	}
	if current != 1 {
		t.Fatalf("Expected current=1 after release, got %d", current)
	}
}

func TestKeyFixedWindow_ReleaseUnderflowProtection(t *testing.T) {
	mr, _, kl := setupKeyFixedWindow(t, WithDefaultMaxConcurrent(5))
	defer mr.Close()

	ctx := context.Background()
	key := "user:4"

	newCount, err := kl.Release(ctx, key)
	if err != nil {
		t.Fatalf("Release on empty key failed: %v", err)
	}
	if newCount != 0 {
		t.Fatalf("Expected newCount=0 on empty key, got %d", newCount)
	}

	_, _, _ = kl.Acquire(ctx, key)
	_, _ = kl.Release(ctx, key)
	_, _ = kl.Release(ctx, key)

	current, err := kl.CurrentConcurrent(ctx, key)
	if err != nil {
		t.Fatalf("CurrentConcurrent failed: %v", err)
	}
	if current != 0 {
		t.Fatalf("Expected current=0 after double release, got %d", current)
	}
}

func TestKeyFixedWindow_ReleaseNonExistentKey(t *testing.T) {
	mr, _, kl := setupKeyFixedWindow(t, WithDefaultMaxConcurrent(5))
	defer mr.Close()

	ctx := context.Background()
	key := "user:nonexistent"

	newCount, err := kl.Release(ctx, key)
	if err != nil {
		t.Fatalf("Release on non-existent key should not error: %v", err)
	}
	if newCount != 0 {
		t.Fatalf("Expected newCount=0 for non-existent key, got %d", newCount)
	}
}

func TestKeyFixedWindow_CurrentConcurrent(t *testing.T) {
	mr, _, kl := setupKeyFixedWindow(t, WithDefaultMaxConcurrent(5))
	defer mr.Close()

	ctx := context.Background()

	current, err := kl.CurrentConcurrent(ctx, "user:nonexistent")
	if err != nil {
		t.Fatalf("CurrentConcurrent on non-existent key failed: %v", err)
	}
	if current != 0 {
		t.Fatalf("Expected 0 for non-existent key, got %d", current)
	}

	key := "user:5"
	_, _, _ = kl.Acquire(ctx, key)
	_, _, _ = kl.Acquire(ctx, key)

	current, err = kl.CurrentConcurrent(ctx, key)
	if err != nil {
		t.Fatalf("CurrentConcurrent failed: %v", err)
	}
	if current != 2 {
		t.Fatalf("Expected current=2, got %d", current)
	}
}

func TestKeyFixedWindow_KeyIsolation(t *testing.T) {
	mr, _, kl := setupKeyFixedWindow(t, WithDefaultMaxConcurrent(2))
	defer mr.Close()

	ctx := context.Background()
	keyA := "user:A"
	keyB := "user:B"

	allowedA, _, err := kl.Acquire(ctx, keyA)
	if err != nil {
		t.Fatalf("Acquire keyA failed: %v", err)
	}
	if !allowedA {
		t.Fatal("Acquire keyA should be allowed")
	}

	allowedA2, _, err := kl.Acquire(ctx, keyA)
	if err != nil {
		t.Fatalf("Second Acquire keyA failed: %v", err)
	}
	if !allowedA2 {
		t.Fatal("Second Acquire keyA should be allowed")
	}

	allowedA3, _, err := kl.Acquire(ctx, keyA)
	if err != nil {
		t.Fatalf("Third Acquire keyA failed: %v", err)
	}
	if allowedA3 {
		t.Fatal("Third Acquire keyA should be rejected (at limit)")
	}

	allowedB, _, err := kl.Acquire(ctx, keyB)
	if err != nil {
		t.Fatalf("Acquire keyB failed: %v", err)
	}
	if !allowedB {
		t.Fatal("Acquire keyB should be allowed (independent of keyA)")
	}

	currentA, _ := kl.CurrentConcurrent(ctx, keyA)
	currentB, _ := kl.CurrentConcurrent(ctx, keyB)
	if currentA != 2 {
		t.Fatalf("Expected keyA current=2, got %d", currentA)
	}
	if currentB != 1 {
		t.Fatalf("Expected keyB current=1, got %d", currentB)
	}
}

func TestKeyFixedWindow_ConcurrentSafety(t *testing.T) {
	mr, _, kl := setupKeyFixedWindow(t, WithDefaultMaxConcurrent(50))
	defer mr.Close()

	ctx := context.Background()
	key := "user:concurrent"

	var (
		allowedCount int64
		wg           sync.WaitGroup
	)

	total := 200
	for i := 0; i < total; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ok, _, err := kl.Acquire(ctx, key)
			if err == nil && ok {
				atomic.AddInt64(&allowedCount, 1)
			}
		}()
	}
	wg.Wait()

	if allowedCount > 50 {
		t.Fatalf("Concurrent allowed count %d exceeded max 50", allowedCount)
	}
	if allowedCount == 0 {
		t.Fatal("At least one Acquire should be allowed")
	}
	t.Logf("Concurrent test: %d/%d allowed (max=50)", allowedCount, total)
}

func TestKeyFixedWindow_InvalidConfig(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()

	_, err := NewKeyFixedWindowLimiter(context.Background(), client, WithDefaultMaxConcurrent(0))
	if err == nil {
		t.Fatal("Expected error for zero maxConcurrent")
	}

	_, err = NewKeyFixedWindowLimiter(context.Background(), client, WithDefaultMaxConcurrent(-1))
	if err == nil {
		t.Fatal("Expected error for negative maxConcurrent")
	}

	_, err = NewKeyFixedWindowLimiter(context.Background(), client, WithDefaultExpiration(-1*time.Second))
	if err == nil {
		t.Fatal("Expected error for negative expiration")
	}

	_, err = NewKeyFixedWindowLimiter(context.Background(), client, WithDefaultExpiration(0))
	if err == nil {
		t.Fatal("Expected error for zero expiration")
	}
}

func TestKeyFixedWindow_InterfaceCompliance(t *testing.T) {
	var _ KeyFixedWindowLimiterInterface = (*KeyFixedWindowLimiter)(nil)
}

func TestKeyFixedWindow_TTLSetAndRefresh(t *testing.T) {
	mr, client, kl := setupKeyFixedWindow(t, WithDefaultMaxConcurrent(5), WithDefaultExpiration(1*time.Hour))
	defer mr.Close()

	ctx := context.Background()
	key := "user:ttl"
	redisKey := keyFixedWindowPrefix + key

	_, _, _ = kl.Acquire(ctx, key)

	ttl1, err := client.TTL(ctx, redisKey).Result()
	if err != nil {
		t.Fatalf("TTL after first Acquire failed: %v", err)
	}
	if ttl1 <= 0 {
		t.Fatal("Key should have positive TTL after Acquire")
	}

	mr.FastForward(30 * time.Minute)

	_, _, _ = kl.Acquire(ctx, key)

	ttl2, err := client.TTL(ctx, redisKey).Result()
	if err != nil {
		t.Fatalf("TTL after second Acquire failed: %v", err)
	}

	if ttl2 <= ttl1-30*time.Minute+5*time.Second {
		t.Fatalf("TTL should be refreshed after Acquire, got ttl1=%v ttl2=%v", ttl1, ttl2)
	}
}

func TestKeyFixedWindow_PerRequestOverride(t *testing.T) {
	mr, _, kl := setupKeyFixedWindow(t, WithDefaultMaxConcurrent(2))
	defer mr.Close()

	ctx := context.Background()
	key := "user:override"

	allowed, _, err := kl.Acquire(ctx, key, WithMaxConcurrent(1))
	if err != nil {
		t.Fatalf("First Acquire failed: %v", err)
	}
	if !allowed {
		t.Fatal("First Acquire with maxConcurrent=1 should be allowed")
	}

	allowed, current, err := kl.Acquire(ctx, key, WithMaxConcurrent(1))
	if err != nil {
		t.Fatalf("Second Acquire failed: %v", err)
	}
	if allowed {
		t.Fatal("Second Acquire with maxConcurrent=1 should be rejected")
	}
	if current != 1 {
		t.Fatalf("Expected current=1, got %d", current)
	}

	allowed, _, err = kl.Acquire(ctx, key, WithMaxConcurrent(5))
	if err != nil {
		t.Fatalf("Third Acquire with higher limit failed: %v", err)
	}
	if !allowed {
		t.Fatal("Third Acquire with maxConcurrent=5 should be allowed (current=1 < 5)")
	}
}

func TestKeyFixedWindow_AcquireReleaseCycle(t *testing.T) {
	mr, _, kl := setupKeyFixedWindow(t, WithDefaultMaxConcurrent(2))
	defer mr.Close()

	ctx := context.Background()
	key := "user:cycle"

	allowed, _, _ := kl.Acquire(ctx, key)
	if !allowed {
		t.Fatal("First Acquire should be allowed")
	}

	allowed, _, _ = kl.Acquire(ctx, key)
	if !allowed {
		t.Fatal("Second Acquire should be allowed")
	}

	allowed, _, _ = kl.Acquire(ctx, key)
	if allowed {
		t.Fatal("Third Acquire should be rejected (at limit)")
	}

	_, _ = kl.Release(ctx, key)

	allowed, _, _ = kl.Acquire(ctx, key)
	if !allowed {
		t.Fatal("Acquire after Release should be allowed")
	}
}

func TestKeyFixedWindow_KeyNamespacePrefix(t *testing.T) {
	mr, client, kl := setupKeyFixedWindow(t, WithDefaultMaxConcurrent(5))
	defer mr.Close()

	ctx := context.Background()
	key := "user:prefix"

	_, _, _ = kl.Acquire(ctx, key)

	redisKey := keyFixedWindowPrefix + key
	exists, err := client.Exists(ctx, redisKey).Result()
	if err != nil {
		t.Fatalf("Exists check failed: %v", err)
	}
	if exists != 1 {
		t.Fatalf("Expected key %s to exist in Redis", redisKey)
	}

	rawExists, err := client.Exists(ctx, key).Result()
	if err != nil {
		t.Fatalf("Raw key Exists check failed: %v", err)
	}
	if rawExists != 0 {
		t.Fatalf("Raw key %s should NOT exist (only prefixed key should)", key)
	}
}
