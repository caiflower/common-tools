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
	"errors"
	"net"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	xredis "github.com/caiflower/common-tools/redis"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/redis/go-redis/v9"
	tassert "github.com/stretchr/testify/assert"
	trequire "github.com/stretchr/testify/require"
)

// swapTestMetrics replaces the global xredis metric vectors with fresh ones
// backed by an isolated registry, returning a teardown function that restores
// the originals.
func swapTestMetrics(t *testing.T) (*prometheus.Registry, func()) {
	t.Helper()

	origCmdTotal := xredis.CmdTotal
	origCmdDuration := xredis.CmdDuration
	origPipTotal := xredis.PipTotal
	origPipDuration := xredis.PipDuration

	reg := prometheus.NewRegistry()

	xredis.CmdTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "redis_commands_total", Help: "test"},
		[]string{"addr", "command", "status"},
	)
	xredis.CmdDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{Name: "redis_command_duration_seconds", Help: "test", Buckets: xredis.DurationBuckets},
		[]string{"addr", "command"},
	)
	xredis.PipTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "redis_pipeline_commands_total", Help: "test"},
		[]string{"addr", "status"},
	)
	xredis.PipDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{Name: "redis_pipeline_duration_seconds", Help: "test", Buckets: xredis.DurationBuckets},
		[]string{"addr"},
	)

	reg.MustRegister(xredis.CmdTotal, xredis.CmdDuration, xredis.PipTotal, xredis.PipDuration)

	teardown := func() {
		xredis.CmdTotal = origCmdTotal
		xredis.CmdDuration = origCmdDuration
		xredis.PipTotal = origPipTotal
		xredis.PipDuration = origPipDuration
	}
	return reg, teardown
}

func setupTestClient(t *testing.T, enableMetrics bool) (RedisClient, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)

	cfg := Config{
		Addrs:         []string{mr.Addr()},
		DB:            0,
		EnableMetrics: "true",
	}
	if !enableMetrics {
		cfg.EnableMetrics = "false"
	}

	client, err := NewRedisClient(cfg)
	trequire.NoError(t, err, "NewRedisClient should succeed with miniredis")
	return client, mr
}

func TestNewRedisClient_PasswordFromEnv(t *testing.T) {
	mr := miniredis.RunT(t)
	mr.RequireAuth("env-secret")

	t.Setenv("REDIS_PASSWORD", "env-secret")
	client, err := NewRedisClient(Config{Addrs: []string{mr.Addr()}, EnableMetrics: "false"})
	trequire.NoError(t, err, "env password should be used")
	client.Close()
}

func TestNewRedisClient_PasswordEnvOverridesConfig(t *testing.T) {
	mr := miniredis.RunT(t)
	mr.RequireAuth("env-secret")

	t.Setenv("REDIS_PASSWORD", "env-secret")
	client, err := NewRedisClient(Config{Addrs: []string{mr.Addr()}, Password: "config-secret", EnableMetrics: "false"})
	trequire.NoError(t, err, "env password should override config password")
	client.Close()
}

// --- statusLabel tests ---

func TestStatusLabel(t *testing.T) {
	tassert.Equal(t, "ok", statusLabel(nil))
	tassert.Equal(t, "ok", statusLabel(redis.Nil))
	tassert.Equal(t, "error", statusLabel(errors.New("connection refused")))
}

// --- MetricsHook interface compliance ---

func TestMetricsHook_ImplementsHookInterface(t *testing.T) {
	var _ redis.Hook = (*MetricsHook)(nil)
}

// --- newMetricsHook tests ---

func TestNewMetricsHook_NonClusterMode(t *testing.T) {
	hook := newMetricsHook(&Config{Mode: "standalone", Addrs: []string{"10.0.0.1:6379"}})
	tassert.Equal(t, "10.0.0.1:6379", hook.addr)
}

func TestNewMetricsHook_ClusterMode(t *testing.T) {
	hook := newMetricsHook(&Config{Mode: xredis.ClusterMode, Addrs: []string{"10.0.0.1:6379", "10.0.0.2:6379"}})
	tassert.Equal(t, "cluster", hook.addr)
}

// --- ProcessHook unit tests ---

