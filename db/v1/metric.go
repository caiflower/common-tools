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

package dbv1

import (
	"context"
	"database/sql"
	"strings"
	"sync"
	"time"

	"github.com/caiflower/common-tools/global/env"
	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/uptrace/bun"
)

var metricsOnce sync.Once

var queryDurationBuckets = []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}

var (
	globalQueryTotal    *prometheus.CounterVec
	globalQueryDuration *prometheus.HistogramVec

	globalStatsMaxOpenConnections prometheus.GaugeVec
	globalStatsOpenConnections    prometheus.GaugeVec
	globalStatsInUse              prometheus.GaugeVec
	globalStatsIdle               prometheus.GaugeVec
	globalStatsWaitCount          prometheus.CounterVec
	globalStatsWaitDuration       prometheus.CounterVec
	globalStatsMaxIdleClosed      prometheus.CounterVec
	globalStatsMaxLifetimeClosed  prometheus.CounterVec
	globalStatsMaxIdleTimeClosed  prometheus.CounterVec
)

func ensureMetricsRegistered() {
	metricsOnce.Do(func() {
		initMetrics()
	})
}

func initMetrics() {
	constLabels := prometheus.Labels{"ip": env.GetLocalHostIP()}
	if env.GetNamespace() != "" {
		constLabels["namespace"] = env.GetNamespace()
		constLabels["app"] = env.GetApp()
	}

	globalQueryTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name:        "db_query_total",
			Help:        "Total number of database queries executed.",
			ConstLabels: constLabels,
		},
		[]string{"dialect", "database", "status"},
	)
	globalQueryDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:        "db_query_duration_seconds",
			Help:        "Histogram of database query execution durations in seconds.",
			Buckets:     queryDurationBuckets,
			ConstLabels: constLabels,
		},
		[]string{"dialect", "database"},
	)

	poolLabels := []string{"dialect", "database"}

	globalStatsMaxOpenConnections = *prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name:        "db_stats_max_open_connections",
			Help:        "Maximum number of open connections to the database.",
			ConstLabels: constLabels,
		},
		poolLabels,
	)
	globalStatsOpenConnections = *prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name:        "db_stats_open_connections",
			Help:        "The number of established connections both in use and idle.",
			ConstLabels: constLabels,
		},
		poolLabels,
	)
	globalStatsInUse = *prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name:        "db_stats_in_use",
			Help:        "The number of connections currently in use.",
			ConstLabels: constLabels,
		},
		poolLabels,
	)
	globalStatsIdle = *prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name:        "db_stats_idle",
			Help:        "The number of idle connections.",
			ConstLabels: constLabels,
		},
		poolLabels,
	)
	globalStatsWaitCount = *prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name:        "db_stats_wait_count_total",
			Help:        "The total number of connections waited for.",
			ConstLabels: constLabels,
		},
		poolLabels,
	)
	globalStatsWaitDuration = *prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name:        "db_stats_wait_duration_seconds_total",
			Help:        "The total time blocked waiting for a new connection.",
			ConstLabels: constLabels,
		},
		poolLabels,
	)
	globalStatsMaxIdleClosed = *prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name:        "db_stats_max_idle_closed_total",
			Help:        "The total number of connections closed due to SetMaxIdleConns.",
			ConstLabels: constLabels,
		},
		poolLabels,
	)
	globalStatsMaxLifetimeClosed = *prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name:        "db_stats_max_lifetime_closed_total",
			Help:        "The total number of connections closed due to SetConnMaxLifetime.",
			ConstLabels: constLabels,
		},
		poolLabels,
	)
	globalStatsMaxIdleTimeClosed = *prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name:        "db_stats_max_idle_time_closed_total",
			Help:        "The total number of connections closed due to SetConnMaxIdleTime.",
			ConstLabels: constLabels,
		},
		poolLabels,
	)

	for _, c := range []prometheus.Collector{
		globalQueryTotal,
		globalQueryDuration,
		&globalStatsMaxOpenConnections,
		&globalStatsOpenConnections,
		&globalStatsInUse,
		&globalStatsIdle,
		&globalStatsWaitCount,
		&globalStatsWaitDuration,
		&globalStatsMaxIdleClosed,
		&globalStatsMaxLifetimeClosed,
		&globalStatsMaxIdleTimeClosed,
	} {
		if err := prometheus.Register(c); err != nil {
			if _, ok := err.(prometheus.AlreadyRegisteredError); !ok {
				logger.Error("failed to register prometheus collector: %v", err)
			}
		}
	}
}

