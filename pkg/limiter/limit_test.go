package limiter

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
)

func setupMiniredis(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	return mr, client
}

func newTestRedisLimiter(t *testing.T, mr *miniredis.Miniredis, client *redis.Client) *RedisLimiter {
	t.Helper()
	ctx := context.Background()
	rl, err := NewRedisLimiter(ctx, client)
	if err != nil {
		t.Fatalf("创建 RedisLimiter 失败: %v", err)
	}
	return rl
}

// simulateTimeElapsed 模拟时间流逝：将 Redis 中的 last_time 向前移动 elapsed 微秒，
// 使下次 Lua 脚本执行时计算出的 elapsed 与实际等待时间一致。
//
// 限制说明：
//   - miniredis 的 FastForward 只减少 TTL，不推进 TIME 命令返回的服务器时间，
//     因此令牌补充逻辑需要通过直接修改 last_time 来模拟。
//   - 此方法直接修改 Redis 内部状态，绕过了 Lua 脚本的原子性保证。
//     在真实 Redis 中，TIME 命令返回的是服务器时间，不能被客户端修改。
//   - 因此，通过此方法通过的测试不代表真实场景行为完全正确，
//     有条件时应补充基于真实 Redis 的集成测试。
func simulateTimeElapsed(t *testing.T, client *redis.Client, key string, elapsed time.Duration) {
	t.Helper()
	lastTimeStr, err := client.HGet(context.Background(), key, "last_time").Result()
	if err != nil {
		t.Fatalf("获取 last_time 失败: %v", err)
	}
	lastTimeMicros, err := strconv.ParseInt(lastTimeStr, 10, 64)
	if err != nil {
		t.Fatalf("解析 last_time 失败: %v", err)
	}
	newLastTime := lastTimeMicros - elapsed.Microseconds()
	if err := client.HSet(context.Background(), key, "last_time", newLastTime).Err(); err != nil {
		t.Fatalf("设置 last_time 失败: %v", err)
	}
}

func TestRedisLimiter_BasicAllow(t *testing.T) {
	mr, client := setupMiniredis(t)
	defer client.Close()
	rl := newTestRedisLimiter(t, mr, client)

	ctx := context.Background()
	key := "test:basic"
	capacity := int64(3)

	success := 0
	failed := 0
	total := int(capacity) + 2
	for i := 0; i < total; i++ {
		allowed, err := rl.Allow(ctx, key, WithCapacity(capacity), WithRate(1))
		if err != nil {
			t.Fatalf("第 %d 次请求出错: %v", i+1, err)
		}
		if allowed {
			success++
		} else {
			failed++
		}
	}

	fmt.Printf("BasicAllow: total=%d, success=%d, failed=%d\n", total, success, failed)
	if int64(success) > capacity {
		t.Fatalf("成功次数 %d 超出容量 %d", success, capacity)
	}
	if success == 0 {
		t.Fatal("至少应有 1 次请求被允许")
	}
}

func TestRedisLimiter_TokenReplenish(t *testing.T) {
	mr, client := setupMiniredis(t)
	defer client.Close()
	rl := newTestRedisLimiter(t, mr, client)

	ctx := context.Background()
	key := "test:replenish"
	capacity := int64(3)
	rate := int64(1)

	rejectedBefore := 0
	for i := 0; i < int(capacity)+5; i++ {
		allowed, err := rl.Allow(ctx, key, WithCapacity(capacity), WithRate(rate))
		if err != nil {
			t.Fatalf("耗尽阶段第 %d 次出错: %v", i+1, err)
		}
		if !allowed {
			rejectedBefore++
		}
	}
	fmt.Printf("TokenReplenish: 耗尽阶段 rejected=%d\n", rejectedBefore)
	if rejectedBefore == 0 {
		t.Fatal("发送超过容量的请求后，至少应有 1 次被限流拒绝")
	}

	simulateTimeElapsed(t, client, key, 3*time.Second)

	successAfter := 0
	for i := 0; i < 5; i++ {
		ok, e := rl.Allow(ctx, key, WithCapacity(capacity), WithRate(rate))
		if e != nil {
			t.Fatalf("补充后第 %d 次出错: %v", i+1, e)
		}
		if ok {
			successAfter++
		}
	}
	fmt.Printf("TokenReplenish: 补充后 success=%d\n", successAfter)
	if successAfter == 0 {
		t.Fatal("等待令牌补充后，至少应有 1 次请求被允许")
	}
}

