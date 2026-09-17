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
	"fmt"
	"sync/atomic"
	"time"

	"github.com/caiflower/common-tools/pkg/crontab"
	"github.com/caiflower/common-tools/pkg/json"
	"github.com/caiflower/common-tools/pkg/logger"
	redisv2 "github.com/caiflower/common-tools/redis/v2"
	"github.com/redis/go-redis/v9"
)

// redisKeyNodes 返回节点索引 set key
func (c *Cluster) redisKeyNodes() string {
	return "{" + c.config.RedisDiscovery.DataPath + "}:Nodes"
}

// redisKeyNode 返回指定节点的注册 key
func (c *Cluster) redisKeyNode(nodeName string) string {
	return c.redisKeyNodes() + ":" + nodeName
}

// redisKeyElection 返回选举 key
func (c *Cluster) redisKeyElection() string {
	return "{" + c.config.RedisDiscovery.DataPath + "}:Election"
}

const redisRenewLeaderLeaseScript = `
if redis.call("GET", KEYS[1]) == ARGV[1] then
	return redis.call("PEXPIRE", KEYS[1], ARGV[2])
end
return 0
`

const redisOpRenewLeaderLease = "renewLeaderLease"

const redisRemoveMissingNodeScript = `
if redis.call("EXISTS", KEYS[1]) == 0 then
	return redis.call("SREM", KEYS[2], ARGV[1])
end
return 0
`

const redisOpRemoveMissingNode = "removeMissingNode"

func (c *Cluster) redisClusterStartUp() {
	if err := c.initRedisScriptManager(c.ctx); err != nil {
		c.logger.Error("[cluster-redis] init script manager failed: %v", err)
		return
	}

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
			c.logger.Error("[cluster-redis] renew node failed: %v", err)
		}
	}

	job := crontab.NewRegularJob("redisNodeHeartbeat", fn, crontab.WithInterval(c.config.RedisDiscovery.NodeHeartbeatPeriod), crontab.WithImmediately())
	job.Run()

	<-c.ctx.Done()
	c.logger.Info("[cluster-redis] node heartbeat stopped")
	job.Stop()

	// 退出时删除节点注册信息
	// 注意：此时 c.ctx 已被 cancel，使用 context.TODO() 确保删除操作能执行完成，避免注册信息残留
	if err := c.Redis.Cmd().SRem(context.TODO(), c.redisKeyNodes(), c.GetMyName()).Err(); err != nil {
		c.logger.Warn("[cluster-redis] remove node index failed: %v", err)
	}
	if err := c.Redis.Cmd().Del(context.TODO(), key).Err(); err != nil {
		c.logger.Warn("[cluster-redis] delete node registration failed: %v", err)
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

	// Use SET with TTL for registration and renewal
	if err := c.Redis.Cmd().Set(c.ctx, key, string(data), c.config.RedisDiscovery.NodeRegisterTTL).Err(); err != nil {
		return fmt.Errorf("set node info failed: %w", err)
	}

	if err := c.Redis.Cmd().SAdd(c.ctx, c.redisKeyNodes(), nodeInfo.Name).Err(); err != nil {
		return fmt.Errorf("add node index failed: %w", err)
	}

	c.logger.Debug("[cluster-redis] node registered: %s", c.GetMyName())
	return nil
}

// redisSyncNodes 从 Redis 同步节点信息
func (c *Cluster) redisSyncNodes() {
	fn := func() {
		if err := c.doSyncNodes(); err != nil {
			c.logger.Error("[cluster-redis] sync nodes failed: %v", err)
		}
	}

	job := crontab.NewRegularJob("redisSyncNodes", fn, crontab.WithInterval(c.config.RedisDiscovery.NodeSyncInterval), crontab.WithImmediately())
	job.Run()

	<-c.ctx.Done()
	c.logger.Info("[cluster-redis] node sync stopped")
	job.Stop()
}

// doSyncNodes 执行节点同步
func (c *Cluster) doSyncNodes() error {
	indexKey := c.redisKeyNodes()
	nodeNames, err := c.Redis.Cmd().SMembers(c.ctx, indexKey).Result()
	if err != nil {
		return fmt.Errorf("get node index failed: %w", err)
	}

	newNodes := make(map[string]*Node)
	for _, nodeName := range nodeNames {
		key := c.redisKeyNode(nodeName)
		data, err := c.Redis.Cmd().Get(c.ctx, key).Result()
		if err != nil {
			if errors.Is(err, redis.Nil) {
				if _, removeErr := c.redisRemoveMissingNode(c.ctx, indexKey, key, nodeName); removeErr != nil {
					c.logger.Warn("[cluster-redis] remove missing node from index failed, node=%s: %v", nodeName, removeErr)
				}
				continue
			}
			return fmt.Errorf("get node info failed, node=%s: %w", nodeName, err)
		}

		var nodeInfo NodeInfo
		if err := json.Unmarshal([]byte(data), &nodeInfo); err != nil {
			return fmt.Errorf("unmarshal node info failed, node=%s: %w", nodeName, err)
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
				c.logger.Info("[cluster-redis] node removed: %s (%s)", name, value.(*Node).address)
			}
		}
		return true
	})

	// 建立节点连接
	c.reconnect()
}

func (c *Cluster) redisFighting() {
	// 防止重复选举（类似 modeCluster 的 fightingState）
	if !atomic.CompareAndSwapUint32(&c.redisFightingState, 0, 1) {
		c.logger.Debug("[cluster-redis] election already running, skip")
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
		c.logger.Error("[cluster-redis] election failed after retries: %v", err)
	}
}

