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
	"errors"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	tassert "github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newIsolatedCollector(t *testing.T, dialect, dbName string) (*metricsCollector, *prometheus.Registry) {
	t.Helper()

	reg := prometheus.NewRegistry()

	queryTotal := prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "db_query_total", Help: "test"},
		[]string{"dialect", "database", "status"},
	)
	queryDuration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{Name: "db_query_duration_seconds", Help: "test", Buckets: queryDurationBuckets},
		[]string{"dialect", "database"},
	)

	poolLabels := []string{"dialect", "database"}

	maxOpenConns := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{Name: "db_stats_max_open_connections", Help: "test"},
		poolLabels,
	)
	openConns := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{Name: "db_stats_open_connections", Help: "test"},
		poolLabels,
	)
	inUse := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{Name: "db_stats_in_use", Help: "test"},
		poolLabels,
	)
	idle := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{Name: "db_stats_idle", Help: "test"},
		poolLabels,
	)
	waitCount := prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "db_stats_wait_count_total", Help: "test"},
		poolLabels,
	)
	waitDuration := prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "db_stats_wait_duration_seconds_total", Help: "test"},
		poolLabels,
	)
	maxIdleClosed := prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "db_stats_max_idle_closed_total", Help: "test"},
		poolLabels,
	)
	maxLifetimeClosed := prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "db_stats_max_lifetime_closed_total", Help: "test"},
		poolLabels,
	)
	maxIdleTimeClosed := prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "db_stats_max_idle_time_closed_total", Help: "test"},
		poolLabels,
	)

	reg.MustRegister(queryTotal, queryDuration, maxOpenConns, openConns, inUse, idle,
		waitCount, waitDuration, maxIdleClosed, maxLifetimeClosed, maxIdleTimeClosed)

	globalQueryTotal = queryTotal
	globalQueryDuration = queryDuration
	globalStatsMaxOpenConnections = *maxOpenConns
	globalStatsOpenConnections = *openConns
	globalStatsInUse = *inUse
	globalStatsIdle = *idle
	globalStatsWaitCount = *waitCount
	globalStatsWaitDuration = *waitDuration
	globalStatsMaxIdleClosed = *maxIdleClosed
	globalStatsMaxLifetimeClosed = *maxLifetimeClosed
	globalStatsMaxIdleTimeClosed = *maxIdleTimeClosed

	collector := &metricsCollector{
		dialect: dialect,
		dbName:  dbName,
		labels:  []string{dialect, dbName},
	}

	return collector, reg
}

func TestMetricsCollector_RecordQuery_Ok(t *testing.T) {
	m, _ := newIsolatedCollector(t, "mysql", "order_db")

	m.recordQuery(nil, 50*time.Millisecond)

	count := testutil.ToFloat64(globalQueryTotal.WithLabelValues("mysql", "order_db", "ok"))
	tassert.Equal(t, float64(1), count)
}

func TestMetricsCollector_RecordQuery_Error(t *testing.T) {
	m, _ := newIsolatedCollector(t, "mysql", "order_db")

	m.recordQuery(errors.New("connection refused"), 10*time.Millisecond)

	okCount := testutil.ToFloat64(globalQueryTotal.WithLabelValues("mysql", "order_db", "ok"))
	errCount := testutil.ToFloat64(globalQueryTotal.WithLabelValues("mysql", "order_db", "error"))
	tassert.Equal(t, float64(0), okCount)
	tassert.Equal(t, float64(1), errCount)
}

