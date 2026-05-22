package redisv1

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

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

const testScript = `
local key = KEYS[1]
local value = ARGV[1]
redis.call('SET', key, value)
return 1
`

const testScriptMultiReturn = `
local key = KEYS[1]
local value = ARGV[1]
redis.call('SET', key, value)
return {1, value}
`

func TestScriptManager_RegisterAndLoad(t *testing.T) {
	_, client := setupMiniredis(t)
	defer client.Close()

	sm := NewScriptManager(client)
	sm.Register("test_op", testScript)

	if err := sm.LoadScripts(context.Background()); err != nil {
		t.Fatalf("LoadScripts failed: %v", err)
	}
}

func TestScriptManager_RegisterDuplicate(t *testing.T) {
	_, client := setupMiniredis(t)
	defer client.Close()

	sm := NewScriptManager(client)
	sm.Register("test_op", testScript)

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic on duplicate registration, but no panic occurred")
		}
		msg, ok := r.(string)
		if !ok {
			t.Fatalf("expected string panic, got %T: %v", r, r)
		}
		if msg != `script already registered for op "test_op"` {
			t.Fatalf("unexpected panic message: %s", msg)
		}
	}()
	sm.Register("test_op", "another script")
}

func TestScriptManager_UnregisteredOp(t *testing.T) {
	_, client := setupMiniredis(t)
	defer client.Close()

	sm := NewScriptManager(client)
	sm.Register("test_op", testScript)
	if err := sm.LoadScripts(context.Background()); err != nil {
		t.Fatalf("LoadScripts failed: %v", err)
	}

	_, err := sm.EvalSha(context.Background(), "nonexistent", []string{"key"}, "val")
	if err == nil {
		t.Fatal("expected error for unregistered op, got nil")
	}
}

func TestScriptManager_EvalShaInt(t *testing.T) {
	_, client := setupMiniredis(t)
	defer client.Close()

	sm := NewScriptManager(client)
	sm.Register("test_op", testScript)
	if err := sm.LoadScripts(context.Background()); err != nil {
		t.Fatalf("LoadScripts failed: %v", err)
	}

	result, err := sm.EvalShaInt(context.Background(), "test_op", []string{"test:key"}, "hello")
	if err != nil {
		t.Fatalf("EvalShaInt failed: %v", err)
	}
	if result != 1 {
		t.Fatalf("expected 1, got %d", result)
	}

	got, err := client.Get(context.Background(), "test:key").Result()
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got != "hello" {
		t.Fatalf("expected 'hello', got '%s'", got)
	}
}

func TestScriptManager_EvalSha(t *testing.T) {
	_, client := setupMiniredis(t)
	defer client.Close()

	sm := NewScriptManager(client)
	sm.Register("test_op", testScriptMultiReturn)
	if err := sm.LoadScripts(context.Background()); err != nil {
		t.Fatalf("LoadScripts failed: %v", err)
	}

	result, err := sm.EvalSha(context.Background(), "test_op", []string{"test:key2"}, "world")
	if err != nil {
		t.Fatalf("EvalSha failed: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 results, got %d", len(result))
	}
	allowed, ok := result[0].(int64)
	if !ok || allowed != 1 {
		t.Fatalf("expected result[0]=1, got %v", result[0])
	}
	val, ok := result[1].(string)
	if !ok || val != "world" {
		t.Fatalf("expected result[1]='world', got %v", result[1])
	}
}

func TestScriptManager_NOSCRIPTFallback(t *testing.T) {
	_, client := setupMiniredis(t)
	defer client.Close()

	sm := NewScriptManager(client)
	sm.Register("test_op", testScript)
	if err := sm.LoadScripts(context.Background()); err != nil {
		t.Fatalf("LoadScripts failed: %v", err)
	}

	client.ScriptFlush(context.Background()).Err()

	result, err := sm.EvalShaInt(context.Background(), "test_op", []string{"test:noscript"}, "after_flush")
	if err != nil {
		t.Fatalf("EvalShaInt after SCRIPT FLUSH failed: %v", err)
	}
	if result != 1 {
		t.Fatalf("expected 1, got %d", result)
	}

	got, err := client.Get(context.Background(), "test:noscript").Result()
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got != "after_flush" {
		t.Fatalf("expected 'after_flush', got '%s'", got)
	}
}

func TestScriptManager_ConcurrentEvalSha(t *testing.T) {
	_, client := setupMiniredis(t)
	defer client.Close()

	sm := NewScriptManager(client)
	sm.Register("test_op", testScript)
	if err := sm.LoadScripts(context.Background()); err != nil {
		t.Fatalf("LoadScripts failed: %v", err)
	}

	var wg sync.WaitGroup
	var errors atomic.Int64

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := "test:concurrent"
			_, err := sm.EvalShaInt(context.Background(), "test_op", []string{key}, i)
			if err != nil {
				errors.Add(1)
			}
		}(i)
	}
	wg.Wait()

	if errors.Load() > 0 {
		t.Fatalf("expected no errors in concurrent EvalSha, got %d", errors.Load())
	}
}

func TestScriptManager_ConcurrentNOSCRIPTRecovery(t *testing.T) {
	_, client := setupMiniredis(t)
	defer client.Close()

	sm := NewScriptManager(client)
	sm.Register("test_op", testScript)
	if err := sm.LoadScripts(context.Background()); err != nil {
		t.Fatalf("LoadScripts failed: %v", err)
	}

	client.ScriptFlush(context.Background()).Err()

	var wg sync.WaitGroup
	var errors atomic.Int64

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := "test:concurrent_noscript"
			_, err := sm.EvalShaInt(context.Background(), "test_op", []string{key}, i)
			if err != nil {
				errors.Add(1)
			}
		}(i)
	}
	wg.Wait()

	if errors.Load() > 0 {
		t.Fatalf("expected no errors in concurrent NOSCRIPT recovery, got %d", errors.Load())
	}
}

func TestScriptManager_LoadScriptsFailure(t *testing.T) {
	_, client := setupMiniredis(t)
	defer client.Close()

	sm := NewScriptManager(client)
	sm.Register("test_op", "invalid lua script !!!")

	err := sm.LoadScripts(context.Background())
	if err == nil {
		t.Fatal("expected error for invalid script, got nil")
	}
}
