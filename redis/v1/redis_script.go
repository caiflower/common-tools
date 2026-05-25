package redisv1

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/go-redis/redis/v8"
)

type ScriptEntry struct {
	Script string
	SHA    string
}

type ScriptManager struct {
	client  redis.Cmdable
	mu      sync.RWMutex
	scripts map[string]*ScriptEntry
}

func NewScriptManager(client redis.Cmdable) *ScriptManager {
	return &ScriptManager{
		client:  client,
		scripts: make(map[string]*ScriptEntry),
	}
}

// Register adds a script entry for the given operation.
// It must be called before LoadScripts. Duplicate op registration will panic.
func (sm *ScriptManager) Register(op string, script string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if _, exists := sm.scripts[op]; exists {
		panic(fmt.Sprintf("script already registered for op %q", op))
	}
	sm.scripts[op] = &ScriptEntry{Script: script}
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

func (sm *ScriptManager) evalShaCmd(ctx context.Context, op string, keys []string, args ...interface{}) *redis.Cmd {
	sha, script, err := sm.getSHA(op)
	if err != nil {
		cmd := redis.NewCmd(ctx)
		cmd.SetErr(err)
		return cmd
	}

	cmd := sm.client.EvalSha(ctx, sha, keys, args...)
	if err := cmd.Err(); err != nil && isNOSCRIPTERR(err) {
		if reloadErr := sm.LoadScripts(ctx); reloadErr != nil {
			cmd.SetErr(fmt.Errorf("EVALSHA failed and script reload also failed: %w (reload: %v)", err, reloadErr))
			return cmd
		}

		newSHA, _, shaErr := sm.getSHA(op)
		if shaErr != nil {
			cmd = sm.client.Eval(ctx, script, keys, args...)
			if err := cmd.Err(); err != nil {
				cmd.SetErr(fmt.Errorf("EVAL fallback failed: %w", err))
			}
			return cmd
		}

		cmd = sm.client.EvalSha(ctx, newSHA, keys, args...)
		if err := cmd.Err(); err != nil && isNOSCRIPTERR(err) {
			cmd = sm.client.Eval(ctx, script, keys, args...)
			if err := cmd.Err(); err != nil {
				cmd.SetErr(fmt.Errorf("EVAL fallback failed: %w", err))
			}
		}
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
