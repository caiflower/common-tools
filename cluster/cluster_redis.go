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
	"encoding/json"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/caiflower/common-tools/pkg/crontab"
	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/go-redis/redis/v8"
)

// redisKeyNodes 返回节点注册 key 的前缀
func (c *Cluster) redisKeyNodes() string {
	return c.config.RedisDiscovery.DataPath + ":Nodes"
}

// redisKeyNode 返回指定节点的注册 key
func (c *Cluster) redisKeyNode(nodeName string) string {
	return c.redisKeyNodes() + ":" + nodeName
}

// redisKeyNodesPattern 返回节点扫描的 pattern
func (c *Cluster) redisKeyNodesPattern() string {
	return c.redisKeyNodes() + ":*"
}

// redisKeyElection 返回选举 key
func (c *Cluster) redisKeyElection() string {
	return c.config.RedisDiscovery.DataPath + ":Election"
}

func (c *Cluster) redisClusterStartUp() {
	// 注册节点到 Redis
	go c.redisRegisterNode()
	// 获取主节点
	go c.redisSyncLeader()
	// 同步节点信息
	go c.redisSyncNodes()
}

// NodeInfo 节点信息结构
type NodeInfo struct {
	Name      string `json:"name"`
	Address   string `json:"address"`
	Timestamp int64  `json:"timestamp"`
}

// redisRegisterNode 注册节点到 Redis 并定期续约
func (c *Cluster) redisRegisterNode() {
	key := c.redisKeyNode(c.GetMyName())

	fn := func() {
		if err := c.doRegisterNode(key); err != nil {
			c.logger.Error("[cluster-redis] renew node failed. Error: %v", err)
		}
	}

	job := crontab.NewRegularJob("redisNodeHeartbeat", fn, crontab.WithInterval(c.config.RedisDiscovery.NodeHeartbeatPeriod), crontab.WithImmediately())
	job.Run()

	<-c.ctx.Done()
	c.logger.Info("[cluster-redis] stop node heartbeat")
	job.Stop()

	// 退出时删除节点注册信息
	// 注意：此时 c.ctx 已被 cancel，使用 context.TODO() 确保删除操作能执行完成，避免注册信息残留
	if err := c.Redis.Del(context.TODO(), key); err != nil {
		c.logger.Warn("[cluster-redis] delete node registration failed. Error: %v", err)
	}
}

// doRegisterNode 执行节点注册/续约
func (c *Cluster) doRegisterNode(key string) error {
	nodeInfo := NodeInfo{
		Name:      c.GetMyName(),
		Address:   c.GetMyAddress(),
		Timestamp: time.Now().Unix(),
	}

	data, err := json.Marshal(nodeInfo)
	if err != nil {
		return fmt.Errorf("marshal node info failed: %w", err)
	}

	// 使用 SetEXPeriod 实现注册和续约（带过期时间）
	if err = c.Redis.SetEXPeriod(c.ctx, key, string(data), c.config.RedisDiscovery.NodeRegisterTTL); err != nil {
		return fmt.Errorf("set node info failed: %w", err)
	}

	c.logger.Debug("[cluster-redis] node registered: %s", c.GetMyName())
	return nil
}

// redisSyncNodes 从 Redis 同步节点信息
func (c *Cluster) redisSyncNodes() {
	pattern := c.redisKeyNodesPattern()

	fn := func() {
		if err := c.doSyncNodes(pattern); err != nil {
			c.logger.Error("[cluster-redis] sync nodes failed. Error: %v", err)
		}
	}

	job := crontab.NewRegularJob("redisSyncNodes", fn, crontab.WithInterval(c.config.RedisDiscovery.NodeSyncInterval), crontab.WithImmediately())
	job.Run()

	<-c.ctx.Done()
	c.logger.Info("[cluster-redis] stop sync nodes")
	job.Stop()
}

// doSyncNodes 执行节点同步
func (c *Cluster) doSyncNodes(pattern string) error {
	// 使用 GetRedis() 获取原生的 redis.Cmdable 来执行 SCAN
	redisCmd := c.Redis.GetRedis()

	// SCAN 是游标迭代器，需循环直到 cursor 归零才能获取所有 key
	var keys []string
	var cursor uint64
	for {
		var batch []string
		var err error
		batch, cursor, err = redisCmd.Scan(c.ctx, cursor, pattern, 100).Result()
		if err != nil {
			return fmt.Errorf("scan nodes failed: %w", err)
		}
		keys = append(keys, batch...)
		if cursor == 0 {
			break
		}
	}

	if len(keys) == 0 {
		c.logger.Debug("[cluster-redis] no nodes found in Redis")
		return nil
	}

	// 构建新的节点映射
	// 注意：SCAN 返回的 key 已经包含 KeyPrefix，因此必须用原始 Redis 客户端直接 GET，
	// 不能使用 c.Redis.GetString（会通过 GetKey 再次添加 KeyPrefix，导致双重前缀）
	newNodes := make(map[string]*Node)
	for _, key := range keys {
		data, err := redisCmd.Get(c.ctx, key).Result()
		if err != nil {
			if errors.Is(err, redis.Nil) {
				continue
			}
			c.logger.Warn("[cluster-redis] get node info failed. Key: %s, Error: %v", key, err)
			continue
		}

		var nodeInfo NodeInfo
		if err := json.Unmarshal([]byte(data), &nodeInfo); err != nil {
			c.logger.Warn("[cluster-redis] unmarshal node info failed. Data: %s, Error: %v", data, err)
			continue
		}

		// 跳过自身节点，避免重复创建
		if nodeInfo.Name == c.GetMyName() {
			continue
		}

		node := newNode(nodeInfo.Address, nodeInfo.Name, c.config.Timeout.Seconds()/3)
		newNodes[nodeInfo.Name] = node
	}

	// 更新节点列表
	c.updateNodesFromRedis(newNodes)

	c.logger.Debug("[cluster-redis] synced %d nodes from Redis", len(newNodes))
	return nil
}