func TestRedisLimiter_DefaultConfig(t *testing.T) {
	mr, client := setupMiniredis(t)
	defer client.Close()
	rl := newTestRedisLimiter(t, mr, client)

	ctx := context.Background()
	key := "test:default"
	defaultCapacity := int64(10)

	success := 0
	failed := 0
	total := int(defaultCapacity) * 2
	for i := 0; i < total; i++ {
		allowed, err := rl.Allow(ctx, key)
		if err != nil {
			t.Fatalf("第 %d 次请求出错: %v", i+1, err)
		}
		if allowed {
			success++
		} else {
			failed++
		}
	}

	fmt.Printf("DefaultConfig: total=%d, success=%d, failed=%d\n", total, success, failed)
	if success == 0 {
		t.Fatal("至少应有 1 次请求被允许")
	}
	if failed == 0 {
		t.Fatal("发送 2 倍容量的请求后，至少应有 1 次被限流拒绝")
	}
}

func TestRedisLimiter_WithRequested(t *testing.T) {
	mr, client := setupMiniredis(t)
	defer client.Close()
	rl := newTestRedisLimiter(t, mr, client)

	ctx := context.Background()
	key := "test:requested"
	capacity := int64(5)
	requested := int64(3)

	success := 0
	failed := 0
	total := 4
	for i := 0; i < total; i++ {
		allowed, err := rl.Allow(ctx, key, WithCapacity(capacity), WithRate(1), WithRequested(requested))
		if err != nil {
			t.Fatalf("第 %d 次请求出错: %v", i+1, err)
		}
		if allowed {
			success++
		} else {
			failed++
		}
	}

	fmt.Printf("WithRequested: total=%d, success=%d, failed=%d\n", total, success, failed)
	maxAllowed := capacity / requested
	if int64(success) > maxAllowed {
		t.Fatalf("成功次数 %d 超出预期上限 %d（容量=%d，每次消耗=%d）", success, maxAllowed, capacity, requested)
	}
	if success == 0 {
		t.Fatal("至少应有 1 次请求被允许")
	}
}

func TestRedisLimiter_Concurrent(t *testing.T) {
	mr, client := setupMiniredis(t)
	defer client.Close()
	rl := newTestRedisLimiter(t, mr, client)

	ctx := context.Background()
	key := "test:concurrent"
	capacity := int64(50)

	var (
		allowedCount int64
		wg           sync.WaitGroup
	)
	total := 200
	for i := 0; i < total; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ok, err := rl.Allow(ctx, key, WithCapacity(capacity), WithRate(1))
			if err == nil && ok {
				atomic.AddInt64(&allowedCount, 1)
			}
		}()
	}
	wg.Wait()

	fmt.Printf("Concurrent: total=%d, capacity=%d, allowed=%d\n", total, capacity, allowedCount)
	if allowedCount > capacity {
		t.Fatalf("并发场景下允许数量 %d 超出容量 %d", allowedCount, capacity)
	}
	if allowedCount == 0 {
		t.Fatal("至少应有 1 次请求被允许")
	}
}

func TestRedisLimiter_MultipleKeys(t *testing.T) {
	mr, client := setupMiniredis(t)
	defer client.Close()
	rl := newTestRedisLimiter(t, mr, client)

	ctx := context.Background()
	keys := []string{"test:multi:a", "test:multi:b", "test:multi:c"}
	capacity := int64(1)

	for _, key := range keys {
		success := 0
		for i := 0; i < 3; i++ {
			allowed, err := rl.Allow(ctx, key, WithCapacity(capacity), WithRate(1))
			if err != nil {
				t.Fatalf("key=%s 第 %d 次请求出错: %v", key, i+1, err)
			}
			if allowed {
				success++
			}
		}
		fmt.Printf("MultipleKeys: key=%s, success=%d\n", key, success)
		if int64(success) > capacity {
			t.Fatalf("key=%s 成功次数 %d 超出容量 %d", key, success, capacity)
		}
		if success == 0 {
			t.Fatalf("key=%s 至少应有 1 次请求被允许", key)
		}
	}
}

func TestRedisLimiter_HighConcurrency(t *testing.T) {
	mr, client := setupMiniredis(t)
	defer client.Close()
	rl := newTestRedisLimiter(t, mr, client)

	ctx := context.Background()
	key := "test:high-concurrency"

	var success, rejected, errors atomic.Int64
	group := sync.WaitGroup{}

	for i := 0; i < 10000; i++ {
		group.Add(1)
		go func(v int) {
			defer group.Done()
			allowed, err := rl.Allow(ctx, key, WithCapacity(1000), WithRate(1000))
			if err != nil {
				errors.Add(1)
				return
			}
			if allowed {
				success.Add(1)
			} else {
				rejected.Add(1)
			}
		}(i)
	}

	group.Wait()
	fmt.Printf("HighConcurrency: success=%d, rejected=%d, errors=%d\n", success.Load(), rejected.Load(), errors.Load())
	if errors.Load() > 0 {
		t.Fatalf("高并发测试中不应有 Redis 错误，但出现了 %d 次", errors.Load())
	}
	if success.Load() == 0 {
		t.Fatal("至少应有 1 次请求被允许")
	}
}

