package metric

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

var (
	httpRegistry = prometheus.NewRegistry()
)

func init() {
	httpRegistry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)
}

func GetGather() prometheus.Gatherer {
	return httpRegistry
}

// GetRegistry returns the isolated prometheus registry used for HTTP metrics.
func GetRegistry() *prometheus.Registry {
	return httpRegistry
}