// updateNodesFromRedis 更新节点列表
func (c *Cluster) updateNodesFromRedis(newNodes map[string]*Node) {
	// 添加新节点
	for name, node := range newNodes {
		if _, exists := c.allNode.Load(name); !exists {
			c.allNode.Store(name, node)
			c.logger.Info("[cluster-redis] new node discovered: %s (%s)", name, node.address)
		}
	}

	// 移除不存在的节点（除了自己）
	c.allNode.Range(func(key, value interface{}) bool {
		name := key.(string)
		if name != c.GetMyName() {
			if _, exists := newNodes[name]; !exists {
				c.allNode.Delete(name)
				c.aliveNodes.Delete(name)
				c.logger.Info("[cluster-redis] node removed: %s", name)
			}
		}
		return true
	})

	// 建立节点连接
	if c.needReconnect() {
		c.reconnect()
	}
}

func (c *Cluster) redisFighting() {
	// 防止重复选举（类似 modeCluster 的 fightingState）
	if !atomic.CompareAndSwapUint32(&c.redisFightingState, 0, 1) {
		c.logger.Debug("[cluster-redis] redisFighting already running, skip")
		return
	}
	defer atomic.StoreUint32(&c.redisFightingState, 0)

	c.createEvent(eventNameElectionStart, "")
	defer func() {
		c.createEvent(eventNameElectionFinish, c.GetLeaderName())
	}()

	key := c.redisKeyElection()

	logger.Debug("[cluster-redis] redisFighting")
	if err := c.redisFightingWithRetry(key, 0); err != nil {
		c.logger.Error("[cluster-redis] fighting failed after retries. Error: %v", err)
	}
}

// redisFightingWithRetry 带重试的选主逻辑
func (c *Cluster) redisFightingWithRetry(key string, retryCount int) error {
	const maxRetries = 3

	err := c.Redis.SetNXPeriod(c.ctx, key, c.GetMyName(), c.config.RedisDiscovery.ElectionPeriod)
	if err != nil {
		if retryCount < maxRetries {
			backoff := time.Duration(retryCount+1) * time.Second
			c.logger.Warn("[cluster-redis] fighting failed, retry %d/%d after %v. Error: %v", retryCount+1, maxRetries, backoff, err)
			time.Sleep(backoff)
			return c.redisFightingWithRetry(key, retryCount+1)
		}
		return fmt.Errorf("fighting failed after %d retries: %w", maxRetries, err)
	}
	return nil
}

func (c *Cluster) redisSyncLeader() {
	key := c.redisKeyElection()

	fn := func() {
		leaderName, err := c.Redis.GetString(c.ctx, key)
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
				if node == nil {
					// leader 节点未在本地节点列表中，触发一次节点同步尝试发现该节点
					c.logger.Warn("[cluster-redis] leader node %s not found in local, triggering node sync", leaderName)
					if syncErr := c.doSyncNodes(c.redisKeyNodesPattern()); syncErr != nil {
						c.logger.Error("[cluster-redis] sync nodes for leader discovery failed. Error: %v", syncErr)
						return
					}
					node = c.GetNodeByName(leaderName)
				}
				if node == nil {
					c.logger.Warn("[cluster-redis] leader node %s still not found after sync", leaderName)
					return
				}

				if c.signLeader(node, 0) && c.IsLeader() {
					// 看门狗续租
					go c.redisWatchDog()
				}
			}
		}
	}

	job := crontab.NewRegularJob("redisSyncLeader", fn, crontab.WithInterval(c.config.RedisDiscovery.SyncLeaderInterval), crontab.WithImmediately())
	job.Run()

	<-c.ctx.Done()
	c.logger.Info("[cluster-redis] stop sync leader")
	job.Stop()
}

func (c *Cluster) redisWatchDog() {
	// 防止重复启动 WatchDog
	if !atomic.CompareAndSwapUint32(&c.redisWatchDogState, 0, 1) {
		c.logger.Debug("[cluster-redis] redisWatchDog already running, skip")
		return
	}
	defer atomic.StoreUint32(&c.redisWatchDogState, 0)

	key := c.redisKeyElection()
	ctx, cancel := context.WithCancel(c.ctx)
	defer cancel() // 确保退出时取消 context

	fn := func() {
		leaderName, err := c.Redis.GetString(ctx, key)
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

		err = c.Redis.SetPeriod(ctx, key, leaderName, c.config.RedisDiscovery.ElectionPeriod)
		if err != nil {
			logger.Error("[cluster-redis] set lease failed. Error: %v", err)
		}
	}

	job := crontab.NewRegularJob("redisWatchDog", fn, crontab.WithInterval(c.config.RedisDiscovery.ElectionInterval), crontab.WithImmediately())
	job.Run()

	<-ctx.Done()
	c.logger.Info("[cluster-redis] stop redis watch dog")
	job.Stop()
}
