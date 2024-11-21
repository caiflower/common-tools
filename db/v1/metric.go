package v1

import (
	"context"
	"time"

	"github.com/caiflower/common-tools/global/env"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/uptrace/bun"
)

var dbIdleTotal *prometheus.GaugeVec
var dbInUseTotal *prometheus.GaugeVec
var dbWaitDuration *prometheus.CounterVec

func startMetric(ctx context.Context, db *bun.DB, config *Config) {
	register()

	go func() {
		ticker := time.NewTicker(10 * time.Second)
		var preWaitDuration float64

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				stats := db.Stats()

				dbIdleTotal.WithLabelValues(config.Dialect, config.Url, config.DbName).Set(float64(stats.Idle))
				dbInUseTotal.WithLabelValues(config.Dialect, config.Url, config.DbName).Set(float64(stats.InUse))
				current := stats.WaitDuration.Seconds()
				dbWaitDuration.WithLabelValues(config.Dialect, config.Url, config.DbName).Add(current - preWaitDuration)
				preWaitDuration = current
			}
		}
	}()
}

func register() {
	constLabels := prometheus.Labels{"ip": env.GetLocalHostIP()}
	dbIdleTotal = prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "db_idle", Help: "The number of idle connections.", ConstLabels: constLabels}, []string{"type", "url", "database"})
	dbInUseTotal = prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "db_in_use", Help: "The number of connections currently in use.", ConstLabels: constLabels}, []string{"type", "url", "database"})
	dbWaitDuration = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "db_wait_total_time", Help: "The total time blocked waiting for a new connection.", ConstLabels: constLabels}, []string{"type", "url", "database"})

	prometheus.MustRegister(dbIdleTotal)
}