func TestMetricsCollector_MultipleDBs_IndependentLabels(t *testing.T) {
	m1, _ := newIsolatedCollector(t, "mysql", "order_db")
	m2, _ := newIsolatedCollector(t, "pgsql", "user_db")

	m1.recordQuery(nil, 10*time.Millisecond)
	m1.recordQuery(nil, 20*time.Millisecond)
	m2.recordQuery(errors.New("timeout"), 5*time.Millisecond)

	mysqlOk := testutil.ToFloat64(globalQueryTotal.WithLabelValues("mysql", "order_db", "ok"))
	mysqlErr := testutil.ToFloat64(globalQueryTotal.WithLabelValues("mysql", "order_db", "error"))
	pgsqlOk := testutil.ToFloat64(globalQueryTotal.WithLabelValues("pgsql", "user_db", "ok"))
	pgsqlErr := testutil.ToFloat64(globalQueryTotal.WithLabelValues("pgsql", "user_db", "error"))

	tassert.Equal(t, float64(2), mysqlOk, "mysql order_db ok count")
	tassert.Equal(t, float64(0), mysqlErr, "mysql order_db error count")
	tassert.Equal(t, float64(0), pgsqlOk, "pgsql user_db ok count")
	tassert.Equal(t, float64(1), pgsqlErr, "pgsql user_db error count")
}

func TestMetricsCollector_SameDialect_DifferentDB(t *testing.T) {
	m1, _ := newIsolatedCollector(t, "mysql", "order_db")
	m2, _ := newIsolatedCollector(t, "mysql", "product_db")

	m1.recordQuery(nil, 10*time.Millisecond)
	m2.recordQuery(nil, 20*time.Millisecond)
	m2.recordQuery(errors.New("deadlock"), 30*time.Millisecond)

	orderOk := testutil.ToFloat64(globalQueryTotal.WithLabelValues("mysql", "order_db", "ok"))
	orderErr := testutil.ToFloat64(globalQueryTotal.WithLabelValues("mysql", "order_db", "error"))
	productOk := testutil.ToFloat64(globalQueryTotal.WithLabelValues("mysql", "product_db", "ok"))
	productErr := testutil.ToFloat64(globalQueryTotal.WithLabelValues("mysql", "product_db", "error"))

	tassert.Equal(t, float64(1), orderOk)
	tassert.Equal(t, float64(0), orderErr)
	tassert.Equal(t, float64(1), productOk)
	tassert.Equal(t, float64(1), productErr)
}

func TestMetricsCollector_SetDBStats_Gauges(t *testing.T) {
	m, _ := newIsolatedCollector(t, "mysql", "test_db")

	stats := sql.DBStats{
		MaxOpenConnections: 100,
		OpenConnections:    10,
		InUse:              3,
		Idle:               7,
		WaitCount:          0,
		WaitDuration:       0,
		MaxIdleClosed:      0,
		MaxLifetimeClosed:  0,
		MaxIdleTimeClosed:  0,
	}

	var prevWaitCount, prevWaitDuration, prevMaxIdleClosed, prevMaxLifetimeClosed, prevMaxIdleTimeClosed float64
	m.setDBStats(stats, &prevWaitCount, &prevWaitDuration, &prevMaxIdleClosed, &prevMaxLifetimeClosed, &prevMaxIdleTimeClosed)

	tassert.Equal(t, float64(100), testutil.ToFloat64(globalStatsMaxOpenConnections.WithLabelValues("mysql", "test_db")))
	tassert.Equal(t, float64(10), testutil.ToFloat64(globalStatsOpenConnections.WithLabelValues("mysql", "test_db")))
	tassert.Equal(t, float64(3), testutil.ToFloat64(globalStatsInUse.WithLabelValues("mysql", "test_db")))
	tassert.Equal(t, float64(7), testutil.ToFloat64(globalStatsIdle.WithLabelValues("mysql", "test_db")))
}