func TestRedisLimiter_ExactCapacityBoundary(t *testing.T) {
	mr, client := setupMiniredis(t)
	defer client.Close()
	rl := newTestRedisLimiter(t, mr, client)

	ctx := context.Background()
	key := "test:exact-capacity"
	capacity := int64(5)

	for i := int64(0); i < capacity; i++ {
		allowed, err := rl.Allow(ctx, key, WithCapacity(capacity), WithRate(1))
		if err != nil {
			t.Fatalf("第 %d 次请求出错: %v", i+1, err)
		}
		if !allowed {
			t.Fatalf("第 %d 次请求应被允许（未超出容量 %d）", i+1, capacity)
		}
	}

	allowed, err := rl.Allow(ctx, key, WithCapacity(capacity), WithRate(1))
	if err != nil {
		t.Fatalf("超出容量请求出错: %v", err)
	}
	if allowed {
		t.Fatal("超出容量的请求应被拒绝")
	}
}

func TestRedisLimiter_RequestedExceedsCapacity(t *testing.T) {
	mr, client := setupMiniredis(t)
	defer client.Close()
	rl := newTestRedisLimiter(t, mr, client)

	ctx := context.Background()
	key := "test:requested-exceeds"

	allowed, err := rl.Allow(ctx, key, WithCapacity(3), WithRate(1), WithRequested(5))
	if err != nil {
		t.Fatalf("请求出错: %v", err)
	}
	if allowed {
		t.Fatal("请求令牌数超过容量时应被拒绝")
	}
}

func TestRedisLimiter_RequestedEqualsCapacity(t *testing.T) {
	mr, client := setupMiniredis(t)
	defer client.Close()
	rl := newTestRedisLimiter(t, mr, client)

	ctx := context.Background()
	key := "test:requested-equals-capacity"
	capacity := int64(5)

	allowed, err := rl.Allow(ctx, key, WithCapacity(capacity), WithRate(1), WithRequested(capacity))
	if err != nil {
		t.Fatalf("请求出错: %v", err)
	}
	if !allowed {
		t.Fatal("请求令牌数等于容量时应被允许")
	}

	allowed, err = rl.Allow(ctx, key, WithCapacity(capacity), WithRate(1), WithRequested(1))
	if err != nil {
		t.Fatalf("第二次请求出错: %v", err)
	}
	if allowed {
		t.Fatal("令牌耗尽后应被拒绝")
	}
}

func TestRedisLimiter_SingleTokenCapacity(t *testing.T) {
	mr, client := setupMiniredis(t)
	defer client.Close()
	rl := newTestRedisLimiter(t, mr, client)

	ctx := context.Background()
	key := "test:single-token"

	allowed, err := rl.Allow(ctx, key, WithCapacity(1), WithRate(1))
	if err != nil {
		t.Fatalf("第一次请求出错: %v", err)
	}
	if !allowed {
		t.Fatal("容量为1时第一次请求应被允许")
	}

	allowed, err = rl.Allow(ctx, key, WithCapacity(1), WithRate(1))
	if err != nil {
		t.Fatalf("第二次请求出错: %v", err)
	}
	if allowed {
		t.Fatal("容量为1时第二次请求应被拒绝")
	}
}

func TestRedisLimiter_TokenReplenishPrecise(t *testing.T) {
	mr, client := setupMiniredis(t)
	defer client.Close()
	rl := newTestRedisLimiter(t, mr, client)

	ctx := context.Background()
	key := "test:replenish-precise"
	capacity := int64(5)
	rate := int64(2)

	for i := int64(0); i < capacity; i++ {
		_, _ = rl.Allow(ctx, key, WithCapacity(capacity), WithRate(rate))
	}

	allowed, _ := rl.Allow(ctx, key, WithCapacity(capacity), WithRate(rate))
	if allowed {
		t.Fatal("令牌耗尽后应被拒绝")
	}

	simulateTimeElapsed(t, client, key, 1*time.Second)

	allowedCount := 0
	for i := 0; i < 5; i++ {
		ok, _ := rl.Allow(ctx, key, WithCapacity(capacity), WithRate(rate))
		if ok {
			allowedCount++
		}
	}
	fmt.Printf("TokenReplenishPrecise: after 1s replenish at rate=2, allowed=%d\n", allowedCount)
	if allowedCount == 0 {
		t.Fatal("等待1秒后（速率2/s），至少应有1个令牌被补充")
	}
	if allowedCount > 2 {
		t.Fatalf("等待1秒后（速率2/s），最多补充2个令牌，但允许了 %d 次", allowedCount)
	}
}

