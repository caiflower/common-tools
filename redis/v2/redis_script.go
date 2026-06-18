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
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/redis/go-redis/v9"
)

type ScriptEntry struct {
	Script string
	SHA    string
}

type ScriptManager struct {
	client    redis.Cmdable
	mu        sync.RWMutex
	scripts   map[string]*ScriptEntry
	reloading atomic.Bool
}

func NewScriptManager(client redis.Cmdable) *ScriptManager {
	return &ScriptManager{
		client:  client,
		scripts: make(map[string]*ScriptEntry),
	}
}

func (sm *ScriptManager) Register(op string, script string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if _, exists := sm.scripts[op]; exists {
		return fmt.Errorf("script already registered for op %q", op)
	}
	sm.scripts[op] = &ScriptEntry{Script: script}
	return nil
}

func (sm *ScriptManager) LoadScripts(ctx context.Context) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	for op, entry := range sm.scripts {
		sha, err := sm.client.ScriptLoad(ctx, entry.Script).Result()
		if err != nil {
			return fmt.Errorf("load script %s: %w", op, err)
		}
		entry.SHA = sha
	}
	return nil
}

func (sm *ScriptManager) getSHA(op string) (string, string, error) {
	sm.mu.RLock()
	entry, ok := sm.scripts[op]
	sm.mu.RUnlock()
	if !ok {
		return "", "", fmt.Errorf("script %s not registered", op)
	}
	return entry.SHA, entry.Script, nil
}

// triggerReload asynchronously reloads all script SHAs into Redis.
// The current request always falls back to EVAL immediately; this reload
// is solely for preparing subsequent requests so they can use EVALSHA.
// A detached context (context.Background()) is used so the reload is not
// cancelled when the triggering request's context expires.
func (sm *ScriptManager) triggerReload() {
	if sm.reloading.CompareAndSwap(false, true) {
		go func() {
			defer sm.reloading.Store(false)
			if err := sm.LoadScripts(context.Background()); err != nil {
				logger.Warn("ScriptManager: async reload failed: %v", err)
			}
		}()
	}
}

func (sm *ScriptManager) evalShaCmd(ctx context.Context, op string, keys []string, args ...interface{}) *redis.Cmd {
	sha, script, err := sm.getSHA(op)
	if err != nil {
		cmd := redis.NewCmd(ctx)
		cmd.SetErr(err)
		return cmd
	}

	if sha != "" {
		cmd := sm.client.EvalSha(ctx, sha, keys, args...)
		if err := cmd.Err(); err == nil || !isNOSCRIPTERR(err) {
			return cmd
		}
	}

	// NOSCRIPT error or SHA not loaded yet:
	// 1. Trigger async reload so subsequent requests can use EVALSHA.
	// 2. Fall back to EVAL for the current request (reload is async, SHA not ready yet).
	sm.triggerReload()

	cmd := sm.client.Eval(ctx, script, keys, args...)
	if err := cmd.Err(); err != nil {
		cmd.SetErr(fmt.Errorf("EVAL fallback failed: %w", err))
	}
	return cmd
}

func (sm *ScriptManager) EvalSha(ctx context.Context, op string, keys []string, args ...interface{}) ([]interface{}, error) {
	cmd := sm.evalShaCmd(ctx, op, keys, args...)
	if err := cmd.Err(); err != nil {
		return nil, err
	}
	return cmd.Slice()
}

func (sm *ScriptManager) EvalShaInt(ctx context.Context, op string, keys []string, args ...interface{}) (int64, error) {
	cmd := sm.evalShaCmd(ctx, op, keys, args...)
	if err := cmd.Err(); err != nil {
		return 0, err
	}
	return cmd.Int64()
}

func isNOSCRIPTERR(err error) bool {
	return err != nil && strings.Contains(err.Error(), "NOSCRIPT")
}
