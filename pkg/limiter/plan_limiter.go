package limiter

import (
	"context"
	_ "embed"
	"fmt"
	"strconv"
	"time"

	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/caiflower/common-tools/redis/v2"
	"github.com/redis/go-redis/v9"
)

//go:embed lua/plan_sliding_window.lua
var planSlidingWindowScript string

//go:embed lua/plan_refund.lua
var planRefundScript string

//go:embed lua/plan_usage.lua
var planUsageScript string

type PlanLimiter struct {
	client            redis.Cmdable
	scriptManager     *v2.ScriptManager
	defaultBudget     int64
	defaultWindow     time.Duration
	defaultBucketSize time.Duration
	defaultWeight     float64
}

func NewPlanLimiter(ctx context.Context, r redis.Cmdable, opts ...PlanOption) (*PlanLimiter, error) {
	pl := &PlanLimiter{
		client:            r,
		defaultBudget:     100000,
		defaultWindow:     8 * time.Hour,
		defaultBucketSize: 1 * time.Hour,
		defaultWeight:     1.0,
	}

	config := &PlanConfig{
		Budget:     pl.defaultBudget,
		Window:     pl.defaultWindow,
		BucketSize: pl.defaultBucketSize,
		Weight:     pl.defaultWeight,
	}
	for _, opt := range opts {
		opt(config)
	}
	pl.defaultBudget = config.Budget
	pl.defaultWindow = config.Window
	pl.defaultBucketSize = config.BucketSize
	pl.defaultWeight = config.Weight

	if err := validatePlanConfig(config); err != nil {
		return nil, fmt.Errorf("invalid plan limiter config: %w", err)
	}

	pl.scriptManager = v2.NewScriptManager(r)
	if err := pl.scriptManager.Register("allow", planSlidingWindowScript); err != nil {
		return nil, fmt.Errorf("failed to register allow script: %w", err)
	}
	if err := pl.scriptManager.Register("refund", planRefundScript); err != nil {
		return nil, fmt.Errorf("failed to register refund script: %w", err)
	}
	if err := pl.scriptManager.Register("usage", planUsageScript); err != nil {
		return nil, fmt.Errorf("failed to register usage script: %w", err)
	}

	if err := pl.scriptManager.LoadScripts(ctx); err != nil {
		return nil, fmt.Errorf("failed to load plan limiter scripts: %w", err)
	}

	return pl, nil
}

type PlanConfig struct {
	Budget     int64
	Window     time.Duration
	BucketSize time.Duration
	Weight     float64
}

type PlanOption func(*PlanConfig)

func WithPlanBudget(b int64) PlanOption {
	return func(cfg *PlanConfig) { cfg.Budget = b }
}

func WithPlanWindow(w time.Duration) PlanOption {
	return func(cfg *PlanConfig) { cfg.Window = w }
}

func WithPlanBucketSize(bs time.Duration) PlanOption {
	return func(cfg *PlanConfig) { cfg.BucketSize = bs }
}

func WithPlanWeight(w float64) PlanOption {
	return func(cfg *PlanConfig) { cfg.Weight = w }
}

type PlanUsage struct {
	TotalUsed int64
	Budget    int64
	Remaining int64
	WindowEnd time.Time
	ByModel   map[string]int64
}

type PlanLimiterInterface interface {
	Allow(ctx context.Context, key string, model string, tokens int64, opts ...PlanOption) (bool, *PlanUsage, error)
	Refund(ctx context.Context, key string, model string, tokens int64, opts ...PlanOption) (*PlanUsage, error)
	Usage(ctx context.Context, key string, opts ...PlanOption) (*PlanUsage, error)
}

var _ PlanLimiterInterface = (*PlanLimiter)(nil)

func (pl *PlanLimiter) Allow(ctx context.Context, key string, model string, tokens int64, opts ...PlanOption) (bool, *PlanUsage, error) {
	config := pl.applyOptions(opts)

	if err := validatePlanConfig(config); err != nil {
		return false, nil, fmt.Errorf("invalid config: %w", err)
	}

	result, err := pl.scriptManager.EvalSha(ctx, "allow",
		[]string{key},
		tokens, config.Budget, int64(config.Window.Seconds()), int64(config.BucketSize.Seconds()), model, config.Weight,
	)

	if err != nil {
		return false, nil, fmt.Errorf("plan allow failed: %w", err)
	}

	return pl.parseAllowResult(result, config)
}