func TestRedisLimiter_FullReplenishAfterDrain(t *testing.T) {
	mr, client := setupMiniredis(t)
	defer client.Close()
	rl := newTestRedisLimiter(t, mr, client)

	ctx := context.Background()
	key := "test:full-replenish"
	capacity := int64(5)
	rate := int64(1)

	for i := int64(0); i < capacity; i++ {
		_, _ = rl.Allow(ctx, key, WithCapacity(capacity), WithRate(rate))
	}

	simulateTimeElapsed(t, client, key, 10*time.Second)

	successCount := 0
	for i := int64(0); i < capacity; i++ {
		ok, _ := rl.Allow(ctx, key, WithCapacity(capacity), WithRate(rate))
		if ok {
			successCount++
		}
	}
	fmt.Printf("FullReplenish: after 10s replenish, success=%d (capacity=%d)\n", successCount, capacity)
	if int64(successCount) < capacity {
		t.Fatalf("等待足够长时间后应恢复全部容量，期望 %d，实际 %d", capacity, successCount)
	}
}

func TestRedisLimiter_KeyExpiration(t *testing.T) {
	mr, client := setupMiniredis(t)
	defer client.Close()
	rl := newTestRedisLimiter(t, mr, client)

	ctx := context.Background()
	key := "test:expiration"
	capacity := int64(3)
	rate := int64(1)

	_, err := rl.Allow(ctx, key, WithCapacity(capacity), WithRate(rate))
	if err != nil {
		t.Fatalf("请求出错: %v", err)
	}

	ttl, err := client.TTL(ctx, key).Result()
	if err != nil {
		t.Fatalf("获取 TTL 出错: %v", err)
	}
	if ttl <= 0 {
		t.Fatal("Key 应有正的 TTL")
	}
	fmt.Printf("KeyExpiration: TTL=%v\n", ttl)
}

func TestRedisLimiter_KeyExpiredAndRecreated(t *testing.T) {
	mr, client := setupMiniredis(t)
	defer client.Close()
	rl := newTestRedisLimiter(t, mr, client)

	ctx := context.Background()
	key := "test:expired-recreate"
	capacity := int64(3)
	rate := int64(1)

	for i := int64(0); i < capacity; i++ {
		_, _ = rl.Allow(ctx, key, WithCapacity(capacity), WithRate(rate))
	}

	allowed, _ := rl.Allow(ctx, key, WithCapacity(capacity), WithRate(rate))
	if allowed {
		t.Fatal("令牌耗尽后应被拒绝")
	}

	ttl, _ := client.TTL(ctx, key).Result()
	mr.FastForward(ttl + time.Second)

	exists, err := client.Exists(ctx, key).Result()
	if err != nil {
		t.Fatalf("检查 key 是否存在出错: %v", err)
	}
	if exists == 1 {
		t.Fatal("TTL 过期后 key 应不存在")
	}

	allowed, err = rl.Allow(ctx, key, WithCapacity(capacity), WithRate(rate))
	if err != nil {
		t.Fatalf("重建后请求出错: %v", err)
	}
	if !allowed {
		t.Fatal("key 过期重建后应重新获得全部令牌")
	}
}

func TestRedisLimiter_BurstThenRecover(t *testing.T) {
	mr, client := setupMiniredis(t)
	defer client.Close()
	rl := newTestRedisLimiter(t, mr, client)

	ctx := context.Background()
	key := "test:burst-recover"
	capacity := int64(10)
	rate := int64(5)

	success := 0
	for i := 0; i < int(capacity)+5; i++ {
		ok, _ := rl.Allow(ctx, key, WithCapacity(capacity), WithRate(rate))
		if ok {
			success++
		}
	}
	if int64(success) > capacity {
		t.Fatalf("突发请求成功数 %d 超出容量 %d", success, capacity)
	}

	simulateTimeElapsed(t, client, key, 2*time.Second)

	recoveredCount := 0
	for i := 0; i < 15; i++ {
		ok, _ := rl.Allow(ctx, key, WithCapacity(capacity), WithRate(rate))
		if ok {
			recoveredCount++
		}
	}
	fmt.Printf("BurstThenRecover: burst=%d, recovered=%d\n", success, recoveredCount)
	if recoveredCount == 0 {
		t.Fatal("等待恢复后应至少有1次请求被允许")
	}
}