func TestProcessHook_OkCommand(t *testing.T) {
	_, teardown := swapTestMetrics(t)
	defer teardown()

	hook := &MetricsHook{addr: "127.0.0.1:6379"}
	ctx := context.Background()

	cmd := redis.NewStatusCmd(ctx, "get", "key")

	processFn := hook.ProcessHook(func(ctx context.Context, cmd redis.Cmder) error {
		return nil
	})
	err := processFn(ctx, cmd)

	trequire.NoError(t, err)
	count := testutil.ToFloat64(xredis.CmdTotal.WithLabelValues("127.0.0.1:6379", "get", "ok"))
	tassert.Equal(t, float64(1), count)
}

func TestProcessHook_ErrorCommand(t *testing.T) {
	_, teardown := swapTestMetrics(t)
	defer teardown()

	hook := &MetricsHook{addr: "127.0.0.1:6379"}
	ctx := context.Background()

	cmd := redis.NewStatusCmd(ctx, "set", "key", "value")
	cmd.SetErr(errors.New("WRONGTYPE"))

	processFn := hook.ProcessHook(func(ctx context.Context, cmd redis.Cmder) error {
		return nil // next returns nil, but cmd has error set
	})
	err := processFn(ctx, cmd)

	trequire.NoError(t, err)
	count := testutil.ToFloat64(xredis.CmdTotal.WithLabelValues("127.0.0.1:6379", "set", "error"))
	tassert.Equal(t, float64(1), count)
}

func TestProcessHook_RedisNilIsOk(t *testing.T) {
	_, teardown := swapTestMetrics(t)
	defer teardown()

	hook := &MetricsHook{addr: "127.0.0.1:6379"}
	ctx := context.Background()

	cmd := redis.NewStatusCmd(ctx, "get", "missing")
	cmd.SetErr(redis.Nil)

	processFn := hook.ProcessHook(func(ctx context.Context, cmd redis.Cmder) error {
		return nil
	})
	err := processFn(ctx, cmd)

	trequire.NoError(t, err)
	count := testutil.ToFloat64(xredis.CmdTotal.WithLabelValues("127.0.0.1:6379", "get", "ok"))
	tassert.Equal(t, float64(1), count)
}

func TestProcessHook_DurationRecorded(t *testing.T) {
	reg, teardown := swapTestMetrics(t)
	defer teardown()

	hook := &MetricsHook{addr: "127.0.0.1:6379"}
	ctx := context.Background()

	cmd := redis.NewStatusCmd(ctx, "ping")
	processFn := hook.ProcessHook(func(ctx context.Context, cmd redis.Cmder) error {
		time.Sleep(1 * time.Millisecond) // ensure non-zero duration
		return nil
	})
	trequire.NoError(t, processFn(ctx, cmd))

	mfs, err := reg.Gather()
	trequire.NoError(t, err)
	found := false
	for _, mf := range mfs {
		if mf.GetName() == "redis_command_duration_seconds" {
			found = true
			break
		}
	}
	tassert.True(t, found, "duration histogram should be populated")
}

// --- ProcessPipelineHook unit tests ---

func TestProcessPipelineHook_OkPath(t *testing.T) {
	_, teardown := swapTestMetrics(t)
	defer teardown()

	hook := &MetricsHook{addr: "127.0.0.1:6379"}
	ctx := context.Background()

	cmds := []redis.Cmder{
		redis.NewStatusCmd(ctx, "set", "k1", "v1"),
		redis.NewStatusCmd(ctx, "set", "k2", "v2"),
	}

	pipFn := hook.ProcessPipelineHook(func(ctx context.Context, cmds []redis.Cmder) error {
		return nil
	})
	err := pipFn(ctx, cmds)

	trequire.NoError(t, err)
	count := testutil.ToFloat64(xredis.PipTotal.WithLabelValues("127.0.0.1:6379", "ok"))
	tassert.Equal(t, float64(2), count, "pipeline should count all commands")
}

func TestProcessPipelineHook_ErrorPath(t *testing.T) {
	_, teardown := swapTestMetrics(t)
	defer teardown()

	hook := &MetricsHook{addr: "127.0.0.1:6379"}
	ctx := context.Background()

	cmd1 := redis.NewStatusCmd(ctx, "set", "k1", "v1")
	cmd2 := redis.NewStatusCmd(ctx, "set", "k2", "v2")
	cmd2.SetErr(errors.New("WRONGTYPE"))

	cmds := []redis.Cmder{cmd1, cmd2}
	pipFn := hook.ProcessPipelineHook(func(ctx context.Context, cmds []redis.Cmder) error {
		return nil
	})
	err := pipFn(ctx, cmds)

	trequire.NoError(t, err)
	count := testutil.ToFloat64(xredis.PipTotal.WithLabelValues("127.0.0.1:6379", "error"))
	tassert.Equal(t, float64(2), count)
}

