//go:build test

package cluster

import (
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"
)

func TestElectionConsensusDiagnosis(t *testing.T) {
	n := 100
	clusters, _ := buildN100Clusters(n)

	startMeasure := time.Now()
	var wg sync.WaitGroup
	for _, c := range clusters {
		wg.Add(1)
		go func(cl *Cluster) {
			defer wg.Done()
			_ = cl.Start()
		}(c)
	}
	wg.Wait()
	t.Logf("所有节点启动完毕，耗时: %v", time.Since(startMeasure))

	firstLeaderResult := waitForFirstLeader(clusters, 30*time.Second)
	if firstLeaderResult < 0 {
		dumpClusterStates(t, clusters)
		dumpGoroutines(t)
		t.Fatal("30秒内没有节点成为 leader")
	}
	firstLeaderElapsed := time.Since(startMeasure)
	t.Logf("首个 leader 出现耗时: %v", firstLeaderElapsed)

	consensusResult, leaderName, agreedCount := waitForConsensus(clusters, 30*time.Second)
	totalConsensusElapsed := time.Since(startMeasure)
	if consensusResult < 0 {
		t.Logf("30秒内未达成完全共识，leader=%s, 认同数=%d/%d", leaderName, agreedCount, n)
		dumpClusterStates(t, clusters)
		dumpGoroutines(t)
		t.Fatal("共识超时")
	}
	t.Logf("所有节点达成共识耗时: %v（收敛: %v），leader: %s", totalConsensusElapsed, totalConsensusElapsed-firstLeaderElapsed, leaderName)

	for _, c := range clusters {
		c.Close()
	}
}

func dumpClusterStates(t *testing.T, clusters []*Cluster) {
	t.Helper()
	t.Log("===== 集群状态详情 =====")
	termDistribution := map[int]int{}
	for _, c := range clusters {
		if c.IsClosed() {
			continue
		}
		state := "unknown"
		switch {
		case c.IsLeader():
			state = "leader"
		case c.IsFollower():
			state = "follower"
		case c.IsCandidate():
			state = "candidate"
		}
		term := c.GetMyTerm()
		termDistribution[term]++
		t.Logf("  %s: state=%s term=%d leader=%s ready=%v alive=%d",
			c.GetMyName(), state, term, c.GetLeaderName(), c.IsReady(), c.GetAliveNodeCount())
	}
	t.Logf("term分布: %v", termDistribution)
	stats := collectLeaderStats(clusters)
	t.Logf("leader分布: %v", stats)
}

func dumpGoroutines(t *testing.T) {
	t.Helper()
	t.Log("===== Goroutine Dump =====")
	buf := make([]byte, 1<<20)
	n := runtime.Stack(buf, true)
	fmt.Printf("%s\n", buf[:n])
}
