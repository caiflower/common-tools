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
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/caiflower/common-tools/pkg/logger"
)

const (
	numNodes = 100
)

// buildN100Clusters 构造 n 个节点的集群配置，所有节点共享同一份节点列表
func buildN100Clusters(n int) ([]*Cluster, []int) {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	// 为每个节点分配随机端口
	ports := make([]int, n)
	used := map[int]bool{}
	for i := range n {
		for {
			p := rng.Intn(20000) + 30000
			if !used[p] {
				used[p] = true
				ports[i] = p
				break
			}
		}
	}

	// 构建全量节点列表
	allNodes := make([]*struct {
		Name  string
		Ip    string
		Port  int
		Local bool
	}, n)
	for i := range n {
		allNodes[i] = &struct {
			Name  string
			Ip    string
			Port  int
			Local bool
		}{
			Ip:   "127.0.0.1",
			Name: fmt.Sprintf("node-%03d", i+1),
			Port: ports[i],
		}
	}

	clusters := make([]*Cluster, n)
	log := logger.NewLogger(&logger.Config{Level: "FATAL"}) // 减少日志干扰性能测试

	for i := range n {
		cfg := Config{}
		// 深拷贝节点列表，并标记当前节点
		for j, nd := range allNodes {
			entry := &struct {
				Name  string
				Ip    string
				Port  int
				Local bool
			}{
				Ip:    nd.Ip,
				Name:  nd.Name,
				Port:  nd.Port,
				Local: j == i,
			}
			cfg.Nodes = append(cfg.Nodes, entry)
		}

		c, err := NewClusterWithArgs(cfg, log)
		if err != nil {
			panic(fmt.Sprintf("创建节点 %d 失败: %v", i, err))
		}
		clusters[i] = c
	}

	return clusters, ports
}

// TestElectionPerformance100Nodes 测试 100 个节点场景下的选主性能
// 指标：
//  1. 首次选主耗时（从所有节点 Start 到第一个节点成为 leader）
//  2. 全部节点达成一致耗时（所有 follower 都认同同一个 leader）
//  3. leader 故障后重新选主耗时
func TestElectionPerformance100Nodes(t *testing.T) {
	t.Log("========== 100节点选主性能测试 开始 ==========")
	t.Logf("节点数量: %d", numNodes)

	clusters, _ := buildN100Clusters(numNodes)

	// ---- 阶段1：并发启动所有节点，记录首次选主时间 ----
	t.Log("[阶段1] 并发启动所有节点...")
	startAll := time.Now()

	var wg sync.WaitGroup
	for _, c := range clusters {
		wg.Add(1)
		go func(cl *Cluster) {
			defer wg.Done()
			if err := cl.Start(); err != nil {
				t.Logf("节点 %s 启动失败: %v", cl.GetMyName(), err)
			}
		}(c)
	}
	wg.Wait()

	t.Logf("所有节点启动完毕，耗时: %v", time.Since(startAll))

	// ---- 阶段2：等待首个 leader 出现 ----
	t.Log("[阶段2] 等待首个 leader 出现...")
	firstLeaderElapsed := waitForFirstLeader(clusters, 120*time.Second)
	if firstLeaderElapsed < 0 {
		t.Fatal("超时：120秒内没有节点成为 leader")
	}
	t.Logf("首个 leader 出现耗时: %v", firstLeaderElapsed)

	// ---- 阶段3：等待所有节点达成 leader 一致 ----
	t.Log("[阶段3] 等待所有节点达成 leader 共识...")
	consensusElapsed, leaderName, agreedCount := waitForConsensus(clusters, 120*time.Second)
	if consensusElapsed < 0 {
		// 未达成全部一致，输出当前统计
		stats := collectLeaderStats(clusters)
		t.Logf("120秒内未达成完全共识，当前各 leader 统计: %v", stats)
		t.Logf("已认同 leader(%s) 的节点数: %d / %d", leaderName, agreedCount, numNodes)
	} else {
		t.Logf("所有节点达成共识耗时: %v，leader: %s", consensusElapsed, leaderName)
	}

	// 输出当前集群状态快照
	printClusterSnapshot(t, clusters)

	// ---- 阶段4：模拟 leader 故障，测试重新选主耗时 ----
	t.Log("[阶段4] 模拟 leader 故障，测试重新选主...")
	leaderIdx := findLeaderIndex(clusters)
	if leaderIdx < 0 {
		t.Log("找不到当前 leader，跳过阶段4")
	} else {
		oldLeaderName := clusters[leaderIdx].GetMyName()
		oldTerm := clusters[leaderIdx].GetMyTerm()
		t.Logf("关闭 leader: %s (term=%d)", oldLeaderName, oldTerm)

		failoverStart := time.Now()
		clusters[leaderIdx].Close()

		// 在其余节点中等待新 leader
		remaining := make([]*Cluster, 0, numNodes-1)
		for i, c := range clusters {
			if i != leaderIdx {
				remaining = append(remaining, c)
			}
		}

		newLeaderElapsed := waitForFirstLeader(remaining, 120*time.Second)
		if newLeaderElapsed < 0 {
			t.Fatal("leader 故障后 120 秒内未选出新 leader")
		}
		t.Logf("leader 故障后重新选主耗时: %v (从关闭leader开始: %v)",
			newLeaderElapsed, time.Since(failoverStart))

		// 等待剩余节点达成共识
		reConsensusElapsed, newLeader, agreedCnt := waitForConsensus(remaining, 120*time.Second)
		if reConsensusElapsed < 0 {
			t.Logf("重新选主后共识未完全达成，已认同新leader(%s)的节点数: %d / %d",
				newLeader, agreedCnt, len(remaining))
		} else {
			t.Logf("重新选主后所有节点达成共识耗时: %v，新leader: %s", reConsensusElapsed, newLeader)
		}

		printClusterSnapshot(t, remaining)

		// 关闭剩余节点
		t.Log("[清理] 关闭所有剩余节点...")
		for _, c := range remaining {
			go c.Close()
		}
	}

	t.Log("========== 100节点选主性能测试 结束 ==========")
}