func TestMetricsCollector_SetDBStats_CounterDelta(t *testing.T) {
	m, _ := newIsolatedCollector(t, "mysql", "test_db")

	var prevWaitCount, prevWaitDuration, prevMaxIdleClosed, prevMaxLifetimeClosed, prevMaxIdleTimeClosed float64

	stats1 := sql.DBStats{
		WaitCount:         5,
		WaitDuration:      200 * time.Millisecond,
		MaxIdleClosed:     2,
		MaxLifetimeClosed: 1,
		MaxIdleTimeClosed: 0,
	}
	m.setDBStats(stats1, &prevWaitCount, &prevWaitDuration, &prevMaxIdleClosed, &prevMaxLifetimeClosed, &prevMaxIdleTimeClosed)

	tassert.Equal(t, float64(5), testutil.ToFloat64(globalStatsWaitCount.WithLabelValues("mysql", "test_db")))
	tassert.InDelta(t, 0.2, testutil.ToFloat64(globalStatsWaitDuration.WithLabelValues("mysql", "test_db")), 0.001)
	tassert.Equal(t, float64(2), testutil.ToFloat64(globalStatsMaxIdleClosed.WithLabelValues("mysql", "test_db")))

	stats2 := sql.DBStats{
		WaitCount:         8,
		WaitDuration:      350 * time.Millisecond,
		MaxIdleClosed:     5,
		MaxLifetimeClosed: 3,
		MaxIdleTimeClosed: 1,
	}
	m.setDBStats(stats2, &prevWaitCount, &prevWaitDuration, &prevMaxIdleClosed, &prevMaxLifetimeClosed, &prevMaxIdleTimeClosed)

	tassert.Equal(t, float64(8), testutil.ToFloat64(globalStatsWaitCount.WithLabelValues("mysql", "test_db")))
	tassert.InDelta(t, 0.35, testutil.ToFloat64(globalStatsWaitDuration.WithLabelValues("mysql", "test_db")), 0.001)
	tassert.Equal(t, float64(5), testutil.ToFloat64(globalStatsMaxIdleClosed.WithLabelValues("mysql", "test_db")))
	tassert.Equal(t, float64(3), testutil.ToFloat64(globalStatsMaxLifetimeClosed.WithLabelValues("mysql", "test_db")))
	tassert.Equal(t, float64(1), testutil.ToFloat64(globalStatsMaxIdleTimeClosed.WithLabelValues("mysql", "test_db")))
}

func TestMetricsCollector_SetDBStats_CounterReset(t *testing.T) {
	m, _ := newIsolatedCollector(t, "mysql", "test_db")

	var prevWaitCount, prevWaitDuration, prevMaxIdleClosed, prevMaxLifetimeClosed, prevMaxIdleTimeClosed float64

	stats1 := sql.DBStats{WaitCount: 100, WaitDuration: 5 * time.Second}
	m.setDBStats(stats1, &prevWaitCount, &prevWaitDuration, &prevMaxIdleClosed, &prevMaxLifetimeClosed, &prevMaxIdleTimeClosed)

	stats2 := sql.DBStats{WaitCount: 3, WaitDuration: 100 * time.Millisecond}
	m.setDBStats(stats2, &prevWaitCount, &prevWaitDuration, &prevMaxIdleClosed, &prevMaxLifetimeClosed, &prevMaxIdleTimeClosed)

	tassert.Equal(t, float64(100+3), testutil.ToFloat64(globalStatsWaitCount.WithLabelValues("mysql", "test_db")))
	tassert.InDelta(t, 5.1, testutil.ToFloat64(globalStatsWaitDuration.WithLabelValues("mysql", "test_db")), 0.001)
}