type metricsCollector struct {
	dialect string
	dbName  string
	labels  []string
}

func newMetricsCollector(config *Config) *metricsCollector {
	return &metricsCollector{
		dialect: config.Dialect,
		dbName:  config.DbName,
		labels:  []string{config.Dialect, config.DbName},
	}
}

func (m *metricsCollector) recordQuery(err error, duration time.Duration) {
	status := "ok"
	if err != nil {
		status = "error"
	}
	globalQueryTotal.WithLabelValues(append(m.labels, status)...).Inc()
	globalQueryDuration.WithLabelValues(m.labels...).Observe(duration.Seconds())
}

func (m *metricsCollector) startPoolStats(ctx context.Context, db *bun.DB) {
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		var prevWaitCount, prevMaxIdleClosed, prevMaxLifetimeClosed, prevMaxIdleTimeClosed float64
		var prevWaitDuration float64

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				stats := db.Stats()
				m.setDBStats(stats, &prevWaitCount, &prevWaitDuration, &prevMaxIdleClosed, &prevMaxLifetimeClosed, &prevMaxIdleTimeClosed)
			}
		}
	}()
}

func (m *metricsCollector) setDBStats(
	stats sql.DBStats,
	prevWaitCount *float64,
	prevWaitDuration *float64,
	prevMaxIdleClosed *float64,
	prevMaxLifetimeClosed *float64,
	prevMaxIdleTimeClosed *float64,
) {
	globalStatsMaxOpenConnections.WithLabelValues(m.labels...).Set(float64(stats.MaxOpenConnections))
	globalStatsOpenConnections.WithLabelValues(m.labels...).Set(float64(stats.OpenConnections))
	globalStatsInUse.WithLabelValues(m.labels...).Set(float64(stats.InUse))
	globalStatsIdle.WithLabelValues(m.labels...).Set(float64(stats.Idle))

	addCounterDelta(&globalStatsWaitCount, m.labels, float64(stats.WaitCount), prevWaitCount)
	addCounterDelta(&globalStatsWaitDuration, m.labels, stats.WaitDuration.Seconds(), prevWaitDuration)
	addCounterDelta(&globalStatsMaxIdleClosed, m.labels, float64(stats.MaxIdleClosed), prevMaxIdleClosed)
	addCounterDelta(&globalStatsMaxLifetimeClosed, m.labels, float64(stats.MaxLifetimeClosed), prevMaxLifetimeClosed)
	addCounterDelta(&globalStatsMaxIdleTimeClosed, m.labels, float64(stats.MaxIdleTimeClosed), prevMaxIdleTimeClosed)
}

func addCounterDelta(counterVec *prometheus.CounterVec, labels []string, current float64, prev *float64) {
	if current >= *prev {
		counterVec.WithLabelValues(labels...).Add(current - *prev)
	} else {
		logger.Warn("prometheus counter reset detected for %s, prev=%v current=%v", counterVec.WithLabelValues(labels...).Desc().String(), *prev, current)
		counterVec.WithLabelValues(labels...).Add(current)
	}
	*prev = current
}

func isMetricsQuery(query string) bool {
	q := strings.TrimSpace(strings.ToLower(query))
	return strings.HasPrefix(q, "select") &&
		strings.Contains(q, "information_schema")
}