func TestProcessPipelineHook_PipelineDurationRecorded(t *testing.T) {
	reg, teardown := swapTestMetrics(t)
	defer teardown()

	hook := &MetricsHook{addr: "127.0.0.1:6379"}
	ctx := context.Background()

	cmds := []redis.Cmder{
		redis.NewStatusCmd(ctx, "get", "k1"),
	}

	pipFn := hook.ProcessPipelineHook(func(ctx context.Context, cmds []redis.Cmder) error {
		time.Sleep(1 * time.Millisecond)
		return nil
	})
	trequire.NoError(t, pipFn(ctx, cmds))

	mfs, err := reg.Gather()
	trequire.NoError(t, err)
	found := false
	for _, mf := range mfs {
		if mf.GetName() == "redis_pipeline_duration_seconds" {
			found = true
			break
		}
	}
	tassert.True(t, found, "pipeline duration histogram should be populated")
}

// --- DialHook test ---

func TestDialHook_PassThrough(t *testing.T) {
	hook := &MetricsHook{addr: "127.0.0.1:6379"}

	called := false
	var inner redis.DialHook = func(ctx context.Context, network, addr string) (net.Conn, error) {
		called = true
		return nil, nil
	}
	wrapped := hook.DialHook(inner)
	tassert.NotNil(t, wrapped, "DialHook should return a non-nil function")
	tassert.False(t, called, "inner should not be called yet")

	// Invoke the wrapped dial hook
	_, err := wrapped(context.Background(), "tcp", "127.0.0.1:6379")
	tassert.NoError(t, err)
	tassert.True(t, called, "inner dial function should be called")
}

// --- Integration tests with miniredis ---

func TestIntegration_MetricsRecordedOnSetGet(t *testing.T) {
	_, teardown := swapTestMetrics(t)
	defer teardown()

	client, _ := setupTestClient(t, true)
	defer client.Close()
	ctx := context.Background()

	// Set a key
	err := client.Cmd().Set(ctx, client.Cmd().Key("metric_test"), "hello", 0).Err()
	trequire.NoError(t, err, "Set should succeed")

	// Verify set metric
	setCount := testutil.ToFloat64(xredis.CmdTotal.WithLabelValues(client.(*redisClient).config.Addrs[0], "set", "ok"))
	tassert.GreaterOrEqual(t, setCount, float64(1), "set command metric should be >= 1")

	// Get a key
	val, err := client.Cmd().Get(ctx, client.Cmd().Key("metric_test")).Result()
	trequire.NoError(t, err, "Get should succeed")
	tassert.Equal(t, "hello", val)

	// Verify get metric
	getCount := testutil.ToFloat64(xredis.CmdTotal.WithLabelValues(client.(*redisClient).config.Addrs[0], "get", "ok"))
	tassert.GreaterOrEqual(t, getCount, float64(1), "get command metric should be >= 1")
}

func TestIntegration_MetricsRecordedOnDel(t *testing.T) {
	_, teardown := swapTestMetrics(t)
	defer teardown()

	client, _ := setupTestClient(t, true)
	defer client.Close()
	ctx := context.Background()

	// Set then delete
	trequire.NoError(t, client.Cmd().Set(ctx, "del_metric_key", "v", 0).Err())
	deleted, err := client.Cmd().Del(ctx, "del_metric_key").Result()
	trequire.NoError(t, err, "Del should succeed")
	tassert.Equal(t, int64(1), deleted)

	delCount := testutil.ToFloat64(xredis.CmdTotal.WithLabelValues(client.(*redisClient).config.Addrs[0], "del", "ok"))
	tassert.GreaterOrEqual(t, delCount, float64(1), "del command metric should be >= 1")
}

