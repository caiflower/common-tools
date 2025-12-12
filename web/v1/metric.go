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

package webv1

import (
	"github.com/caiflower/common-tools/global/env"
	"github.com/prometheus/client_golang/prometheus"
)

type HttpMetric struct {
	httpRequestTotal     *prometheus.CounterVec
	httpRequestTimeTotal *prometheus.CounterVec
	costHistogram        prometheus.Histogram
}

func NewHttpMetric() *HttpMetric {
	constLabels := prometheus.Labels{"ip": env.GetLocalHostIP()}

	buckets := []float64{20, 50, 100, 200, 500, 1000, 2000, 5000, 10000}
	metric := &HttpMetric{
		httpRequestTimeTotal: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "http_request_time_total", Help: "http_request_time_total counter", ConstLabels: constLabels}, []string{"web", "code", "method", "path"}),
		httpRequestTotal:     prometheus.NewCounterVec(prometheus.CounterOpts{Name: "http_request_total", Help: "http_request_total counter", ConstLabels: constLabels}, []string{"web", "code", "method", "path"}),
		costHistogram:        prometheus.NewHistogram(prometheus.HistogramOpts{Name: "http_request_histogram", Help: "http_request_histogram", Buckets: buckets, ConstLabels: constLabels}),
	}

	prometheus.Register(metric.httpRequestTotal)
	prometheus.Register(metric.httpRequestTimeTotal)
	prometheus.Register(metric.costHistogram)

	return metric
}

func (m *HttpMetric) saveMetric(web string, code string, method, path string, cost int64) {
	m.httpRequestTotal.WithLabelValues(web, code, method, path).Inc()
	m.httpRequestTimeTotal.WithLabelValues(web, code, method, path).Add(float64(cost))
	m.costHistogram.Observe(float64(cost))
}
