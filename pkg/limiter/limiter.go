package limiter

import (
	"context"
	_ "embed"
	"fmt"

	redisv1 "github.com/caiflower/common-tools/redis/v1"
	"github.com/go-redis/redis/v8"
)

//go:embed lua/rate_limit.lua
var rateLimitScript string

type RedisLimiter struct {
	client           redis.Cmdable
	scriptManager    *redisv1.ScriptManager
	defaultCapacity  int64
	defaultRate      int64
	defaultRequested int64
}

// RedisLimiterInterface defines the interface for token bucket rate limiting.
type RedisLimiterInterface interface {
	Allow(ctx context.Context, key string, opts ...Option) (bool, error)
}

var _ RedisLimiterInterface = (*RedisLimiter)(nil)

// NewRedisLimiter creates a new RedisLimiter instance.
// LimiterOption configures the default values used when Allow is called without explicit options.
// This is distinct from Option, which overrides parameters on a per-request basis.
func NewRedisLimiter(ctx context.Context, r redis.Cmdable, opts ...LimiterOption) (*RedisLimiter, error) {
	rl := &RedisLimiter{
		client:           r,
		defaultCapacity:  10,
		defaultRate:      1,
		defaultRequested: 1,
	}

	config := &LimiterConfig{
		Capacity:  rl.defaultCapacity,
		Rate:      rl.defaultRate,
		Requested: rl.defaultRequested,
	}
	for _, opt := range opts {
		opt(config)
	}
	rl.defaultCapacity = config.Capacity
	rl.defaultRate = config.Rate
	rl.defaultRequested = config.Requested

	if err := validateLimiterConfig(config); err != nil {
		return nil, fmt.Errorf("invalid limiter config: %w", err)
	}

	rl.scriptManager = redisv1.NewScriptManager(r)
	rl.scriptManager.Register("rate_limit", rateLimitScript)

	if err := rl.scriptManager.LoadScripts(ctx); err != nil {
		return nil, fmt.Errorf("failed to load rate limit script: %w", err)
	}

	return rl, nil
}

// Allow checks whether a request is allowed under the token bucket rate limit.
// Option overrides the default capacity/rate/requested for this specific call.
// This is distinct from LimiterOption, which sets the default values at construction time.
func (rl *RedisLimiter) Allow(ctx context.Context, key string, opts ...Option) (bool, error) {
	config := &Config{
		LimiterConfig: LimiterConfig{
			Capacity:  rl.defaultCapacity,
			Rate:      rl.defaultRate,
			Requested: rl.defaultRequested,
		},
	}

	for _, opt := range opts {
		opt(config)
	}

	if err := validateLimiterConfig(&config.LimiterConfig); err != nil {
		return false, fmt.Errorf("invalid rate limit config: %w", err)
	}

	result, err := rl.scriptManager.EvalShaInt(ctx, "rate_limit",
		[]string{key},
		config.Requested,
		config.Rate,
		config.Capacity,
	)

	if err != nil {
		return false, fmt.Errorf("rate limit failed: %w", err)
	}

	return result == 1, nil
}

// LimiterConfig holds the default configuration for RedisLimiter construction.
// These values are used when Allow is called without explicit Option overrides.
type LimiterConfig struct {
	Capacity  int64
	Rate      int64
	Requested int64
}

// LimiterOption configures default values at construction time.
type LimiterOption func(*LimiterConfig)

func WithDefaultCapacity(c int64) LimiterOption {
	return func(cfg *LimiterConfig) { cfg.Capacity = c }
}

func WithDefaultRate(r int64) LimiterOption {
	return func(cfg *LimiterConfig) { cfg.Rate = r }
}

func WithDefaultRequested(n int64) LimiterOption {
	return func(cfg *LimiterConfig) { cfg.Requested = n }
}

func validateLimiterConfig(config *LimiterConfig) error {
	if config.Capacity <= 0 {
		return fmt.Errorf("capacity must be positive, got %d", config.Capacity)
	}
	if config.Rate <= 0 {
		return fmt.Errorf("rate must be positive, got %d", config.Rate)
	}
	if config.Requested <= 0 {
		return fmt.Errorf("requested must be positive, got %d", config.Requested)
	}
	return nil
}

// Config holds per-request configuration for Allow calls.
// These values override the defaults set by LimiterConfig at construction time.
type Config struct {
	LimiterConfig
}

// Option overrides parameters on a per-request basis.
type Option func(*Config)

func WithCapacity(c int64) Option {
	return func(cfg *Config) { cfg.Capacity = c }
}

func WithRate(r int64) Option {
	return func(cfg *Config) { cfg.Rate = r }
}

func WithRequested(n int64) Option {
	return func(cfg *Config) { cfg.Requested = n }
}