func TestIntegration_MetricsRecordedOnGetMissing(t *testing.T) {
	_, teardown := swapTestMetrics(t)
	defer teardown()

	client, _ := setupTestClient(t, true)
	defer client.Close()
	ctx := context.Background()

	err := client.Cmd().Get(ctx, "nonexistent_metric_key").Err()
	tassert.Equal(t, redis.Nil, err, "Get on missing key should return redis.Nil")

	// redis.Nil is treated as "ok" in statusLabel
	getCount := testutil.ToFloat64(xredis.CmdTotal.WithLabelValues(client.(*redisClient).config.Addrs[0], "get", "ok"))
	tassert.GreaterOrEqual(t, getCount, float64(1), "get with redis.Nil should be recorded as ok")
}

func TestIntegration_NoMetricsWhenDisabled(t *testing.T) {
	_, teardown := swapTestMetrics(t)
	defer teardown()

	client, _ := setupTestClient(t, false)
	defer client.Close()
	ctx := context.Background()

	trequire.NoError(t, client.Cmd().Set(ctx, "no_metric_key", "v", 0).Err())

	// No metrics hook should have been registered, so counter should remain at 0.
	// Use CollectAndCount to avoid panicking on missing label values.
	count := testutil.CollectAndCount(xredis.CmdTotal)
	tassert.Equal(t, 0, count, "no metrics should be recorded when disabled")
}

func TestIntegration_PipelineMetrics(t *testing.T) {
	_, teardown := swapTestMetrics(t)
	defer teardown()

	client, _ := setupTestClient(t, true)
	defer client.Close()
	ctx := context.Background()

	rc, ok := client.(*redisClient)
	trequire.True(t, ok, "should be *redisClient")

	sc, ok := rc.redis.(*redis.Client)
	trequire.True(t, ok, "standalone mode should use *redis.Client")

	pipe := sc.Pipeline()
	pipe.Set(ctx, "pip_k1", "v1", 0)
	pipe.Set(ctx, "pip_k2", "v2", 0)
	pipe.Get(ctx, "pip_k1")
	_, err := pipe.Exec(ctx)
	trequire.NoError(t, err, "pipeline exec should succeed")

	pipCount := testutil.ToFloat64(xredis.PipTotal.WithLabelValues(rc.config.Addrs[0], "ok"))
	tassert.GreaterOrEqual(t, pipCount, float64(3), "pipeline should record all 3 commands as ok")
}

func TestIntegration_KeyPrefix(t *testing.T) {
	client, _ := setupTestClient(t, false)
	defer client.Close()

	cmd := client.Cmd()
	prefixed := cmd.Key("foo")
	tassert.Equal(t, "foo", prefixed, "no prefix configured, key should be unchanged")
}

func TestIntegration_KeyPrefixWithPrefix(t *testing.T) {
	mr := miniredis.RunT(t)

	cfg := Config{
		Addrs:     []string{mr.Addr()},
		DB:        0,
		KeyPrefix: "myapp",
	}

	client, err := NewRedisClient(cfg)
	trequire.NoError(t, err)
	defer client.Close()

	prefixed := client.Cmd().Key("bar")
	tassert.Equal(t, "myapp:bar", prefixed)

	ctx := context.Background()
	trequire.NoError(t, client.Cmd().Set(ctx, client.Cmd().Key("item"), "val", 0).Err())

	got, err := client.Cmd().Get(ctx, "myapp:item").Result()
	trequire.NoError(t, err)
	tassert.Equal(t, "val", got)
}

// --- collectPoolStats tests ---

func TestCollectPoolStats_Standalone(t *testing.T) {
	client, _ := setupTestClient(t, false)
	defer client.Close()

	rc, ok := client.(*redisClient)
	trequire.True(t, ok)

	var prevStale uint32
	collectPoolStats(rc, &prevStale)
	// Just verify it does not panic on a standalone client.
}

func TestCollectPoolStats_MultipleCollections(t *testing.T) {
	client, _ := setupTestClient(t, false)
	defer client.Close()

	rc, ok := client.(*redisClient)
	trequire.True(t, ok)

	// Run a command so pool has activity
	ctx := context.Background()
	trequire.NoError(t, client.Cmd().Set(ctx, "pool_test", "v", 0).Err())

	var prevStale uint32
	collectPoolStats(rc, &prevStale)
	collectPoolStats(rc, &prevStale)
	// No panic means the stats collection is stable across multiple calls.
}