// redisFightingWithRetry 带重试的选主逻辑
func (c *Cluster) redisFightingWithRetry(key string, retryCount int) error {
	const maxRetries = 3

	ok, err := c.Redis.Cmd().SetNX(c.ctx, key, c.GetMyName(), c.config.RedisDiscovery.ElectionPeriod).Result()
	if err != nil {
		if retryCount < maxRetries {
			backoff := time.Duration(retryCount+1) * time.Second
			c.logger.Warn("[cluster-redis] election failed, retry %d/%d after %v: %v", retryCount+1, maxRetries, backoff, err)
			time.Sleep(backoff)
			return c.redisFightingWithRetry(key, retryCount+1)
		}
		return fmt.Errorf("fighting failed after %d retries: %w", maxRetries, err)
	}
	if !ok {
		return fmt.Errorf("fighting failed: key already held by another node")
	}
	return nil
}

func (c *Cluster) redisSyncLeader() {
	key := c.redisKeyElection()

	fn := func() {
		leaderName, err := c.Redis.Cmd().Get(c.ctx, key).Result()
		if err != nil {
			if errors.Is(err, redis.Nil) {
				c.releaseLeader()
				// 重新开始选举
				go c.redisFighting()
			} else {
				c.logger.Error("[cluster-redis] get leader name failed: %v", err)
			}
		} else {
			if c.GetLeaderName() != leaderName {
				node := c.GetNodeByName(leaderName)
				if node == nil {
					// leader 节点未在本地节点列表中，触发一次节点同步尝试发现该节点
					c.logger.Warn("[cluster-redis] leader %s not found locally, triggering node sync", leaderName)
					if syncErr := c.doSyncNodes(); syncErr != nil {
						c.logger.Error("[cluster-redis] sync nodes for leader discovery failed: %v", syncErr)
						return
					}
					node = c.GetNodeByName(leaderName)
				}
				if node == nil {
					c.logger.Warn("[cluster-redis] leader %s still not found after sync", leaderName)
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
	c.logger.Info("[cluster-redis] leader sync stopped")
	job.Stop()
}

func (c *Cluster) redisWatchDog() {
	// 防止重复启动 WatchDog
	if !atomic.CompareAndSwapUint32(&c.redisWatchDogState, 0, 1) {
		c.logger.Debug("[cluster-redis] watchdog already running, skip")
		return
	}
	defer atomic.StoreUint32(&c.redisWatchDogState, 0)

	key := c.redisKeyElection()
	ctx, cancel := context.WithCancel(c.ctx)
	defer cancel() // 确保退出时取消 context

	fn := func() {
		renewed, err := c.redisRenewLeaderLease(ctx, key, c.GetLeaderName())
		if err != nil {
			c.logger.Error("[cluster-redis] renew lease failed: %v", err)
			c.releaseLeader()
			cancel()
			return
		}

		if !renewed {
			c.logger.Warn("[cluster-redis] leader lease lost")
			c.releaseLeader()
			cancel()
		}
	}

	job := crontab.NewRegularJob("redisWatchDog", fn, crontab.WithInterval(c.config.RedisDiscovery.ElectionInterval), crontab.WithImmediately())
	job.Run()

	<-ctx.Done()
	c.logger.Info("[cluster-redis] watchdog stopped")
	job.Stop()
}

func (c *Cluster) redisRenewLeaderLease(ctx context.Context, key, leaderName string) (bool, error) {
	if leaderName == "" {
		return false, nil
	}

	ttlMillis := c.config.RedisDiscovery.ElectionPeriod.Milliseconds()
	if ttlMillis <= 0 {
		return false, fmt.Errorf("invalid election period: %s", c.config.RedisDiscovery.ElectionPeriod)
	}

	if c.redisScriptManager == nil {
		return false, errors.New("redis script manager is not initialized")
	}

	result, err := c.redisScriptManager.EvalShaInt(
		ctx,
		redisOpRenewLeaderLease,
		[]string{key},
		leaderName,
		ttlMillis,
	)
	if err != nil {
		return false, fmt.Errorf("evaluate leader lease renewal failed: %w", err)
	}

	return result == 1, nil
}

func (c *Cluster) redisRemoveMissingNode(ctx context.Context, indexKey, nodeKey, nodeName string) (bool, error) {
	if c.redisScriptManager == nil {
		return false, errors.New("redis script manager is not initialized")
	}

	result, err := c.redisScriptManager.EvalShaInt(
		ctx,
		redisOpRemoveMissingNode,
		[]string{nodeKey, indexKey},
		nodeName,
	)
	if err != nil {
		return false, fmt.Errorf("evaluate missing node removal failed: %w", err)
	}

	return result == 1, nil
}

func (c *Cluster) initRedisScriptManager(ctx context.Context) error {
	scriptManager := redisv2.NewScriptManager(c.Redis.GetRedis())
	if err := scriptManager.Register(redisOpRenewLeaderLease, redisRenewLeaderLeaseScript); err != nil {
		return fmt.Errorf("register leader lease script failed: %w", err)
	}
	if err := scriptManager.Register(redisOpRemoveMissingNode, redisRemoveMissingNodeScript); err != nil {
		return fmt.Errorf("register missing node removal script failed: %w", err)
	}

	if err := scriptManager.LoadScripts(ctx); err != nil {
		// ScriptManager falls back to EVAL and reloads asynchronously on NOSCRIPT.
		c.logger.Warn("[cluster-redis] load scripts failed, EVAL fallback will be used: %v", err)
	}

	c.redisScriptManager = scriptManager
	return nil
}