func (pl *PlanLimiter) Refund(ctx context.Context, key string, model string, tokens int64, opts ...PlanOption) (*PlanUsage, error) {
	config := pl.applyOptions(opts)

	if err := validatePlanConfig(config); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	result, err := pl.scriptManager.EvalSha(ctx, "refund",
		[]string{key},
		tokens, config.Budget, int64(config.Window.Seconds()), int64(config.BucketSize.Seconds()), model, config.Weight,
	)

	if err != nil {
		return nil, fmt.Errorf("plan refund failed: %w", err)
	}

	return pl.parseRefundResult(result, config), nil
}

func (pl *PlanLimiter) Usage(ctx context.Context, key string, opts ...PlanOption) (*PlanUsage, error) {
	config := pl.applyOptions(opts)

	if err := validatePlanConfig(config); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	result, err := pl.scriptManager.EvalSha(ctx, "usage",
		[]string{key},
		config.Budget, int64(config.Window.Seconds()), int64(config.BucketSize.Seconds()),
	)

	if err != nil {
		return nil, fmt.Errorf("plan usage failed: %w", err)
	}

	return pl.parseUsageResult(result, config), nil
}

func validatePlanConfig(config *PlanConfig) error {
	if config.Weight <= 0 {
		return fmt.Errorf("weight must be positive, got %f", config.Weight)
	}
	if config.BucketSize <= 0 {
		return fmt.Errorf("bucketSize must be positive, got %v", config.BucketSize)
	}
	if config.Window <= 0 {
		return fmt.Errorf("window must be positive, got %v", config.Window)
	}
	if config.BucketSize > config.Window {
		return fmt.Errorf("bucketSize (%v) must not exceed window (%v)", config.BucketSize, config.Window)
	}
	if config.Budget <= 0 {
		return fmt.Errorf("budget must be positive, got %d", config.Budget)
	}
	return nil
}

func (pl *PlanLimiter) applyOptions(opts []PlanOption) *PlanConfig {
	config := &PlanConfig{
		Budget:     pl.defaultBudget,
		Window:     pl.defaultWindow,
		BucketSize: pl.defaultBucketSize,
		Weight:     pl.defaultWeight,
	}
	for _, opt := range opts {
		opt(config)
	}
	return config
}

func (pl *PlanLimiter) parseAllowResult(result []interface{}, config *PlanConfig) (bool, *PlanUsage, error) {
	if len(result) < 3 {
		return false, nil, fmt.Errorf("unexpected plan allow result length: %d", len(result))
	}

	allowed, ok := result[0].(int64)
	if !ok {
		return false, nil, fmt.Errorf("unexpected plan allow result[0] type: %T", result[0])
	}

	usage := pl.parseUsageFromResult(result[1], result[2], config)
	return allowed == 1, usage, nil
}

func (pl *PlanLimiter) parseRefundResult(result []interface{}, config *PlanConfig) *PlanUsage {
	if len(result) < 2 {
		return &PlanUsage{Budget: config.Budget}
	}
	return pl.parseUsageFromResult(result[0], result[1], config)
}

func (pl *PlanLimiter) parseUsageResult(result []interface{}, config *PlanConfig) *PlanUsage {
	if len(result) < 3 {
		return &PlanUsage{Budget: config.Budget}
	}

	usage := pl.parseUsageFromResult(result[0], result[1], config)

	if modelStats, ok := result[2].([]interface{}); ok {
		usage.ByModel = parseModelStats(modelStats)
	}

	return usage
}

func (pl *PlanLimiter) parseUsageFromResult(remainingIface, totalUsedIface interface{}, config *PlanConfig) *PlanUsage {
	remaining, _ := remainingIface.(int64)
	totalUsed, _ := totalUsedIface.(int64)

	return &PlanUsage{
		TotalUsed: totalUsed,
		Budget:    config.Budget,
		Remaining: remaining,
		WindowEnd: time.Now().Add(config.Window),
	}
}

func parseModelStats(stats []interface{}) map[string]int64 {
	result := make(map[string]int64)
	for i := 0; i+1 < len(stats); i += 2 {
		name, ok1 := stats[i].(string)
		val, ok2 := stats[i+1].(string)
		if ok1 && ok2 {
			v, err := strconv.ParseInt(val, 10, 64)
			if err != nil {
				logger.Warn("Failed to parse model stat value for %s: %v", name, err)
				v = 0
			}
			result[name] = v
		}
	}
	return result
}
