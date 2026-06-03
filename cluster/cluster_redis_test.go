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
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/caiflower/common-tools/pkg/logger"
	redisv1 "github.com/caiflower/common-tools/redis/v1"
	"github.com/stretchr/testify/assert"
)

// newMiniredisClient 创建一个基于 miniredis 的 RedisClient（内存 Redis，适合集成测试）
func newMiniredisClient(t *testing.T, addr string) redisv1.RedisClient {
	t.Helper()
	client, err := redisv1.NewRedisClient(redisv1.Config{
		Addrs: []string{addr},
	})
	if err != nil {
		t.Fatalf("failed to create miniredis client: %v", err)
	}
	return client
}

// createTestRedisCluster 创建一个测试用的 Redis 模式 Cluster 实例
// 通过覆盖 curNode 来模拟不同节点身份，避免多节点测试时名称冲突
func createTestRedisCluster(t *testing.T, redisClient redisv1.RedisClient, name string, port int) *Cluster {
	t.Helper()

	cfg := Config{
		Mode:    "redis",
		Enable:  "true",
		Timeout: 5 * time.Second,
		RedisDiscovery: RedisDiscovery{
			DataPath:            "/test/redis",
			Port:                port,
			ElectionInterval:    500 * time.Millisecond,
			ElectionPeriod:      2 * time.Second,
			SyncLeaderInterval:  500 * time.Millisecond,
			NodeSyncInterval:    500 * time.Millisecond,
			NodeHeartbeatPeriod: 500 * time.Millisecond,
			NodeRegisterTTL:     5 * time.Second,
		},
	}

	cluster, err := NewClusterWithArgs(cfg, logger.NewLogger(&logger.Config{Level: "INFO"}))
	if err != nil {
		t.Fatalf("failed to create cluster: %v", err)
	}

	// 覆盖节点身份，使用唯一的 name 和 port 避免多节点冲突
	oldName := cluster.GetMyName()
	node := newNode(fmt.Sprintf("127.0.0.1:%d", port), name, 5)
	cluster.curNode = node
	cluster.allNode.Delete(oldName)
	cluster.allNode.Store(name, node)
	cluster.aliveNodes.Delete(oldName)
	cluster.aliveNodes.Store(name, node)
	cluster.Redis = redisClient

	return cluster
}

// countLeaders 统计 leaders 中为 leader 的节点数量
func countLeaders(clusters ...*Cluster) int {
	count := 0
	for _, c := range clusters {
		if c.IsLeader() {
			count++
		}
	}
	return count
}

// allClustersReady 检查所有集群是否就绪
func allClustersReady(clusters ...*Cluster) bool {
	for _, c := range clusters {
		if !c.IsReady() {
			return false
		}
	}
	return true
}

// TestRedisElection 测试 Redis 模式的选主：3 节点中应有且仅有一个 Leader
func TestRedisElection(t *testing.T) {
	mr := miniredis.RunT(t)
	redisClient := newMiniredisClient(t, mr.Addr())

	rand.Seed(time.Now().UnixNano())

	cluster1 := createTestRedisCluster(t, redisClient, "node-1", rand.Intn(10000)+8000)
	cluster2 := createTestRedisCluster(t, redisClient, "node-2", rand.Intn(10000)+8000)
	cluster3 := createTestRedisCluster(t, redisClient, "node-3", rand.Intn(10000)+8000)

	assert.NoError(t, cluster1.Start())
	assert.NoError(t, cluster2.Start())
	assert.NoError(t, cluster3.Start())

	defer func() {
		cluster1.Close()
		cluster2.Close()
		cluster3.Close()
	}()

	// 等待选举完成
	waitForReady(t, 20*time.Second, cluster1, cluster2, cluster3)

	// 验证有且仅有一个 Leader
	leaderCount := countLeaders(cluster1, cluster2, cluster3)
	assert.Equal(t, 1, leaderCount, "should have exactly one leader, got %d", leaderCount)

	// 验证所有节点看到同一个 Leader
	leaderName := ""
	for _, c := range []*Cluster{cluster1, cluster2, cluster3} {
		name := c.GetLeaderName()
		assert.NotEmpty(t, name, "each node should see a leader")
		if leaderName == "" {
			leaderName = name
		} else {
			assert.Equal(t, leaderName, name, "all nodes should see the same leader")
		}
	}
}

