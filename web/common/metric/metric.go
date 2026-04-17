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

package metric

import (
	"sync"

	"github.com/caiflower/common-tools/global/env"
	"github.com/prometheus/client_golang/prometheus"
)

var metric *HttpMetric

var buckets = []float64{20, 50, 100, 200, 500, 1000, 2000, 5000, 10000}

type HttpMetric struct {
	httpRequestTotal *prometheus.CounterVec
	costHistograms   sync.Map
	lock             sync.Mutex
}

func init() {
	constLabels := prometheus.Labels{"ip": env.GetLocalHostIP()}
	if env.GetNamespace() != "" {
		constLabels["namespace"] = env.GetNamespace()
		constLabels["app"] = env.GetApp()
	}

	metric = &HttpMetric{
		httpRequestTotal: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "http_request_total", Help: "http_requests_total counter", ConstLabels: constLabels}, []string{"web", "status", "method", "handler"}),
	}

	_ = prometheus.Register(metric.httpRequestTotal)
}

func SaveMetric(web string, code string, method, path string, cost int64) {
	_costHistogram, ok := metric.costHistograms.Load(path)

	if !ok {
		metric.lock.Lock()
		_costHistogram, ok = metric.costHistograms.Load(path)
		if !ok {
			constLabels := prometheus.Labels{"ip": env.GetLocalHostIP()}
			if env.GetNamespace() != "" {
				constLabels["namespace"] = env.GetNamespace()
				constLabels["app"] = env.GetApp()
			}
			constLabels["handler"] = path

			_costHistogram = prometheus.NewHistogram(prometheus.HistogramOpts{Name: "http_request_duration_seconds_bucket", Help: "http_request_duration_seconds_bucket", Buckets: buckets, ConstLabels: constLabels})
			prometheus.Register(_costHistogram.(prometheus.Histogram))
			metric.costHistograms.Store(path, _costHistogram)
			metric.lock.Unlock()
		}
	}

	costHistogram := _costHistogram.(prometheus.Histogram)

	metric.httpRequestTotal.WithLabelValues(web, code, method, path).Inc()
	costHistogram.Observe(float64(cost))
}
