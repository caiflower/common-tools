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
	"context"
	"errors"

	"github.com/caiflower/common-tools/pkg/crontab"
	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/go-redis/redis/v8"
)

func (c *Cluster) redisClusterStartUp() {
	// 选主
	go c.redisFighting()
	// 获取主节点
	go c.redisSyncLeader()
}

func (c *Cluster) redisFighting() {
	c.createEvent(eventNameElectionStart, "")
	c.sate = fighting
	defer func() {
		c.createEvent(eventNameElectionFinish, c.GetLeaderName())
	}()

	key := c.config.RedisDiscovery.DataPath + "/Election"

	logger.Debug("[cluster-redis] redisFighting")
	err := c.Redis.SetNXPeriod(key, c.GetMyName(), c.config.RedisDiscovery.ElectionPeriod)
	if err != nil {
		c.logger.Error("[cluster-redis] fighting failed. Error: %v", err)
	}
}

func (c *Cluster) redisSyncLeader() {
	key := c.config.RedisDiscovery.DataPath + "/Election"

	fn := func() {
		leaderName, err := c.Redis.GetString(key)
		if err != nil {
			if errors.Is(err, redis.Nil) {
				c.releaseLeader()
				// 重新开始选举
				go c.redisFighting()
			} else {
				c.logger.Error("[cluster-redis] get leaderName failed. Error: %v", err)
			}
		} else {
			if c.GetLeaderName() != leaderName {
				node := c.GetNodeByName(leaderName)
				if node != nil {
					return
				}

				if c.signLeader(node, 0) && c.GetLeaderName() == c.GetMyName() {
					// 看门狗续租
					go c.redisWatchDog()
				}
			}
		}
	}

	job := crontab.NewRegularJob("redisSyncLeader", fn, crontab.WithInterval(c.config.RedisDiscovery.SyncLeaderInterval), crontab.WithImmediately())
	job.Run()

	select {
	case <-c.ctx.Done():
		c.logger.Info("[cluster-redis] stop sync leader")
		job.Stop()
	}
}

func (c *Cluster) redisWatchDog() {
	key := c.config.RedisDiscovery.DataPath + "/Election"
	ctx, cancel := context.WithCancel(c.ctx)

	fn := func() {
		leaderName, err := c.Redis.GetString(key)
		if err != nil {
			logger.Error("[cluster-redis] get lease failed. Error: %v", err)
			c.releaseLeader()
			cancel()
			return
		}

		if leaderName != c.GetLeaderName() {
			c.releaseLeader()
			cancel()
			return
		}

		err = c.Redis.SetPeriod(key, leaderName, c.config.RedisDiscovery.ElectionPeriod)
		if err != nil {
			logger.Error("[cluster-redis] set lease failed. Error: %v", err)
		}
	}

	job := crontab.NewRegularJob("redisWatchDog", fn, crontab.WithInterval(c.config.RedisDiscovery.ElectionInterval), crontab.WithImmediately())
	job.Run()

	select {
	case <-ctx.Done():
		c.logger.Info("[cluster-redis] stop redis watch dog")
		job.Stop()
	}
}
