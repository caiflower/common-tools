/*
 * Copyright 2024 caiflower Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package v2

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
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

func mustRegister(t *testing.T, sm *ScriptManager, op, script string) {
	t.Helper()
	if err := sm.Register(op, script); err != nil {
		t.Fatalf("Register(%q) failed: %v", op, err)
	}
}

func TestScriptManager_RegisterAndLoad(t *testing.T) {
	_, client := setupMiniredis(t)
	defer client.Close()

	sm := NewScriptManager(client)
	mustRegister(t, sm, "test_op", testScript)

	if err := sm.LoadScripts(context.Background()); err != nil {
		t.Fatalf("LoadScripts failed: %v", err)
	}
}

func TestScriptManager_RegisterDuplicate(t *testing.T) {
	_, client := setupMiniredis(t)
	defer client.Close()

	sm := NewScriptManager(client)
	mustRegister(t, sm, "test_op", testScript)

	err := sm.Register("test_op", "another script")
	if err == nil {
		t.Fatal("expected error on duplicate registration, got nil")
	}
	if err.Error() != `script already registered for op "test_op"` {
		t.Fatalf("unexpected error message: %s", err.Error())
	}
}

func TestScriptManager_UnregisteredOp(t *testing.T) {
	_, client := setupMiniredis(t)
	defer client.Close()

	sm := NewScriptManager(client)
	mustRegister(t, sm, "test_op", testScript)
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
	mustRegister(t, sm, "test_op", testScript)
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
	mustRegister(t, sm, "test_op", testScriptMultiReturn)
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
	mustRegister(t, sm, "test_op", testScript)
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
	mustRegister(t, sm, "test_op", testScript)
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
	mustRegister(t, sm, "test_op", testScript)
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
	mustRegister(t, sm, "test_op", "invalid lua script !!!")

	err := sm.LoadScripts(context.Background())
	if err == nil {
		t.Fatal("expected error for invalid script, got nil")
	}
}
