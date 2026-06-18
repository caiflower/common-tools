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

package v2

import (
	"context"
	"net"
	"strings"
	"time"

	xredis "github.com/caiflower/common-tools/redis"
	"github.com/redis/go-redis/v9"
)

func init() {
	xredis.InitMetrics()
}

// MetricsHook implements redis.Hook (v9) to collect Prometheus metrics per command.
type MetricsHook struct {
	addr string
}

var _ redis.Hook = (*MetricsHook)(nil)

func newMetricsHook(config *Config) *MetricsHook {
	addr := "cluster"
	if config.Mode != xredis.ClusterMode {
		addr = config.Addrs[0]
	}
	return &MetricsHook{addr: addr}
}

// DialHook is a no-op pass-through required by the v9 Hook interface.
func (h *MetricsHook) DialHook(next redis.DialHook) redis.DialHook {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		return next(ctx, network, addr)
	}
}

// ProcessHook wraps the next ProcessHook to measure elapsed time and record metrics.
func (h *MetricsHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		start := time.Now()
		err := next(ctx, cmd)
		elapsed := time.Since(start)

		cmdName := strings.ToLower(cmd.FullName())
		status := statusLabel(cmd.Err())

		xredis.RecordCommand(h.addr, cmdName, status, elapsed)
		return err
	}
}

// ProcessPipelineHook wraps the next ProcessPipelineHook to measure elapsed time and record pipeline metrics.
func (h *MetricsHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []redis.Cmder) error {
		start := time.Now()
		err := next(ctx, cmds)
		elapsed := time.Since(start)

		hasErr := false
		for _, cmd := range cmds {
			if cmd.Err() != nil && cmd.Err() != redis.Nil {
				hasErr = true
				break
			}
		}

		xredis.RecordPipeline(h.addr, len(cmds), hasErr, elapsed)
		return err
	}
}

// statusLabel returns "ok" for nil or redis.Nil errors, "error" otherwise.
func statusLabel(err error) string {
	if err == nil || err == redis.Nil {
		return "ok"
	}
	return "error"
}

// startPoolMetrics launches a background goroutine that periodically collects
// connection pool statistics and updates the corresponding Prometheus gauges/counters.
func startPoolMetrics(ctx context.Context, c *redisClient) {
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		var prevStale uint32

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				collectPoolStats(c, &prevStale)
			}
		}
	}()
}

func collectPoolStats(c *redisClient, prevStale *uint32) {
	switch r := c.redis.(type) {
	case *redis.ClusterClient:
		var total xredis.PoolStats
		if err := r.ForEachShard(context.Background(), func(ctx context.Context, shard *redis.Client) error {
			s := shard.PoolStats()
			total.IdleConns += s.IdleConns
			total.TotalConns += s.TotalConns
			total.StaleConns += s.StaleConns
			return nil
		}); err != nil {
			// Best-effort: skip this collection cycle on error.
			return
		}
		xredis.UpdatePoolStats("cluster", total, prevStale)
	case *redis.Client:
		s := r.PoolStats()
		xredis.UpdatePoolStats(c.config.Addrs[0], xredis.PoolStats{
			IdleConns:  s.IdleConns,
			TotalConns: s.TotalConns,
			StaleConns: s.StaleConns,
		}, prevStale)
	}
}