func TestRedisLimiter_DifferentRates(t *testing.T) {
	mr, client := setupMiniredis(t)
	defer client.Close()

	tests := []struct {
		name     string
		capacity int64
		rate     int64
	}{
		{"LowRate", 5, 1},
		{"MediumRate", 10, 5},
		{"HighRate", 100, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rl := newTestRedisLimiter(t, mr, client)
			ctx := context.Background()
			key := fmt.Sprintf("test:rate:%s", tt.name)

			success := 0
			for i := 0; i < int(tt.capacity)+5; i++ {
				ok, _ := rl.Allow(ctx, key, WithCapacity(tt.capacity), WithRate(tt.rate))
				if ok {
					success++
				}
			}
			if int64(success) > tt.capacity {
				t.Fatalf("rate=%d: 成功数 %d 超出容量 %d", tt.rate, success, tt.capacity)
			}
			if success == 0 {
				t.Fatalf("rate=%d: 至少应有1次请求被允许", tt.rate)
			}
		})
	}
}

func TestRedisLimiter_MultipleInstances(t *testing.T) {
	_, client := setupMiniredis(t)
	defer client.Close()

	rl1, err := NewRedisLimiter(context.Background(), client)
	if err != nil {
		t.Fatalf("创建 rl1 失败: %v", err)
	}

	rl2, err := NewRedisLimiter(context.Background(), client)
	if err != nil {
		t.Fatalf("创建 rl2 失败: %v", err)
	}

	if rl1 == rl2 {
		t.Fatal("每次调用 NewRedisLimiter 应创建新实例")
	}
}

func TestRedisLimiter_SameKeyAcrossInstances(t *testing.T) {
	_, client := setupMiniredis(t)
	defer client.Close()

	rl1, err := NewRedisLimiter(context.Background(), client)
	if err != nil {
		t.Fatalf("创建 rl1 失败: %v", err)
	}

	ctx := context.Background()
	key := "test:shared-key"
	capacity := int64(3)

	for i := int64(0); i < capacity; i++ {
		_, _ = rl1.Allow(ctx, key, WithCapacity(capacity), WithRate(1))
	}

	allowed, err := rl1.Allow(ctx, key, WithCapacity(capacity), WithRate(1))
	if err != nil {
		t.Fatalf("请求出错: %v", err)
	}
	if allowed {
		t.Fatal("同一实例上令牌耗尽后应被拒绝")
	}
}

func TestRedisLimiter_MultipleRequestedTokens(t *testing.T) {
	mr, client := setupMiniredis(t)
	defer client.Close()
	rl := newTestRedisLimiter(t, mr, client)

	ctx := context.Background()
	key := "test:multi-requested"
	capacity := int64(10)
	requested := int64(3)

	success := 0
	for i := 0; i < 10; i++ {
		ok, _ := rl.Allow(ctx, key, WithCapacity(capacity), WithRate(1), WithRequested(requested))
		if ok {
			success++
		}
	}

	maxAllowed := capacity / requested
	fmt.Printf("MultipleRequestedTokens: capacity=%d, requested=%d, success=%d, maxAllowed=%d\n",
		capacity, requested, success, maxAllowed)
	if int64(success) > maxAllowed {
		t.Fatalf("成功数 %d 超出预期上限 %d", success, maxAllowed)
	}
}

func TestRedisLimiter_PartialReplenish(t *testing.T) {
	mr, client := setupMiniredis(t)
	defer client.Close()
	rl := newTestRedisLimiter(t, mr, client)

	ctx := context.Background()
	key := "test:partial-replenish"
	capacity := int64(10)
	rate := int64(2)

	for i := int64(0); i < capacity; i++ {
		_, _ = rl.Allow(ctx, key, WithCapacity(capacity), WithRate(rate))
	}

	simulateTimeElapsed(t, client, key, 1500*time.Millisecond)

	successCount := 0
	for i := 0; i < 10; i++ {
		ok, _ := rl.Allow(ctx, key, WithCapacity(capacity), WithRate(rate))
		if ok {
			successCount++
		}
	}
	fmt.Printf("PartialReplenish: after 1.5s at rate=2, allowed=%d\n", successCount)
	if successCount == 0 {
		t.Fatal("部分补充后应至少有1次请求被允许")
	}
	if successCount > 3 {
		t.Fatalf("1.5秒补充最多3个令牌（速率2/s），但允许了 %d 次", successCount)
	}
}