// TestRedisSingletonLeader 测试单节点直接成为 Leader
func TestRedisSingletonLeader(t *testing.T) {
	mr := miniredis.RunT(t)
	redisClient := newMiniredisClient(t, mr.Addr())

	cluster := createTestRedisCluster(t, redisClient, "singleton", 9001)
	assert.NoError(t, cluster.Start())
	defer cluster.Close()

	// 单节点应快速成为 Leader
	waitForReady(t, 10*time.Second, cluster)
	assert.True(t, cluster.IsLeader(), "single node should become leader")
	assert.Equal(t, "singleton", cluster.GetLeaderName())
}

// TestRedisLeaderFailover 测试 Leader 故障转移：Leader 关闭后，其他节点接管
// 注意：Leader 关闭后，选举 key 需要等 TTL 过期才能触发重新选举。
// miniredis 的 TTL 是基于模拟时钟的，需要调用 FastForward 来推进时间。
func TestRedisLeaderFailover(t *testing.T) {
	mr := miniredis.RunT(t)
	redisClient := newMiniredisClient(t, mr.Addr())

	cluster1 := createTestRedisCluster(t, redisClient, "node-1", 8101)
	cluster2 := createTestRedisCluster(t, redisClient, "node-2", 8102)
	cluster3 := createTestRedisCluster(t, redisClient, "node-3", 8103)

	assert.NoError(t, cluster1.Start())
	assert.NoError(t, cluster2.Start())
	assert.NoError(t, cluster3.Start())

	// 等待选举完成
	waitForReady(t, 15*time.Second, cluster1, cluster2, cluster3)

	// 找到 Leader 并关闭它
	var leader, follower1, follower2 *Cluster
	for _, c := range []*Cluster{cluster1, cluster2, cluster3} {
		if c.IsLeader() {
			leader = c
		} else if follower1 == nil {
			follower1 = c
		} else {
			follower2 = c
		}
	}
	assert.NotNil(t, leader, "should have a leader")
	assert.NotNil(t, follower1, "should have at least one follower")

	oldLeaderName := leader.GetMyName()
	t.Logf("initial leader: %s", oldLeaderName)

	// 关闭 Leader
	leader.Close()

	// miniredis 使用模拟时钟，需要用 FastForward 推进时间让 key 过期
	// ElectionPeriod=2s，加 1s 缓冲
	mr.FastForward(3 * time.Second)

	// 等待 follower 的 redisSyncLeader 检测到 key 过期并重新选举
	deadline := time.After(10 * time.Second)
	for {
		if follower1.IsLeader() || follower2.IsLeader() {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timed out waiting for new leader after failover")
		default:
			time.Sleep(200 * time.Millisecond)
		}
	}

	// 验证有新的 leader
	assert.True(t, follower1.IsLeader() || follower2.IsLeader(),
		"a new leader should be elected after old leader closes")

	// 验证新 leader 不是旧的
	newLeaderName := ""
	if follower1.IsLeader() {
		newLeaderName = follower1.GetLeaderName()
	} else {
		newLeaderName = follower2.GetLeaderName()
	}
	assert.NotEqual(t, oldLeaderName, newLeaderName, "new leader should be different from old leader")

	// 验证两个 follower 看到相同的 leader
	assert.Equal(t, follower1.GetLeaderName(), follower2.GetLeaderName(),
		"all nodes should see the same leader")

	// 清理
	follower1.Close()
	follower2.Close()
}

// TestRedisNodeRegistration 测试节点注册和发现
func TestRedisNodeRegistration(t *testing.T) {
	mr := miniredis.RunT(t)
	redisClient := newMiniredisClient(t, mr.Addr())

	rand.Seed(time.Now().UnixNano())
	port1 := rand.Intn(10000) + 8000
	port2 := rand.Intn(10000) + 8000

	cluster1 := createTestRedisCluster(t, redisClient, "node-a", port1)
	cluster2 := createTestRedisCluster(t, redisClient, "node-b", port2)

	assert.NoError(t, cluster1.Start())
	assert.NoError(t, cluster2.Start())
	defer cluster1.Close()
	defer cluster2.Close()

	// 等待节点注册和同步
	waitForReady(t, 15*time.Second, cluster1, cluster2)

	// 验证 cluster1 能发现 cluster2
	node2 := cluster1.GetNodeByName("node-b")
	assert.NotNil(t, node2, "cluster1 should discover node-b")

	// 验证 cluster2 能发现 cluster1
	node1 := cluster2.GetNodeByName("node-a")
	assert.NotNil(t, node1, "cluster2 should discover node-a")
}

