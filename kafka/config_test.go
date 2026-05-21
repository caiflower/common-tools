package xkafka

import (
	"testing"
	"time"

	"github.com/caiflower/common-tools/pkg/tools"
	"github.com/stretchr/testify/assert"
)

func TestConfig_DurationBackwardCompatibility(t *testing.T) {
	cfg := Config{
		Name:                   "test",
		Enable:                 "true",
		ProducerRequestTimeout: 10000,
		ProducerMessageTimeout: 15000,
	}
	_ = tools.DoTagFunc(&cfg, []tools.FnObj{{Fn: tools.SetDefaultValueIfNil}})

	assert.Equal(t, time.Duration(10000), cfg.ProducerRequestTimeout, "integer 10000 parsed as 10000ns (10us)")
	assert.Equal(t, time.Duration(15000), cfg.ProducerMessageTimeout, "integer 15000 parsed as 15000ns (15us)")

	if cfg.ProducerRequestTimeout < time.Millisecond {
		cfg.ProducerRequestTimeout = cfg.ProducerRequestTimeout * time.Millisecond
	}
	if cfg.ProducerMessageTimeout < time.Millisecond {
		cfg.ProducerMessageTimeout = cfg.ProducerMessageTimeout * time.Millisecond
	}

	assert.Equal(t, 10*time.Second, cfg.ProducerRequestTimeout, "10000ns * ms = 10s")
	assert.Equal(t, 15*time.Second, cfg.ProducerMessageTimeout, "15000ns * ms = 15s")
}

func TestConfig_DurationNewFormat(t *testing.T) {
	cfg := Config{
		Name:                   "test",
		Enable:                 "true",
		ProducerRequestTimeout: 10 * time.Second,
		ProducerMessageTimeout: 15 * time.Second,
	}
	_ = tools.DoTagFunc(&cfg, []tools.FnObj{{Fn: tools.SetDefaultValueIfNil}})

	if cfg.ProducerRequestTimeout < time.Millisecond {
		cfg.ProducerRequestTimeout = cfg.ProducerRequestTimeout * time.Millisecond
	}
	if cfg.ProducerMessageTimeout < time.Millisecond {
		cfg.ProducerMessageTimeout = cfg.ProducerMessageTimeout * time.Millisecond
	}

	assert.Equal(t, 10*time.Second, cfg.ProducerRequestTimeout, "10s stays 10s")
	assert.Equal(t, 15*time.Second, cfg.ProducerMessageTimeout, "15s stays 15s")
}

func TestConfig_DurationSmallButValid(t *testing.T) {
	cfg := Config{
		Name:                   "test",
		Enable:                 "true",
		ProducerRequestTimeout: 500 * time.Millisecond,
		ProducerMessageTimeout: 1 * time.Second,
	}
	_ = tools.DoTagFunc(&cfg, []tools.FnObj{{Fn: tools.SetDefaultValueIfNil}})

	if cfg.ProducerRequestTimeout < time.Millisecond {
		cfg.ProducerRequestTimeout = cfg.ProducerRequestTimeout * time.Millisecond
	}
	if cfg.ProducerMessageTimeout < time.Millisecond {
		cfg.ProducerMessageTimeout = cfg.ProducerMessageTimeout * time.Millisecond
	}

	assert.Equal(t, 500*time.Millisecond, cfg.ProducerRequestTimeout, "500ms stays 500ms")
	assert.Equal(t, 1*time.Second, cfg.ProducerMessageTimeout, "1s stays 1s")
}
