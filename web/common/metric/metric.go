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

// var buckets = []float64{20, 50, 100, 200, 500, 700, 1000, 1200, 1500, 1800, 2000, 5000, 10000}
var secondBuckets = []float64{0.02, 0.05, 0.1, 0.2, 0.5, 0.7, 1, 1.2, 1.5, 1.8, 2, 5, 10}

type HttpMetric struct {
	httpRequestTotal *prometheus.CounterVec
	//costHistograms   sync.Map
	secondsHistogram sync.Map
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

func SaveMetric(web string, code string, method, path string, millSeconds int64, seconds float64) {
	//_costHistogram, ok := metric.costHistograms.Load(path)
	_costSecondHistogram, ok := metric.secondsHistogram.Load(path)

	if !ok {
		metric.lock.Lock()
		//_costHistogram, ok = metric.costHistograms.Load(path)
		_costSecondHistogram, ok = metric.secondsHistogram.Load(path)

		if !ok {
			constLabels := prometheus.Labels{"ip": env.GetLocalHostIP()}
			if env.GetNamespace() != "" {
				constLabels["namespace"] = env.GetNamespace()
				constLabels["app"] = env.GetApp()
			}
			constLabels["handler"] = path

			//_costHistogram = prometheus.NewHistogram(prometheus.HistogramOpts{Name: "http_request_duration", Help: "http_request_duration_milliseconds_bucket", Buckets: buckets, ConstLabels: constLabels})
			_costSecondHistogram = prometheus.NewHistogram(prometheus.HistogramOpts{Name: "http_request_duration_seconds", Help: "http_request_duration_seconds_bucket", Buckets: secondBuckets, ConstLabels: constLabels})

			//prometheus.Register(_costHistogram.(prometheus.Histogram))
			prometheus.Register(_costSecondHistogram.(prometheus.Histogram))

			//metric.costHistograms.Store(path, _costHistogram)
			metric.secondsHistogram.Store(path, _costSecondHistogram)

			metric.lock.Unlock()
		}
	}

	//costHistogram := _costHistogram.(prometheus.Histogram)
	costSecondHistogram := _costSecondHistogram.(prometheus.Histogram)

	metric.httpRequestTotal.WithLabelValues(web, code, method, path).Inc()
	//costHistogram.Observe(float64(millSeconds))
	costSecondHistogram.Observe(seconds)
}
