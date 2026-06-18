package limiter

import (
	"context"
	_ "embed"
	"fmt"
	"time"

	redisv2 "github.com/caiflower/common-tools/redis/v2"
	"github.com/redis/go-redis/v9"
)

//go:embed lua/key_fixed_window_acquire.lua
var keyFixedWindowAcquireScript string

//go:embed lua/key_fixed_window_release.lua
var keyFixedWindowReleaseScript string

const keyFixedWindowPrefix = "key_fixed_window:"

type KeyFixedWindowConfig struct {
	MaxConcurrent int64
	Expiration    time.Duration
}

type KeyFixedWindowOption func(*KeyFixedWindowConfig)

func WithDefaultMaxConcurrent(m int64) KeyFixedWindowOption {
	return func(cfg *KeyFixedWindowConfig) { cfg.MaxConcurrent = m }
}

func WithDefaultExpiration(d time.Duration) KeyFixedWindowOption {
	return func(cfg *KeyFixedWindowConfig) { cfg.Expiration = d }
}

type KeyFixedWindowCallConfig struct {
	MaxConcurrent int64
}

type KeyFixedWindowCallOption func(*KeyFixedWindowCallConfig)

func WithMaxConcurrent(m int64) KeyFixedWindowCallOption {
	return func(cfg *KeyFixedWindowCallConfig) { cfg.MaxConcurrent = m }
}

type KeyFixedWindowLimiterInterface interface {
	Acquire(ctx context.Context, key string, opts ...KeyFixedWindowCallOption) (bool, int64, error)
	Release(ctx context.Context, key string) (int64, error)
	CurrentConcurrent(ctx context.Context, key string) (int64, error)
}

var _ KeyFixedWindowLimiterInterface = (*KeyFixedWindowLimiter)(nil)

type KeyFixedWindowLimiter struct {
	client               redis.Cmdable
	scriptManager        *redisv2.ScriptManager
	defaultMaxConcurrent int64
	defaultExpiration    time.Duration
}

func NewKeyFixedWindowLimiter(ctx context.Context, r redis.Cmdable, opts ...KeyFixedWindowOption) (*KeyFixedWindowLimiter, error) {
	kl := &KeyFixedWindowLimiter{
		client:               r,
		defaultMaxConcurrent: 10,
		defaultExpiration:    24 * time.Hour,
	}

	config := &KeyFixedWindowConfig{
		MaxConcurrent: kl.defaultMaxConcurrent,
		Expiration:    kl.defaultExpiration,
	}
	for _, opt := range opts {
		opt(config)
	}
	kl.defaultMaxConcurrent = config.MaxConcurrent
	kl.defaultExpiration = config.Expiration

	if err := validateKeyFixedWindowConfig(config); err != nil {
		return nil, fmt.Errorf("invalid key fixed window config: %w", err)
	}

	kl.scriptManager = redisv2.NewScriptManager(r)
	if err := kl.scriptManager.Register("key_fixed_window_acquire", keyFixedWindowAcquireScript); err != nil {
		return nil, fmt.Errorf("failed to register key_fixed_window_acquire script: %w", err)
	}
	if err := kl.scriptManager.Register("key_fixed_window_release", keyFixedWindowReleaseScript); err != nil {
		return nil, fmt.Errorf("failed to register key_fixed_window_release script: %w", err)
	}

	if err := kl.scriptManager.LoadScripts(ctx); err != nil {
		return nil, fmt.Errorf("failed to load key fixed window scripts: %w", err)
	}

	return kl, nil
}

func (kl *KeyFixedWindowLimiter) buildKey(key string) string {
	return keyFixedWindowPrefix + key
}

func (kl *KeyFixedWindowLimiter) Acquire(ctx context.Context, key string, opts ...KeyFixedWindowCallOption) (bool, int64, error) {
	maxConcurrent := kl.defaultMaxConcurrent
	callCfg := &KeyFixedWindowCallConfig{MaxConcurrent: maxConcurrent}
	for _, opt := range opts {
		opt(callCfg)
	}
	maxConcurrent = callCfg.MaxConcurrent

	result, err := kl.scriptManager.EvalSha(ctx, "key_fixed_window_acquire",
		[]string{kl.buildKey(key)},
		maxConcurrent, int64(kl.defaultExpiration.Seconds()),
	)
	if err != nil {
		return false, 0, fmt.Errorf("key fixed window acquire failed: %w", err)
	}

	if len(result) < 2 {
		return false, 0, fmt.Errorf("unexpected key fixed window acquire result length: %d", len(result))
	}

	allowed, ok1 := result[0].(int64)
	current, ok2 := result[1].(int64)
	if !ok1 || !ok2 {
		return false, 0, fmt.Errorf("unexpected key fixed window acquire result values: %v", result)
	}

	return allowed == 1, current, nil
}

func (kl *KeyFixedWindowLimiter) Release(ctx context.Context, key string) (int64, error) {
	result, err := kl.scriptManager.EvalShaInt(ctx, "key_fixed_window_release",
		[]string{kl.buildKey(key)},
		int64(kl.defaultExpiration.Seconds()),
	)
	if err != nil {
		return 0, fmt.Errorf("key fixed window release failed: %w", err)
	}

	return result, nil
}

// CurrentConcurrent returns the approximate current concurrent count for the given key.
// The value may be stale immediately after reading due to concurrent Acquire/Release
// operations from other clients. Use this for monitoring and observability only.
func (kl *KeyFixedWindowLimiter) CurrentConcurrent(ctx context.Context, key string) (int64, error) {
	val, err := kl.client.Get(ctx, kl.buildKey(key)).Int64()
	if err == redis.Nil {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("key fixed window current concurrent failed: %w", err)
	}
	return val, nil
}

func validateKeyFixedWindowConfig(config *KeyFixedWindowConfig) error {
	if config.MaxConcurrent <= 0 {
		return fmt.Errorf("maxConcurrent must be positive, got %d", config.MaxConcurrent)
	}
	if config.Expiration <= 0 {
		return fmt.Errorf("expiration must be positive, got %v", config.Expiration)
	}
	return nil
}
