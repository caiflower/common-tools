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

package redis

import (
	"time"

	"github.com/caiflower/common-tools/global/env"
	"github.com/prometheus/client_golang/prometheus"
)

// DurationBuckets is tuned for Redis latency characteristics:
// sub-5ms fine-grained (normal range), 5ms~50ms medium (mild jitter),
// 50ms~1s coarse (slow query / alert zone).
var DurationBuckets = []float64{0.0001, 0.0005, 0.001, 0.002, 0.005, 0.01, 0.025, 0.05, 0.1, 0.3, 1}

// PoolStats holds connection pool statistics common across go-redis v8 and v9.
type PoolStats struct {
	Hits       uint32
	Misses     uint32
	Timeouts   uint32
	TotalConns uint32
	IdleConns  uint32
	StaleConns uint32
}

// Metric vectors shared across all Redis client versions.
// Initialized by InitMetrics; v1/v2 call it from their init().
var (
	CmdTotal      *prometheus.CounterVec
	CmdDuration   *prometheus.HistogramVec
	PipTotal      *prometheus.CounterVec
	PipDuration   *prometheus.HistogramVec
	PoolIdleConns *prometheus.GaugeVec
	PoolTotalConn *prometheus.GaugeVec
	PoolStale     *prometheus.CounterVec
)

// InitMetrics creates and registers all Prometheus metric vectors.
// Safe to call from both v1 and v2 init(); uses RegisterOrReuse to avoid
// duplicate registration panics when both versions coexist.
func InitMetrics() {
	constLabels := prometheus.Labels{"ip": env.GetLocalHostIP()}
	if env.GetNamespace() != "" {
		constLabels["namespace"] = env.GetNamespace()
		constLabels["app"] = env.GetApp()
	}

	CmdTotal = RegisterOrReuse(
		prometheus.NewCounterVec(
			prometheus.CounterOpts{Name: "redis_commands_total", Help: "Total number of Redis commands executed.", ConstLabels: constLabels},
			[]string{"addr", "command", "status"},
		),
	).(*prometheus.CounterVec)

	CmdDuration = RegisterOrReuse(
		prometheus.NewHistogramVec(
			prometheus.HistogramOpts{Name: "redis_command_duration_seconds", Help: "Histogram of Redis command execution durations in seconds.", Buckets: DurationBuckets, ConstLabels: constLabels},
			[]string{"addr", "command"},
		),
	).(*prometheus.HistogramVec)

	PipTotal = RegisterOrReuse(
		prometheus.NewCounterVec(
			prometheus.CounterOpts{Name: "redis_pipeline_commands_total", Help: "Total number of commands executed inside Redis pipelines.", ConstLabels: constLabels},
			[]string{"addr", "status"},
		),
	).(*prometheus.CounterVec)

	PipDuration = RegisterOrReuse(
		prometheus.NewHistogramVec(
			prometheus.HistogramOpts{Name: "redis_pipeline_duration_seconds", Help: "Histogram of Redis pipeline batch execution durations in seconds.", Buckets: DurationBuckets, ConstLabels: constLabels},
			[]string{"addr"},
		),
	).(*prometheus.HistogramVec)

	PoolIdleConns = RegisterOrReuse(
		prometheus.NewGaugeVec(
			prometheus.GaugeOpts{Name: "redis_pool_idle_conns", Help: "Number of idle connections in the Redis connection pool.", ConstLabels: constLabels},
			[]string{"addr"},
		),
	).(*prometheus.GaugeVec)

	PoolTotalConn = RegisterOrReuse(
		prometheus.NewGaugeVec(
			prometheus.GaugeOpts{Name: "redis_pool_total_conns", Help: "Total number of connections in the Redis connection pool.", ConstLabels: constLabels},
			[]string{"addr"},
		),
	).(*prometheus.GaugeVec)

	PoolStale = RegisterOrReuse(
		prometheus.NewCounterVec(
			prometheus.CounterOpts{Name: "redis_pool_stale_conns_total", Help: "Total number of stale connections evicted from the Redis connection pool.", ConstLabels: constLabels},
			[]string{"addr"},
		),
	).(*prometheus.CounterVec)
}

// RegisterOrReuse registers a collector or returns the existing one if already registered.
// This enables v1 and v2 to coexist and write to the same metric vectors.
func RegisterOrReuse(c prometheus.Collector) prometheus.Collector {
	if err := prometheus.Register(c); err != nil {
		if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
			return are.ExistingCollector
		}
	}
	return c
}

// RecordCommand records a single command's metrics (counter + duration histogram).
func RecordCommand(addr, cmdName, status string, elapsed time.Duration) {
	CmdTotal.WithLabelValues(addr, cmdName, status).Inc()
	CmdDuration.WithLabelValues(addr, cmdName).Observe(elapsed.Seconds())
}

// RecordPipeline records pipeline batch metrics (counter + duration histogram).
func RecordPipeline(addr string, cmdCount int, hasErr bool, elapsed time.Duration) {
	status := "ok"
	if hasErr {
		status = "error"
	}
	PipTotal.WithLabelValues(addr, status).Add(float64(cmdCount))
	PipDuration.WithLabelValues(addr).Observe(elapsed.Seconds())
}

// UpdatePoolStats updates pool-level Prometheus gauges/counters from a shared PoolStats.
// prevStale tracks the previous stale count to compute deltas for the monotonic counter.
func UpdatePoolStats(addr string, stats PoolStats, prevStale *uint32) {
	PoolIdleConns.WithLabelValues(addr).Set(float64(stats.IdleConns))
	PoolTotalConn.WithLabelValues(addr).Set(float64(stats.TotalConns))
	if stats.StaleConns >= *prevStale {
		PoolStale.WithLabelValues(addr).Add(float64(stats.StaleConns - *prevStale))
	} else {
		// Counter reset (e.g. Redis restart): treat current stale count as
		// an approximate delta since we cannot determine the true increment.
		PoolStale.WithLabelValues(addr).Add(float64(stats.StaleConns))
	}
	*prevStale = stats.StaleConns
}