// TestRedisElectionNoSplitVote 测试大规模节点场景下不会出现平票（Redis SETNX 保证互斥）
func TestRedisElectionNoSplitVote(t *testing.T) {
	mr := miniredis.RunT(t)
	redisClient := newMiniredisClient(t, mr.Addr())

	rand.Seed(time.Now().UnixNano())

	// 模拟 5 个节点同时选举
	const nodeCount = 5
	clusters := make([]*Cluster, nodeCount)
	for i := 0; i < nodeCount; i++ {
		port := rand.Intn(10000) + 8000
		clusters[i] = createTestRedisCluster(t, redisClient, fmt.Sprintf("node-%d", i+1), port)
		assert.NoError(t, clusters[i].Start())
	}

	// 等待选举完成
	waitForReady(t, 20*time.Second, clusters...)

	// 验证有且仅有一个 Leader（Redis SETNX 保证不会平票）
	leaderCount := countLeaders(clusters...)
	assert.Equal(t, 1, leaderCount, "Redis SETNX should guarantee exactly one leader even with %d nodes", nodeCount)

	// 清理
	for _, c := range clusters {
		c.Close()
	}
}

// waitForReady 等待所有集群就绪，超时则失败
func waitForReady(t *testing.T, timeout time.Duration, clusters ...*Cluster) {
	t.Helper()
	deadline := time.After(timeout)
	for {
		if allClustersReady(clusters...) {
			return
		}
		select {
		case <-deadline:
			// 打印调试信息
			for _, c := range clusters {
				t.Logf("cluster %s: ready=%v, leader=%s, isLeader=%v, term=%d",
					c.GetMyName(), c.IsReady(), c.GetLeaderName(), c.IsLeader(), c.GetMyTerm())
			}
			t.Fatal("clusters did not become ready within timeout")
		default:
			time.Sleep(200 * time.Millisecond)
		}
	}
}

// TestRedisFightingDirect 直接测试 redisFighting 选举逻辑（不启动完整集群）
func TestRedisFightingDirect(t *testing.T) {
	mr := miniredis.RunT(t)
	redisClient := newMiniredisClient(t, mr.Addr())

	cluster := createTestRedisCluster(t, redisClient, "direct-test", 9001)
	// 手动设置 context 和启动（不完全启动，只测试选举）
	cluster.ctx, cluster.cancelFunc = context.WithCancel(context.Background())
	defer cluster.cancelFunc()

	// 直接调用选举
	cluster.redisFighting()

	// 验证选举 key 已设置
	key := cluster.redisKeyElection()
	leaderName, err := cluster.Redis.GetString(cluster.ctx, key)
	assert.NoError(t, err)
	assert.Equal(t, "direct-test", leaderName)

	// 手动调用 redisSyncLeader 的核心逻辑
	node := cluster.GetNodeByName(leaderName)
	assert.NotNil(t, node, "should find the leader node in allNode")

	ok := cluster.signLeader(node, 0)
	assert.True(t, ok, "signLeader should succeed")
	assert.True(t, cluster.IsLeader(), "should become leader")
	assert.Equal(t, "direct-test", cluster.GetLeaderName())
	assert.True(t, cluster.IsReady(), "should be ready")
}

// TestRedisFightingOnlyOneWins 测试多个节点同时选举时只有一个获胜
func TestRedisFightingOnlyOneWins(t *testing.T) {
	mr := miniredis.RunT(t)
	redisClient := newMiniredisClient(t, mr.Addr())

	cluster1 := createTestRedisCluster(t, redisClient, "fighter-1", 9001)
	cluster2 := createTestRedisCluster(t, redisClient, "fighter-2", 9002)
	cluster3 := createTestRedisCluster(t, redisClient, "fighter-3", 9003)

	// 手动设置 context
	cluster1.ctx, cluster1.cancelFunc = context.WithCancel(context.Background())
	cluster2.ctx, cluster2.cancelFunc = context.WithCancel(context.Background())
	cluster3.ctx, cluster3.cancelFunc = context.WithCancel(context.Background())
	defer cluster1.cancelFunc()
	defer cluster2.cancelFunc()
	defer cluster3.cancelFunc()

	// 同时选举
	cluster1.redisFighting()
	cluster2.redisFighting()
	cluster3.redisFighting()

	// 验证只有一个获胜
	key := cluster1.redisKeyElection()
	leaderName, err := cluster1.Redis.GetString(cluster1.ctx, key)
	assert.NoError(t, err)
	assert.NotEmpty(t, leaderName)

	// 统计获胜者
	winCount := 0
	for _, c := range []*Cluster{cluster1, cluster2, cluster3} {
		node := c.GetNodeByName(leaderName)
		if node != nil {
			c.signLeader(node, 0)
			if c.IsLeader() {
				winCount++
			}
		}
	}
	assert.Equal(t, 1, winCount, "only one node should become leader")
}
