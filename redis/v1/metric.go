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
	"strings"
	"time"

	"github.com/caiflower/common-tools/global/env"
	"github.com/go-redis/redis/v8"
	"github.com/prometheus/client_golang/prometheus"
)

// contextKey is a private type to avoid context key collisions.
type contextKey struct{}

// durationBuckets is tuned for Redis latency characteristics:
// sub-5ms fine-grained (normal range), 5ms~50ms medium (mild jitter),
// 50ms~1s coarse (slow query / alert zone).
var durationBuckets = []float64{0.0001, 0.0005, 0.001, 0.002, 0.005, 0.01, 0.025, 0.05, 0.1, 0.3, 1}

// package-level metric vectors shared across all clients.
var (
	globalCmdTotal      *prometheus.CounterVec
	globalCmdDuration   *prometheus.HistogramVec
	globalPipTotal      *prometheus.CounterVec
	globalPipDuration   *prometheus.HistogramVec
	globalPoolIdleConns *prometheus.GaugeVec
	globalPoolTotalConn *prometheus.GaugeVec
	globalPoolStale     *prometheus.CounterVec
)

func init() {
	constLabels := prometheus.Labels{"ip": env.GetLocalHostIP()}
	if env.GetNamespace() != "" {
		constLabels["namespace"] = env.GetNamespace()
		constLabels["app"] = env.GetApp()
	}

	globalCmdTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "redis_commands_total", Help: "Total number of Redis commands executed.", ConstLabels: constLabels},
		[]string{"addr", "command", "status"},
	)
	globalCmdDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{Name: "redis_command_duration_seconds", Help: "Histogram of Redis command execution durations in seconds.", Buckets: durationBuckets, ConstLabels: constLabels},
		[]string{"addr", "command"},
	)
	globalPipTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "redis_pipeline_commands_total", Help: "Total number of commands executed inside Redis pipelines.", ConstLabels: constLabels},
		[]string{"addr", "status"},
	)
	globalPipDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{Name: "redis_pipeline_duration_seconds", Help: "Histogram of Redis pipeline batch execution durations in seconds.", Buckets: durationBuckets, ConstLabels: constLabels},
		[]string{"addr"},
	)
	globalPoolIdleConns = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{Name: "redis_pool_idle_conns", Help: "Number of idle connections in the Redis connection pool.", ConstLabels: constLabels},
		[]string{"addr"},
	)
	globalPoolTotalConn = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{Name: "redis_pool_total_conns", Help: "Total number of connections in the Redis connection pool.", ConstLabels: constLabels},
		[]string{"addr"},
	)
	globalPoolStale = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "redis_pool_stale_conns_total", Help: "Total number of stale connections evicted from the Redis connection pool.", ConstLabels: constLabels},
		[]string{"addr"},
	)

	_ = prometheus.Register(globalCmdTotal)
	_ = prometheus.Register(globalCmdDuration)
	_ = prometheus.Register(globalPipTotal)
	_ = prometheus.Register(globalPipDuration)
	_ = prometheus.Register(globalPoolIdleConns)
	_ = prometheus.Register(globalPoolTotalConn)
	_ = prometheus.Register(globalPoolStale)
}

// MetricsHook implements redis.Hook to collect Prometheus metrics per command.
type MetricsHook struct {
	addr          string
	cmdTotal      *prometheus.CounterVec
	cmdDuration   *prometheus.HistogramVec
	pipelineTotal *prometheus.CounterVec
	pipelineDur   *prometheus.HistogramVec
}

var _ redis.Hook = (*MetricsHook)(nil)

func newMetricsHook(config *Config) *MetricsHook {
	addr := "cluster"
	if config.Mode != ClusterMode {
		addr = config.Addrs[0]
	}
	return &MetricsHook{
		addr:          addr,
		cmdTotal:      globalCmdTotal,
		cmdDuration:   globalCmdDuration,
		pipelineTotal: globalPipTotal,
		pipelineDur:   globalPipDuration,
	}
}

func (h *MetricsHook) BeforeProcess(ctx context.Context, _ redis.Cmder) (context.Context, error) {
	return context.WithValue(ctx, contextKey{}, time.Now()), nil
}

func (h *MetricsHook) AfterProcess(ctx context.Context, cmd redis.Cmder) error {
	start, _ := ctx.Value(contextKey{}).(time.Time)
	elapsed := time.Since(start).Seconds()

	cmdName := strings.ToLower(cmd.FullName())
	status := statusLabel(cmd.Err())

	h.cmdTotal.WithLabelValues(h.addr, cmdName, status).Inc()
	h.cmdDuration.WithLabelValues(h.addr, cmdName).Observe(elapsed)
	return nil
}

func (h *MetricsHook) BeforeProcessPipeline(ctx context.Context, _ []redis.Cmder) (context.Context, error) {
	return context.WithValue(ctx, contextKey{}, time.Now()), nil
}

func (h *MetricsHook) AfterProcessPipeline(ctx context.Context, cmds []redis.Cmder) error {
	start, _ := ctx.Value(contextKey{}).(time.Time)
	elapsed := time.Since(start).Seconds()

	hasErr := false
	for _, cmd := range cmds {
		if cmd.Err() != nil && cmd.Err() != redis.Nil {
			hasErr = true
			break
		}
	}
	status := "ok"
	if hasErr {
		status = "error"
	}

	h.pipelineTotal.WithLabelValues(h.addr, status).Add(float64(len(cmds)))
	h.pipelineDur.WithLabelValues(h.addr).Observe(elapsed)
	return nil
}

// statusLabel returns "ok" for nil or redis.Nil errors, "error" otherwise.
func statusLabel(err error) string {
	if err == nil || err == redis.Nil {
		return "ok"
	}
	return "error"
}

// startPoolMetrics launches a background goroutine that periodically collects
// connection pool statistics and updates the corresponding Prometheus gauges/counters.
func startPoolMetrics(ctx context.Context, c *redisClient) {
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		var prevStale uint32

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				collectPoolStats(c, &prevStale)
			}
		}
	}()
}

func collectPoolStats(c *redisClient, prevStale *uint32) {
	switch c.config.Mode {
	case ClusterMode:
		var totalIdle, totalConns, totalStale uint32
		_ = c.clusterClient.ForEachShard(context.Background(), func(ctx context.Context, shard *redis.Client) error {
			s := shard.PoolStats()
			totalIdle += s.IdleConns
			totalConns += s.TotalConns
			totalStale += s.StaleConns
			return nil
		})
		globalPoolIdleConns.WithLabelValues("cluster").Set(float64(totalIdle))
		globalPoolTotalConn.WithLabelValues("cluster").Set(float64(totalConns))
		if totalStale > *prevStale {
			globalPoolStale.WithLabelValues("cluster").Add(float64(totalStale - *prevStale))
		}
		*prevStale = totalStale
	default:
		addr := c.config.Addrs[0]
		s := c.client.PoolStats()
		globalPoolIdleConns.WithLabelValues(addr).Set(float64(s.IdleConns))
		globalPoolTotalConn.WithLabelValues(addr).Set(float64(s.TotalConns))
		if s.StaleConns > *prevStale {
			globalPoolStale.WithLabelValues(addr).Add(float64(s.StaleConns - *prevStale))
		}
		*prevStale = s.StaleConns
	}
}
