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

package redisv1

import (
	"context"
	"errors"
	"testing"

	xredis "github.com/caiflower/common-tools/redis"
	"github.com/go-redis/redis/v8"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	tassert "github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// swapTestMetrics replaces the global xredis metric vectors with fresh ones
// backed by an isolated registry, returning a teardown function that restores
// the originals.
func swapTestMetrics(t *testing.T) (*prometheus.Registry, func()) {
	t.Helper()

	// Save originals.
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

func TestStatusLabel(t *testing.T) {
	tassert.Equal(t, "ok", statusLabel(nil))
	tassert.Equal(t, "ok", statusLabel(redis.Nil))
	tassert.Equal(t, "error", statusLabel(errors.New("connection refused")))
}

func TestMetricsHook_ImplementsHookInterface(t *testing.T) {
	var _ redis.Hook = (*MetricsHook)(nil)
}

func TestMetricsHook_BeforeProcess_InjectsStartTime(t *testing.T) {
	_, teardown := swapTestMetrics(t)
	defer teardown()

	hook := &MetricsHook{addr: "127.0.0.1:6379"}
	ctx := context.Background()

	cmd := redis.NewStatusCmd(ctx, "ping")
	outCtx, err := hook.BeforeProcess(ctx, cmd)

	require.NoError(t, err)
	val := outCtx.Value(contextKey{})
	tassert.NotNil(t, val, "start time should be injected into context")
}

func TestMetricsHook_AfterProcess_OkCommand(t *testing.T) {
	reg, teardown := swapTestMetrics(t)
	defer teardown()

	hook := &MetricsHook{addr: "127.0.0.1:6379"}
	ctx := context.Background()

	cmd := redis.NewStatusCmd(ctx, "get", "key")
	ctx, _ = hook.BeforeProcess(ctx, cmd)
	// cmd.Err() is nil by default → status="ok"
	require.NoError(t, hook.AfterProcess(ctx, cmd))

	count := testutil.ToFloat64(xredis.CmdTotal.WithLabelValues("127.0.0.1:6379", "get", "ok"))
	tassert.Equal(t, float64(1), count)

	mfs, err := reg.Gather()
	require.NoError(t, err)
	found := false
	for _, mf := range mfs {
		if mf.GetName() == "redis_command_duration_seconds" {
			found = true
			break
		}
	}
	tassert.True(t, found, "duration histogram should be populated")
}

func TestMetricsHook_AfterProcess_ErrorCommand(t *testing.T) {
	_, teardown := swapTestMetrics(t)
	defer teardown()

	hook := &MetricsHook{addr: "127.0.0.1:6379"}
	ctx := context.Background()

	cmd := redis.NewStatusCmd(ctx, "set", "key", "value")
	cmd.SetErr(errors.New("WRONGTYPE"))
	ctx, _ = hook.BeforeProcess(ctx, cmd)
	require.NoError(t, hook.AfterProcess(ctx, cmd))

	count := testutil.ToFloat64(xredis.CmdTotal.WithLabelValues("127.0.0.1:6379", "set", "error"))
	tassert.Equal(t, float64(1), count)
}

func TestMetricsHook_AfterProcess_RedisNilIsOk(t *testing.T) {
	_, teardown := swapTestMetrics(t)
	defer teardown()

	hook := &MetricsHook{addr: "127.0.0.1:6379"}
	ctx := context.Background()

	cmd := redis.NewStatusCmd(ctx, "get", "missing")
	cmd.SetErr(redis.Nil)
	ctx, _ = hook.BeforeProcess(ctx, cmd)
	require.NoError(t, hook.AfterProcess(ctx, cmd))

	count := testutil.ToFloat64(xredis.CmdTotal.WithLabelValues("127.0.0.1:6379", "get", "ok"))
	tassert.Equal(t, float64(1), count)
}

func TestMetricsHook_Pipeline_OkPath(t *testing.T) {
	_, teardown := swapTestMetrics(t)
	defer teardown()

	hook := &MetricsHook{addr: "127.0.0.1:6379"}
	ctx := context.Background()

	cmds := []redis.Cmder{
		redis.NewStatusCmd(ctx, "set", "k1", "v1"),
		redis.NewStatusCmd(ctx, "set", "k2", "v2"),
	}

	ctx, err := hook.BeforeProcessPipeline(ctx, cmds)
	require.NoError(t, err)
	require.NoError(t, hook.AfterProcessPipeline(ctx, cmds))

	count := testutil.ToFloat64(xredis.PipTotal.WithLabelValues("127.0.0.1:6379", "ok"))
	tassert.Equal(t, float64(2), count, "pipeline should count all commands")
}

func TestMetricsHook_Pipeline_ErrorPath(t *testing.T) {
	_, teardown := swapTestMetrics(t)
	defer teardown()

	hook := &MetricsHook{addr: "127.0.0.1:6379"}
	ctx := context.Background()

	cmd1 := redis.NewStatusCmd(ctx, "set", "k1", "v1")
	cmd2 := redis.NewStatusCmd(ctx, "set", "k2", "v2")
	cmd2.SetErr(errors.New("WRONGTYPE"))

	cmds := []redis.Cmder{cmd1, cmd2}
	ctx, err := hook.BeforeProcessPipeline(ctx, cmds)
	require.NoError(t, err)
	require.NoError(t, hook.AfterProcessPipeline(ctx, cmds))

	count := testutil.ToFloat64(xredis.PipTotal.WithLabelValues("127.0.0.1:6379", "error"))
	tassert.Equal(t, float64(2), count)
}