func TestRedisLimiter_ReplenishCappedAtCapacity(t *testing.T) {
	mr, client := setupMiniredis(t)
	defer client.Close()
	rl := newTestRedisLimiter(t, mr, client)

	ctx := context.Background()
	key := "test:replenish-capped"
	capacity := int64(5)
	rate := int64(1)

	_, _ = rl.Allow(ctx, key, WithCapacity(capacity), WithRate(rate))

	simulateTimeElapsed(t, client, key, 100*time.Second)

	successCount := 0
	for i := int64(0); i < capacity; i++ {
		ok, _ := rl.Allow(ctx, key, WithCapacity(capacity), WithRate(rate))
		if ok {
			successCount++
		}
	}
	fmt.Printf("ReplenishCappedAtCapacity: after long wait, success=%d (capacity=%d)\n", successCount, capacity)
	if int64(successCount) < capacity {
		t.Fatalf("长时间等待后令牌应恢复到容量上限，期望 %d，实际 %d", capacity, successCount)
	}

	ok, _ := rl.Allow(ctx, key, WithCapacity(capacity), WithRate(rate))
	if ok {
		t.Fatal("令牌数不应超过容量上限")
	}
}

func TestRedisLimiter_EmptyKey(t *testing.T) {
	mr, client := setupMiniredis(t)
	defer client.Close()
	rl := newTestRedisLimiter(t, mr, client)

	ctx := context.Background()

	allowed, err := rl.Allow(ctx, "", WithCapacity(5), WithRate(1))
	if err != nil {
		t.Fatalf("空 key 请求出错: %v", err)
	}
	if !allowed {
		t.Fatal("空 key 也应正常工作")
	}
}

func TestRedisLimiter_LargeCapacity(t *testing.T) {
	mr, client := setupMiniredis(t)
	defer client.Close()
	rl := newTestRedisLimiter(t, mr, client)

	ctx := context.Background()
	key := "test:large-capacity"
	capacity := int64(10000)

	success := 0
	for i := int64(0); i < capacity; i++ {
		ok, _ := rl.Allow(ctx, key, WithCapacity(capacity), WithRate(1000))
		if ok {
			success++
		}
	}
	if int64(success) != capacity {
		t.Fatalf("大容量场景：期望 %d 次成功，实际 %d", capacity, success)
	}
}

func TestRedisLimiter_RequestedOneAfterPartialDrain(t *testing.T) {
	mr, client := setupMiniredis(t)
	defer client.Close()
	rl := newTestRedisLimiter(t, mr, client)

	ctx := context.Background()
	key := "test:partial-drain"
	capacity := int64(10)
	rate := int64(1)

	for i := 0; i < 7; i++ {
		_, _ = rl.Allow(ctx, key, WithCapacity(capacity), WithRate(rate))
	}

	allowed, err := rl.Allow(ctx, key, WithCapacity(capacity), WithRate(rate))
	if err != nil {
		t.Fatalf("请求出错: %v", err)
	}
	if !allowed {
		t.Fatal("部分消耗后剩余令牌应足够")
	}

	simulateTimeElapsed(t, client, key, 5*time.Second)

	recoveredCount := 0
	for i := 0; i < 5; i++ {
		ok, _ := rl.Allow(ctx, key, WithCapacity(capacity), WithRate(rate))
		if ok {
			recoveredCount++
		}
	}
	fmt.Printf("RequestedOneAfterPartialDrain: recovered=%d\n", recoveredCount)
	if recoveredCount == 0 {
		t.Fatal("等待补充后应至少有1次请求被允许")
	}
}

func TestRedisLimiter_ConcurrentDifferentKeys(t *testing.T) {
	mr, client := setupMiniredis(t)
	defer client.Close()
	rl := newTestRedisLimiter(t, mr, client)

	ctx := context.Background()
	capacity := int64(10)
	numKeys := 5

	var wg sync.WaitGroup
	results := make([]int64, numKeys)

	for k := 0; k < numKeys; k++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			key := fmt.Sprintf("test:concurrent-key:%d", idx)
			var count int64
			for i := 0; i < int(capacity)+5; i++ {
				ok, _ := rl.Allow(ctx, key, WithCapacity(capacity), WithRate(1))
				if ok {
					count++
				}
			}
			results[idx] = count
		}(k)
	}
	wg.Wait()

	for k, count := range results {
		if count > capacity {
			t.Errorf("key %d: 成功数 %d 超出容量 %d", k, count, capacity)
		}
		if count == 0 {
			t.Errorf("key %d: 至少应有1次请求被允许", k)
		}
	}
}