func TestMetricsCollector_MultipleDBs_PoolStatsIsolation(t *testing.T) {
	m1, _ := newIsolatedCollector(t, "mysql", "order_db")
	m2, _ := newIsolatedCollector(t, "pgsql", "user_db")

	var p1WaitCount, p1WaitDur, p1IdleClosed, p1LifetimeClosed, p1IdleTimeClosed float64
	var p2WaitCount, p2WaitDur, p2IdleClosed, p2LifetimeClosed, p2IdleTimeClosed float64

	m1.setDBStats(sql.DBStats{
		MaxOpenConnections: 50,
		OpenConnections:    20,
		InUse:              5,
		Idle:               15,
		WaitCount:          10,
	}, &p1WaitCount, &p1WaitDur, &p1IdleClosed, &p1LifetimeClosed, &p1IdleTimeClosed)

	m2.setDBStats(sql.DBStats{
		MaxOpenConnections: 30,
		OpenConnections:    8,
		InUse:              2,
		Idle:               6,
		WaitCount:          3,
	}, &p2WaitCount, &p2WaitDur, &p2IdleClosed, &p2LifetimeClosed, &p2IdleTimeClosed)

	tassert.Equal(t, float64(50), testutil.ToFloat64(globalStatsMaxOpenConnections.WithLabelValues("mysql", "order_db")))
	tassert.Equal(t, float64(20), testutil.ToFloat64(globalStatsOpenConnections.WithLabelValues("mysql", "order_db")))
	tassert.Equal(t, float64(5), testutil.ToFloat64(globalStatsInUse.WithLabelValues("mysql", "order_db")))
	tassert.Equal(t, float64(15), testutil.ToFloat64(globalStatsIdle.WithLabelValues("mysql", "order_db")))

	tassert.Equal(t, float64(30), testutil.ToFloat64(globalStatsMaxOpenConnections.WithLabelValues("pgsql", "user_db")))
	tassert.Equal(t, float64(8), testutil.ToFloat64(globalStatsOpenConnections.WithLabelValues("pgsql", "user_db")))
	tassert.Equal(t, float64(2), testutil.ToFloat64(globalStatsInUse.WithLabelValues("pgsql", "user_db")))
	tassert.Equal(t, float64(6), testutil.ToFloat64(globalStatsIdle.WithLabelValues("pgsql", "user_db")))
}

func TestAddCounterDelta(t *testing.T) {
	reg := prometheus.NewRegistry()
	cv := prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "test_counter", Help: "test"},
		[]string{"label"},
	)
	reg.MustRegister(cv)

	var prev float64
	addCounterDelta(cv, []string{"a"}, 10, &prev)
	tassert.Equal(t, float64(10), testutil.ToFloat64(cv.WithLabelValues("a")))
	tassert.Equal(t, float64(10), prev)

	addCounterDelta(cv, []string{"a"}, 15, &prev)
	tassert.Equal(t, float64(15), testutil.ToFloat64(cv.WithLabelValues("a")))
	tassert.Equal(t, float64(15), prev)

	addCounterDelta(cv, []string{"a"}, 3, &prev)
	tassert.Equal(t, float64(18), testutil.ToFloat64(cv.WithLabelValues("a")))
	tassert.Equal(t, float64(3), prev)
}

func TestIsMetricsQuery(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		expected bool
	}{
		{"information_schema select", "SELECT * FROM information_schema.tables", true},
		{"information_schema with leading space", "  SELECT * FROM information_schema.columns", true},
		{"normal select", "SELECT * FROM users WHERE id = 1", false},
		{"insert", "INSERT INTO users (name) VALUES ('test')", false},
		{"update", "UPDATE users SET name='test' WHERE id=1", false},
		{"delete", "DELETE FROM users WHERE id=1", false},
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tassert.Equal(t, tt.expected, isMetricsQuery(tt.query))
		})
	}
}

func TestNewMetricsCollector_Labels(t *testing.T) {
	config := &Config{Dialect: "mysql", DbName: "test_db"}
	m := newMetricsCollector(config)

	require.NotNil(t, m)
	tassert.Equal(t, "mysql", m.dialect)
	tassert.Equal(t, "test_db", m.dbName)
	tassert.Equal(t, []string{"mysql", "test_db"}, m.labels)
}

func TestEnsureMetricsRegistered_Idempotent(t *testing.T) {
	require.NotPanics(t, func() {
		ensureMetricsRegistered()
		ensureMetricsRegistered()
		ensureMetricsRegistered()
	}, "ensureMetricsRegistered should be safe to call multiple times")
}