// TestElectionScalability 多规模对比测试：3 / 10 / 30 / 100 节点选主耗时对比
func TestElectionScalability(t *testing.T) {
	sizes := []int{3, 10, 30, 100}

	type result struct {
		size            int
		firstLeaderMs   int64
		fullConsensusMs int64
		failoverMs      int64
	}

	results := make([]result, 0, len(sizes))

	for _, n := range sizes {
		t.Logf("===== 规模测试: %d 节点 =====", n)
		clusters, _ := buildN100Clusters(n)

		startAll := time.Now()
		var wg sync.WaitGroup
		for _, c := range clusters {
			wg.Add(1)
			go func(cl *Cluster) {
				defer wg.Done()
				_ = cl.Start()
			}(c)
		}
		wg.Wait()
		_ = startAll

		timeout := time.Duration(n) * 3 * time.Second
		timeout = max(timeout, 30*time.Second)
		timeout = min(timeout, 180*time.Second)

		firstLeaderElapsed := waitForFirstLeader(clusters, timeout)
		consensusElapsed, _, _ := waitForConsensus(clusters, timeout)

		// leader 故障测试
		leaderIdx := findLeaderIndex(clusters)
		var failoverElapsed time.Duration
		if leaderIdx >= 0 {
			failoverStart := time.Now()
			clusters[leaderIdx].Close()
			remaining := make([]*Cluster, 0, n-1)
			for i, c := range clusters {
				if i != leaderIdx {
					remaining = append(remaining, c)
				}
			}
			fe := waitForFirstLeader(remaining, timeout)
			if fe >= 0 {
				failoverElapsed = fe
			} else {
				failoverElapsed = -1
			}
			_ = failoverStart
			for _, c := range remaining {
				go c.Close()
			}
		} else {
			for _, c := range clusters {
				go c.Close()
			}
		}

		var r result
		r.size = n
		if firstLeaderElapsed >= 0 {
			r.firstLeaderMs = firstLeaderElapsed.Milliseconds()
		} else {
			r.firstLeaderMs = -1
		}
		if consensusElapsed >= 0 {
			r.fullConsensusMs = consensusElapsed.Milliseconds()
		} else {
			r.fullConsensusMs = -1
		}
		r.failoverMs = failoverElapsed.Milliseconds()

		results = append(results, r)
		t.Logf("节点数=%d | 首个leader出现=%dms | 全员共识=%dms | 故障转移=%dms",
			r.size, r.firstLeaderMs, r.fullConsensusMs, r.failoverMs)

		time.Sleep(2 * time.Second) // 等待端口释放
	}

	t.Log("\n========== 规模测试汇总 ==========")
	t.Logf("%-10s %-20s %-20s %-20s", "节点数", "首个leader(ms)", "全员共识(ms)", "故障转移(ms)")
	for _, r := range results {
		t.Logf("%-10d %-20d %-20d %-20d", r.size, r.firstLeaderMs, r.fullConsensusMs, r.failoverMs)
	}
}