func TestRedisLimiter_TokenStateInRedis(t *testing.T) {
	mr, client := setupMiniredis(t)
	defer client.Close()
	rl := newTestRedisLimiter(t, mr, client)

	ctx := context.Background()
	key := "test:redis-state"
	capacity := int64(5)
	rate := int64(1)

	_, err := rl.Allow(ctx, key, WithCapacity(capacity), WithRate(rate))
	if err != nil {
		t.Fatalf("请求出错: %v", err)
	}

	tokensStr, err := client.HGet(ctx, key, "tokens").Result()
	if err != nil {
		t.Fatalf("获取 tokens 出错: %v", err)
	}
	tokens, err := strconv.ParseInt(tokensStr, 10, 64)
	if err != nil {
		t.Fatalf("解析 tokens 出错: %v", err)
	}
	if tokens != capacity-1 {
		t.Fatalf("消耗1个令牌后，剩余应为 %d，实际 %d", capacity-1, tokens)
	}

	lastTimeStr, err := client.HGet(ctx, key, "last_time").Result()
	if err != nil {
		t.Fatalf("获取 last_time 出错: %v", err)
	}
	if lastTimeStr == "" {
		t.Fatal("last_time 不应为空")
	}
}

func TestRedisLimiter_ReplenishCalculation(t *testing.T) {
	mr, client := setupMiniredis(t)
	defer client.Close()
	rl := newTestRedisLimiter(t, mr, client)

	ctx := context.Background()
	key := "test:replenish-calc"
	capacity := int64(10)
	rate := int64(5)

	for i := int64(0); i < capacity; i++ {
		_, _ = rl.Allow(ctx, key, WithCapacity(capacity), WithRate(rate))
	}

	tokensStr, _ := client.HGet(ctx, key, "tokens").Result()
	tokens, _ := strconv.ParseInt(tokensStr, 10, 64)
	fmt.Printf("ReplenishCalculation: after drain, tokens=%d\n", tokens)
	if tokens > 0 {
		t.Fatalf("耗尽后令牌应为0或接近0，实际 %d", tokens)
	}

	simulateTimeElapsed(t, client, key, 1*time.Second)

	allowed, err := rl.Allow(ctx, key, WithCapacity(capacity), WithRate(rate))
	if err != nil {
		t.Fatalf("补充后请求出错: %v", err)
	}
	if !allowed {
		t.Fatal("1秒后（速率5/s）应至少补充5个令牌，请求应被允许")
	}

	tokensStr, _ = client.HGet(ctx, key, "tokens").Result()
	tokens, _ = strconv.ParseInt(tokensStr, 10, 64)
	fmt.Printf("ReplenishCalculation: after 1s replenish and 1 consume, tokens=%d\n", tokens)
}

func TestRedisLimiter_ReplenishWithDifferentRates(t *testing.T) {
	mr, client := setupMiniredis(t)
	defer client.Close()

	tests := []struct {
		name        string
		capacity    int64
		rate        int64
		elapsed     time.Duration
		expectedMin int64
		expectedMax int64
	}{
		{"Rate1_3s", 10, 1, 3 * time.Second, 3, 3},
		{"Rate5_2s", 10, 5, 2 * time.Second, 10, 10},
		{"Rate10_1s", 20, 10, 1 * time.Second, 10, 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rl := newTestRedisLimiter(t, mr, client)
			ctx := context.Background()
			key := fmt.Sprintf("test:replenish-rate:%s", tt.name)

			for i := int64(0); i < tt.capacity; i++ {
				_, _ = rl.Allow(ctx, key, WithCapacity(tt.capacity), WithRate(tt.rate))
			}

			simulateTimeElapsed(t, client, key, tt.elapsed)

			successCount := int64(0)
			for i := int64(0); i < tt.expectedMax+2; i++ {
				ok, _ := rl.Allow(ctx, key, WithCapacity(tt.capacity), WithRate(tt.rate))
				if ok {
					successCount++
				}
			}
			fmt.Printf("ReplenishWithRate %s: rate=%d, elapsed=%v, allowed=%d (expected %d-%d)\n",
				tt.name, tt.rate, tt.elapsed, successCount, tt.expectedMin, tt.expectedMax)
			if successCount < tt.expectedMin {
				t.Fatalf("补充令牌数不足：期望至少 %d，实际 %d", tt.expectedMin, successCount)
			}
			if successCount > tt.expectedMax {
				t.Fatalf("补充令牌数超出：期望最多 %d，实际 %d", tt.expectedMax, successCount)
			}
		})
	}
}

