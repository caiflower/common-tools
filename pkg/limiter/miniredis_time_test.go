package limiter

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestMiniredisTimeDiagnostic(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()

	ctx := context.Background()

	initialTime, err := client.Time(ctx).Result()
	if err != nil {
		t.Fatalf("Time failed: %v", err)
	}
	fmt.Printf("Initial TIME: %v\n", initialTime)

	mr.FastForward(5 * time.Second)

	afterTime, err := client.Time(ctx).Result()
	if err != nil {
		t.Fatalf("Time after FastForward failed: %v", err)
	}
	fmt.Printf("After FastForward(5s) TIME: %v\n", afterTime)
	fmt.Printf("Diff: %v\n", afterTime.Sub(initialTime))

	script := `
	local now = redis.call('TIME')
	return {now[1], now[2]}
	`
	result, err := client.Eval(ctx, script, []string{}).Int64Slice()
	if err != nil {
		t.Fatalf("Eval TIME in Lua failed: %v", err)
	}
	fmt.Printf("TIME inside Lua after FastForward: %v (epoch seconds)\n", result[0])

	mr.FastForward(10 * time.Second)

	result2, err := client.Eval(ctx, script, []string{}).Int64Slice()
	if err != nil {
		t.Fatalf("Eval TIME in Lua after 2nd FastForward failed: %v", err)
	}
	fmt.Printf("TIME inside Lua after 2nd FastForward(10s): %v (epoch seconds)\n", result2[0])
	fmt.Printf("Lua TIME diff: %d seconds\n", result2[0]-result[0])
}
