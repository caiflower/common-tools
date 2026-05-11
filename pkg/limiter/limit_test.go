package limiter

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
)

// setupRedisClient 连接本地 Redis，若连接失败则跳过测试
func setupRedisClient(t *testing.T) (*redis.Client, func()) {
	t.Helper()
	client := redis.NewClient(&redis.Options{
		Addr: "127.0.0.1:6379",
	})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		t.Skipf("跳过测试：无法连接 Redis（%v），请启动本地 Redis", err)
	}
	// 每次测试前重置单例
	once = sync.Once{}
	instance = nil
	return client, func() { _ = client.Close() }
}

// TestRedisLimiter_BasicAllow 测试基本的允许/拒绝逻辑
func TestRedisLimiter_BasicAllow(t *testing.T) {
	client, cleanup := setupRedisClient(t)
	defer cleanup()

	ctx := context.Background()
	rl := NewRedisLimiter(ctx, client)

	key := fmt.Sprintf("test:basic:%d", time.Now().UnixNano())
	capacity := int64(3)

	success := 0
	failed := 0
	// 发送 capacity+2 次请求，成功数应不超过 capacity
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

// TestRedisLimiter_TokenReplenish 测试令牌补充逻辑：
// 先快速耗尽令牌桶，确认限流生效，再等待补充后验证恢复
func TestRedisLimiter_TokenReplenish(t *testing.T) {
	client, cleanup := setupRedisClient(t)
	defer cleanup()

	ctx := context.Background()
	rl := NewRedisLimiter(ctx, client)

	key := fmt.Sprintf("test:replenish:%d", time.Now().UnixNano())
	// 容量 3，速率 1/s：请求非常快（毫秒级），补充量可忽略
	capacity := int64(3)
	rate := int64(1)

	// 快速耗尽令牌（capacity+5 次，正常应有若干次被拒绝）
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

	// 等待令牌补充（速率 1/s，等待 3s 确保至少补充 1 个令牌）
	time.Sleep(3 * time.Second)

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

// TestRedisLimiter_DefaultConfig 测试默认配置（容量 10，速率 1）
// 验证：快速连续请求时，限流确实生效（存在被拒绝的请求）
func TestRedisLimiter_DefaultConfig(t *testing.T) {
	client, cleanup := setupRedisClient(t)
	defer cleanup()

	ctx := context.Background()
	rl := NewRedisLimiter(ctx, client)

	key := fmt.Sprintf("test:default:%d", time.Now().UnixNano())
	defaultCapacity := int64(10)

	success := 0
	failed := 0
	// 发送 2*capacity 次请求，验证限流确实生效
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

// TestRedisLimiter_WithRequested 测试一次请求多个令牌
func TestRedisLimiter_WithRequested(t *testing.T) {
	client, cleanup := setupRedisClient(t)
	defer cleanup()

	ctx := context.Background()
	rl := NewRedisLimiter(ctx, client)

	key := fmt.Sprintf("test:requested:%d", time.Now().UnixNano())
	capacity := int64(5)
	requested := int64(3)

	success := 0
	failed := 0
	// 每次消耗 3 个令牌，容量 5，最多允许 1 次
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
	// 容量 5，每次消耗 3，最多允许 floor(5/3)=1 次
	maxAllowed := capacity / requested
	if int64(success) > maxAllowed {
		t.Fatalf("成功次数 %d 超出预期上限 %d（容量=%d，每次消耗=%d）", success, maxAllowed, capacity, requested)
	}
	if success == 0 {
		t.Fatal("至少应有 1 次请求被允许")
	}
}

// TestRedisLimiter_Concurrent 并发测试：多个 goroutine 同时请求同一个 key
func TestRedisLimiter_Concurrent(t *testing.T) {
	client, cleanup := setupRedisClient(t)
	defer cleanup()

	ctx := context.Background()
	rl := NewRedisLimiter(ctx, client)

	key := fmt.Sprintf("test:concurrent:%d", time.Now().UnixNano())
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

// TestRedisLimiter_MultipleKeys 测试多个不同 key 互不影响
func TestRedisLimiter_MultipleKeys(t *testing.T) {
	client, cleanup := setupRedisClient(t)
	defer cleanup()

	ctx := context.Background()
	rl := NewRedisLimiter(ctx, client)

	base := time.Now().UnixNano()
	keys := []string{
		fmt.Sprintf("test:multi:a:%d", base),
		fmt.Sprintf("test:multi:b:%d", base),
		fmt.Sprintf("test:multi:c:%d", base),
	}
	capacity := int64(1)

	// 每个 key 独立计数，容量为 1，每个 key 发送 3 次，成功数应不超过 1
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

func TestRedisLimiter(t *testing.T) {
	client, cleanup := setupRedisClient(t)
	defer cleanup()

	ctx := context.Background()
	key := fmt.Sprintf("test:redis-limiter:%d", time.Now().UnixNano())

	rl := NewRedisLimiter(ctx, client)
	group := sync.WaitGroup{}

	var success, failed atomic.Int64

	for i := 0; i < 10000; i++ {
		group.Add(1)
		go func(v int) {
			defer group.Done()
			allowed, err := rl.Allow(ctx, key, WithCapacity(1000), WithRate(1000))
			if err != nil {
				failed.Add(1)
				return
			}
			if allowed {
				success.Add(1)
			} else {
				failed.Add(1)
			}
		}(i)
	}

	group.Wait()
	fmt.Printf("success: %d, failed: %d\n", success.Load(), failed.Load())
}
