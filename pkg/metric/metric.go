package metric

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	httpRegistry = prometheus.NewRegistry()
)

func GetGather() prometheus.Gatherer {
	return httpRegistry
}

// GetRegistry returns the isolated prometheus registry used for HTTP metrics.
// Use this together with prometheus.DefaultGatherer when serving /metrics.
func GetRegistry() *prometheus.Registry {
	return httpRegistry
}
