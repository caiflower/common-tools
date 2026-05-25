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

package cluster

import (
	"sync"

	"github.com/caiflower/common-tools/global/env"
	"github.com/prometheus/client_golang/prometheus"
)

var clusterMetric *metric
var once sync.Once

type metric struct {
	memberIsLeader    prometheus.Gauge
	connectedMembers  prometheus.Gauge
	configuredMembers prometheus.Gauge
}

func initMetrics() {
	once.Do(func() {
		constLabels := prometheus.Labels{"ip": env.GetLocalHostIP()}
		if env.GetNamespace() != "" {
			constLabels["namespace"] = env.GetNamespace()
			constLabels["app"] = env.GetApp()
		}
		if node := env.GetLocalDNS(); node != "" {
			constLabels["node"] = node
		}

		clusterMetric = &metric{
			memberIsLeader: prometheus.NewGauge(prometheus.GaugeOpts{
				Name:        "cluster_member_is_leader",
				Help:        "1 if this member is currently the cluster leader, 0 otherwise.",
				ConstLabels: constLabels,
			}),
			connectedMembers: prometheus.NewGauge(prometheus.GaugeOpts{
				Name:        "cluster_connected_members",
				Help:        "Number of cluster members currently reachable via heartbeat.",
				ConstLabels: constLabels,
			}),
			configuredMembers: prometheus.NewGauge(prometheus.GaugeOpts{
				Name:        "cluster_configured_members",
				Help:        "Total number of members in the cluster configuration.",
				ConstLabels: constLabels,
			}),
		}

		_ = prometheus.Register(clusterMetric.memberIsLeader)
		_ = prometheus.Register(clusterMetric.connectedMembers)
		_ = prometheus.Register(clusterMetric.configuredMembers)
	})
}

func (c *Cluster) updateMetrics(isLeader bool) {
	clusterMetric.memberIsLeader.Set(boolToFloat(isLeader))
	clusterMetric.connectedMembers.Set(float64(c.GetAliveNodeCount()))
	clusterMetric.configuredMembers.Set(float64(c.GetAllNodeCount()))
}

func boolToFloat(b bool) float64 {
	if b {
		return 1
	}
	return 0
}