func TestRedisLimiter_FirstRequestInitializesFullCapacity(t *testing.T) {
	mr, client := setupMiniredis(t)
	defer client.Close()
	rl := newTestRedisLimiter(t, mr, client)

	ctx := context.Background()
	key := "test:first-request"
	capacity := int64(100)

	allowed, err := rl.Allow(ctx, key, WithCapacity(capacity), WithRate(1))
	if err != nil {
		t.Fatalf("首次请求出错: %v", err)
	}
	if !allowed {
		t.Fatal("首次请求应被允许")
	}

	tokensStr, _ := client.HGet(ctx, key, "tokens").Result()
	tokens, _ := strconv.ParseInt(tokensStr, 10, 64)
	if tokens != capacity-1 {
		t.Fatalf("首次请求后剩余令牌应为 %d，实际 %d", capacity-1, tokens)
	}
}

func TestRedisLimiter_ConsecutiveRejects(t *testing.T) {
	mr, client := setupMiniredis(t)
	defer client.Close()
	rl := newTestRedisLimiter(t, mr, client)

	ctx := context.Background()
	key := "test:consecutive-rejects"
	capacity := int64(2)

	for i := int64(0); i < capacity; i++ {
		_, _ = rl.Allow(ctx, key, WithCapacity(capacity), WithRate(1))
	}

	for i := 0; i < 10; i++ {
		allowed, err := rl.Allow(ctx, key, WithCapacity(capacity), WithRate(1))
		if err != nil {
			t.Fatalf("第 %d 次拒绝请求出错: %v", i+1, err)
		}
		if allowed {
			t.Fatalf("令牌耗尽后第 %d 次请求应被拒绝", i+1)
		}
	}
}

func TestRedisLimiter_RequestedTwoTokens(t *testing.T) {
	mr, client := setupMiniredis(t)
	defer client.Close()
	rl := newTestRedisLimiter(t, mr, client)

	ctx := context.Background()
	key := "test:requested-two"
	capacity := int64(5)

	allowed, err := rl.Allow(ctx, key, WithCapacity(capacity), WithRate(1), WithRequested(2))
	if err != nil {
		t.Fatalf("请求出错: %v", err)
	}
	if !allowed {
		t.Fatal("容量5请求2个令牌应被允许")
	}

	tokensStr, _ := client.HGet(ctx, key, "tokens").Result()
	tokens, _ := strconv.ParseInt(tokensStr, 10, 64)
	if tokens != 3 {
		t.Fatalf("消耗2个令牌后剩余应为3，实际 %d", tokens)
	}
}

func TestRedisLimiter_DrainAndFullReplenish(t *testing.T) {
	mr, client := setupMiniredis(t)
	defer client.Close()
	rl := newTestRedisLimiter(t, mr, client)

	ctx := context.Background()
	key := "test:drain-full-replenish"
	capacity := int64(5)
	rate := int64(1)

	for i := int64(0); i < capacity; i++ {
		_, _ = rl.Allow(ctx, key, WithCapacity(capacity), WithRate(rate))
	}

	allowed, _ := rl.Allow(ctx, key, WithCapacity(capacity), WithRate(rate))
	if allowed {
		t.Fatal("令牌耗尽后应被拒绝")
	}

	simulateTimeElapsed(t, client, key, time.Duration(capacity)*time.Second)

	for i := int64(0); i < capacity; i++ {
		allowed, err := rl.Allow(ctx, key, WithCapacity(capacity), WithRate(rate))
		if err != nil {
			t.Fatalf("补充后第 %d 次请求出错: %v", i+1, err)
		}
		if !allowed {
			t.Fatalf("补充后第 %d 次请求应被允许", i+1)
		}
	}
}

func TestRedisLimiter_PartialDrainPartialReplenish(t *testing.T) {
	mr, client := setupMiniredis(t)
	defer client.Close()
	rl := newTestRedisLimiter(t, mr, client)

	ctx := context.Background()
	key := "test:partial-drain-replenish"
	capacity := int64(10)
	rate := int64(2)

	for i := 0; i < 7; i++ {
		_, _ = rl.Allow(ctx, key, WithCapacity(capacity), WithRate(rate))
	}

	tokensStr, _ := client.HGet(ctx, key, "tokens").Result()
	tokensBefore, _ := strconv.ParseInt(tokensStr, 10, 64)
	fmt.Printf("PartialDrainPartialReplenish: after 7 requests, tokens=%d\n", tokensBefore)

	simulateTimeElapsed(t, client, key, 2*time.Second)

	successCount := 0
	for i := int64(0); i < capacity; i++ {
		ok, _ := rl.Allow(ctx, key, WithCapacity(capacity), WithRate(rate))
		if ok {
			successCount++
		}
	}
	fmt.Printf("PartialDrainPartialReplenish: after 2s replenish at rate=2, allowed=%d\n", successCount)
	if successCount == 0 {
		t.Fatal("部分补充后应至少有1次请求被允许")
	}
}