func TestMaskConfigPassword(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"normal json", `{"user":"root","password":"secret123","host":"localhost"}`, `{"user":"root","password":"******","host":"localhost"}`},
		{"empty password", `{"user":"root","password":"","host":"localhost"}`, `{"user":"root","password":"******","host":"localhost"}`},
		{"no password field", `{"user":"root","host":"localhost"}`, `{"user":"root","host":"localhost"}`},
		{"password with spaces", `{"password": "my_secret"}`, `{"password":"******"}`},
		{"case insensitive Password", `{"User":"root","Password":"secret"}`, `{"User":"root","Password":"******"}`},
		{"secret field", `{"user":"root","secret":"my_secret"}`, `{"user":"root","secret":"******"}`},
		{"token field", `{"user":"root","token":"abc123"}`, `{"user":"root","token":"******"}`},
		{"pass field", `{"user":"root","pass":"mypass123"}`, `{"user":"root","pass":"******"}`},
		{"multiple sensitive fields", `{"password":"pw","secret":"sec","token":"tok"}`, `{"password":"******","secret":"******","token":"******"}`},
		{"passwordHint not masked", `{"passwordHint":"use uppercase"}`, `{"passwordHint":"use uppercase"}`},
		{"passphrase not masked", `{"passphrase":"my long passphrase"}`, `{"passphrase":"my long passphrase"}`},
		{"special chars in value", `{"password":"p@ss!w0rd#123"}`, `{"password":"******"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tassert.Equal(t, tt.expected, maskConfigPassword(tt.input))
		})
	}
}

func TestSplitIndex_ValidInput(t *testing.T) {
	canSplit, start, end := SplitIndex(1, 10, 25)
	tassert.True(t, canSplit)
	tassert.Equal(t, 0, start)
	tassert.Equal(t, 10, end)
}

func TestSplitIndex_SecondPage(t *testing.T) {
	canSplit, start, end := SplitIndex(2, 10, 25)
	tassert.True(t, canSplit)
	tassert.Equal(t, 10, start)
	tassert.Equal(t, 20, end)
}

func TestSplitIndex_LastPagePartial(t *testing.T) {
	canSplit, start, end := SplitIndex(3, 10, 25)
	tassert.True(t, canSplit)
	tassert.Equal(t, 20, start)
	tassert.Equal(t, 25, end)
}

func TestSplitIndex_PageExceedsTotal(t *testing.T) {
	canSplit, _, _ := SplitIndex(4, 10, 25)
	tassert.False(t, canSplit)
}

func TestSplitIndex_ZeroPageNumber(t *testing.T) {
	canSplit, _, _ := SplitIndex(0, 10, 25)
	tassert.False(t, canSplit)
}

func TestSplitIndex_NegativePageNumber(t *testing.T) {
	canSplit, _, _ := SplitIndex(-1, 10, 25)
	tassert.False(t, canSplit)
}

func TestSplitIndex_ZeroPageSize(t *testing.T) {
	canSplit, _, _ := SplitIndex(1, 0, 25)
	tassert.False(t, canSplit)
}

func TestSplitIndex_NegativePageSize(t *testing.T) {
	canSplit, _, _ := SplitIndex(1, -5, 25)
	tassert.False(t, canSplit)
}

func TestSoftDelete_NilID(t *testing.T) {
	config := Config{
		Dialect:            "sqlite",
		Url:                ":memory:",
		MaxOpen:            1,
		MaxIdle:            1,
		TransactionTimeout: 30 * time.Second,
	}
	client, err := NewDBClient(config)
	require.NoError(t, err)
	defer client.Close()

	_, err = client.SoftDelete(context.Background(), &ContainerRegistry{}, nil)
	tassert.Error(t, err)
	tassert.Contains(t, err.Error(), "id must not be nil")
}

func TestDelete_NilID(t *testing.T) {
	config := Config{
		Dialect:            "sqlite",
		Url:                ":memory:",
		MaxOpen:            1,
		MaxIdle:            1,
		TransactionTimeout: 30 * time.Second,
	}
	client, err := NewDBClient(config)
	require.NoError(t, err)
	defer client.Close()

	_, err = client.Delete(context.Background(), &ContainerRegistry{}, nil)
	tassert.Error(t, err)
	tassert.Contains(t, err.Error(), "id must not be nil")
}
