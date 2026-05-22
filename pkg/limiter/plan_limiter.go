package limiter

import (
	"context"
	_ "embed"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/go-redis/redis/v8"
)

//go:embed lua/plan_sliding_window.lua
var planSlidingWindowScript string

//go:embed lua/plan_refund.lua
var planRefundScript string

//go:embed lua/plan_usage.lua
var planUsageScript string

type PlanLimiter struct {
	client            *redis.Client
	allowScriptSHA    string
	refundScriptSHA   string
	usageScriptSHA    string
	defaultBudget     int64
	defaultWindow     time.Duration
	defaultBucketSize time.Duration
	defaultWeight     float64
	mu                sync.RWMutex
}

func NewPlanLimiter(ctx context.Context, r *redis.Client, opts ...PlanOption) (*PlanLimiter, error) {
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

	if err := pl.loadScripts(ctx); err != nil {
		return nil, fmt.Errorf("failed to load plan limiter scripts: %w", err)
	}

	return pl, nil
}

func (pl *PlanLimiter) loadScripts(ctx context.Context) error {
	allowSHA, err := pl.client.ScriptLoad(ctx, planSlidingWindowScript).Result()
	if err != nil {
		return fmt.Errorf("load allow script: %w", err)
	}
	refundSHA, err := pl.client.ScriptLoad(ctx, planRefundScript).Result()
	if err != nil {
		return fmt.Errorf("load refund script: %w", err)
	}
	usageSHA, err := pl.client.ScriptLoad(ctx, planUsageScript).Result()
	if err != nil {
		return fmt.Errorf("load usage script: %w", err)
	}

	pl.mu.Lock()
	pl.allowScriptSHA = allowSHA
	pl.refundScriptSHA = refundSHA
	pl.usageScriptSHA = usageSHA
	pl.mu.Unlock()

	return nil
}

// getScriptSHA returns the SHA for the given operation.
// MUST be called while holding pl.mu read lock (pl.mu.RLock()).
func (pl *PlanLimiter) getScriptSHA(op string) string {
	switch op {
	case "allow":
		return pl.allowScriptSHA
	case "refund":
		return pl.refundScriptSHA
	case "usage":
		return pl.usageScriptSHA
	default:
		return ""
	}
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

	result, err := pl.evalSha(ctx, "allow", planSlidingWindowScript,
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

	result, err := pl.evalSha(ctx, "refund", planRefundScript,
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

	result, err := pl.evalSha(ctx, "usage", planUsageScript,
		[]string{key},
		config.Budget, int64(config.Window.Seconds()), int64(config.BucketSize.Seconds()),
	)

	if err != nil {
		return nil, fmt.Errorf("plan usage failed: %w", err)
	}

	return pl.parseUsageResult(result, config), nil
}

func (pl *PlanLimiter) evalSha(ctx context.Context, op string, script string, keys []string, args ...interface{}) ([]interface{}, error) {
	pl.mu.RLock()
	sha := pl.getScriptSHA(op)
	pl.mu.RUnlock()

	result, err := pl.client.EvalSha(ctx, sha, keys, args...).Slice()
	if err != nil && isNOSHAERR(err) {
		if reloadErr := pl.loadScripts(ctx); reloadErr != nil {
			return nil, fmt.Errorf("EVALSHA failed and script reload also failed: %w (reload: %v)", err, reloadErr)
		}

		pl.mu.RLock()
		newSHA := pl.getScriptSHA(op)
		pl.mu.RUnlock()

		result, err = pl.client.EvalSha(ctx, newSHA, keys, args...).Slice()
		if err != nil && isNOSHAERR(err) {
			result, err = pl.client.Eval(ctx, script, keys, args...).Slice()
			if err != nil {
				return nil, fmt.Errorf("EVAL fallback failed: %w", err)
			}
		}
	}

	if err != nil {
		return nil, err
	}

	return result, nil
}

func isNOSHAERR(err error) bool {
	return err != nil && strings.Contains(err.Error(), "NOSCRIPT")
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