// ---- 辅助函数 ----

// waitForFirstLeader 等待集群中出现第一个 leader，返回等待时长；超时返回 -1
func waitForFirstLeader(clusters []*Cluster, timeout time.Duration) time.Duration {
	start := time.Now()
	deadline := start.Add(timeout)
	for time.Now().Before(deadline) {
		for _, c := range clusters {
			if c.IsLeader() {
				return time.Since(start)
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	return -1
}

// waitForConsensus 等待所有节点认同同一个 leader。
// 返回 (耗时, leaderName, 认同节点数)；超时时耗时为 -1
func waitForConsensus(clusters []*Cluster, timeout time.Duration) (time.Duration, string, int) {
	start := time.Now()
	deadline := start.Add(timeout)

	for time.Now().Before(deadline) {
		stats := collectLeaderStats(clusters)
		// 找票数最多的 leader
		maxCount := 0
		maxLeader := ""
		for name, cnt := range stats {
			if cnt > maxCount {
				maxCount = cnt
				maxLeader = name
			}
		}
		// 排除 ""（无leader）
		if maxLeader != "" && maxCount == len(clusters) {
			return time.Since(start), maxLeader, maxCount
		}
		time.Sleep(100 * time.Millisecond)
	}

	// 超时，返回当前最多认同的 leader
	stats := collectLeaderStats(clusters)
	maxCount := 0
	maxLeader := ""
	for name, cnt := range stats {
		if name != "" && cnt > maxCount {
			maxCount = cnt
			maxLeader = name
		}
	}
	return -1, maxLeader, maxCount
}

// collectLeaderStats 统计各节点认同的 leader 分布
func collectLeaderStats(clusters []*Cluster) map[string]int {
	stats := make(map[string]int)
	for _, c := range clusters {
		if c.IsClosed() {
			continue
		}
		ln := c.GetLeaderName()
		stats[ln]++
	}
	return stats
}

// findLeaderIndex 在 clusters 中找到当前是 leader 状态的节点下标，没有则返回 -1
func findLeaderIndex(clusters []*Cluster) int {
	for i, c := range clusters {
		if c.IsLeader() {
			return i
		}
	}
	return -1
}

// printClusterSnapshot 打印集群当前状态快照（leader/follower 分布）
func printClusterSnapshot(t *testing.T, clusters []*Cluster) {
	t.Helper()
	var leaderCount, followerCount, candidateCount, closedCount int32
	leaderNames := sync.Map{}

	for _, c := range clusters {
		switch {
		case c.IsClosed():
			atomic.AddInt32(&closedCount, 1)
		case c.IsLeader():
			atomic.AddInt32(&leaderCount, 1)
			leaderNames.Store(c.GetMyName(), c.GetMyTerm())
		case c.IsCandidate():
			atomic.AddInt32(&candidateCount, 1)
		case c.IsFollower():
			atomic.AddInt32(&followerCount, 1)
		}
	}

	t.Logf("集群快照: leader=%d follower=%d candidate=%d closed=%d (共%d节点)",
		leaderCount, followerCount, candidateCount, closedCount, len(clusters))

	leaderNames.Range(func(k, v interface{}) bool {
		t.Logf("  ↳ leader: %s (term=%d)", k, v)
		return true
	})
}
