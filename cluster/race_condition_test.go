package cluster

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/caiflower/common-tools/pkg/logger"
)

func TestRaceCondition_TermConcurrentAccess(t *testing.T) {
	c1 := Config{Enable: "true", Mode: modeSingle}
	cluster, err := NewClusterWithArgs(c1, WithLogger(logger.NewLogger(&logger.Config{
		Level: "FATAL",
	})))
	if err != nil {
		t.Fatalf("failed to create cluster: %v", err)
	}

	if err := cluster.Start(); err != nil {
		t.Fatalf("failed to start cluster: %v", err)
	}
	defer cluster.Close()

	time.Sleep(100 * time.Millisecond)

	var wg sync.WaitGroup
	var stop atomic.Int32

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for stop.Load() == 0 {
				_ = cluster.GetMyTerm()
			}
		}()
	}

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for stop.Load() == 0 {
				cluster.signLeader(cluster.GetMyNode(), id)
			}
		}(i)
	}

	time.Sleep(500 * time.Millisecond)
	stop.Store(1)
	wg.Wait()
}

func TestRaceCondition_TermFightingAndRead(t *testing.T) {
	cluster1, cluster2, cluster3 := common()
	_ = cluster1.Start()
	_ = cluster2.Start()
	_ = cluster3.Start()

	time.Sleep(2 * time.Second)

	var wg sync.WaitGroup
	var stop atomic.Int32

	for _, c := range []*Cluster{cluster1, cluster2, cluster3} {
		cc := c
		wg.Add(1)
		go func() {
			defer wg.Done()
			for stop.Load() == 0 {
				_ = cc.GetMyTerm()
				_ = cc.GetLeaderName()
				_ = cc.IsLeader()
				_ = cc.IsFollower()
				_ = cc.IsReady()
			}
		}()
	}

	time.Sleep(2 * time.Second)
	stop.Store(1)
	wg.Wait()

	cluster1.Close()
	cluster2.Close()
	cluster3.Close()
}

func TestRaceCondition_RegisterFuncConcurrentAccess(t *testing.T) {
	c1 := Config{Enable: "true", Mode: modeSingle}
	cluster, err := NewClusterWithArgs(c1, WithLogger(logger.NewLogger(&logger.Config{
		Level: "FATAL",
	})))
	if err != nil {
		t.Fatalf("failed to create cluster: %v", err)
	}

	if err := cluster.Start(); err != nil {
		t.Fatalf("failed to start cluster: %v", err)
	}
	defer cluster.Close()

	time.Sleep(100 * time.Millisecond)

	var wg sync.WaitGroup
	var stop atomic.Int32

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for stop.Load() == 0 {
				cluster.RegisterFunc(fmt.Sprintf("func_%d", id), func(data interface{}) (interface{}, error) {
					return id, nil
				})
			}
		}(i)
	}

	time.Sleep(500 * time.Millisecond)
	stop.Store(1)
	wg.Wait()
}

func TestRaceCondition_SignLeaderAndReleaseLeader(t *testing.T) {
	c1 := Config{Enable: "true", Mode: modeSingle}
	cluster, err := NewClusterWithArgs(c1, WithLogger(logger.NewLogger(&logger.Config{
		Level: "FATAL",
	})))
	if err != nil {
		t.Fatalf("failed to create cluster: %v", err)
	}

	if err := cluster.Start(); err != nil {
		t.Fatalf("failed to start cluster: %v", err)
	}
	defer cluster.Close()

	time.Sleep(100 * time.Millisecond)

	var wg sync.WaitGroup
	var stop atomic.Int32

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for stop.Load() == 0 {
				cluster.signLeader(cluster.GetMyNode(), id)
			}
		}(i)
	}

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for stop.Load() == 0 {
				cluster.releaseLeader()
			}
		}()
	}

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for stop.Load() == 0 {
				_ = cluster.GetMyTerm()
				_ = cluster.GetLeaderNode()
				_ = cluster.GetLeaderName()
			}
		}()
	}

	time.Sleep(500 * time.Millisecond)
	stop.Store(1)
	wg.Wait()
}

func TestRaceCondition_HeartbeatStreamAccess(t *testing.T) {
	cluster1, cluster2, cluster3 := common()
	_ = cluster1.Start()
	_ = cluster2.Start()
	_ = cluster3.Start()

	waitAllForReady(t, cluster1, cluster2, cluster3)

	var wg sync.WaitGroup
	var stop atomic.Int32

	for _, c := range []*Cluster{cluster1, cluster2, cluster3} {
		cc := c
		wg.Add(1)
		go func() {
			defer wg.Done()
			for stop.Load() == 0 {
				_ = cc.GetMyTerm()
				_ = cc.GetLeaderName()
				_ = cc.GetAliveNodeNames()
				_ = cc.GetAliveNodeCount()
			}
		}()
	}

	time.Sleep(2 * time.Second)
	stop.Store(1)
	wg.Wait()

	cluster1.Close()
	cluster2.Close()
	cluster3.Close()
}

func TestRaceCondition_CloseAndRead(t *testing.T) {
	c1 := Config{Enable: "true", Mode: modeSingle}
	cluster, err := NewClusterWithArgs(c1, WithLogger(logger.NewLogger(&logger.Config{
		Level: "FATAL",
	})))
	if err != nil {
		t.Fatalf("failed to create cluster: %v", err)
	}

	if err := cluster.Start(); err != nil {
		t.Fatalf("failed to start cluster: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	var wg sync.WaitGroup
	var stop atomic.Int32

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for stop.Load() == 0 {
				_ = cluster.GetMyTerm()
				_ = cluster.GetLeaderName()
				_ = cluster.IsReady()
				_ = cluster.IsLeader()
			}
		}()
	}

	time.Sleep(200 * time.Millisecond)
	cluster.Close()
	stop.Store(1)
	wg.Wait()
}

func TestRaceCondition_RegisterFuncAndCallFuncConcurrent(t *testing.T) {
	c1 := Config{Enable: "true", Mode: modeSingle}
	cluster, err := NewClusterWithArgs(c1, WithLogger(logger.NewLogger(&logger.Config{
		Level: "FATAL",
	})))
	if err != nil {
		t.Fatalf("failed to create cluster: %v", err)
	}
	if err := cluster.Start(); err != nil {
		t.Fatalf("failed to start cluster: %v", err)
	}
	defer cluster.Close()
	time.Sleep(100 * time.Millisecond)

	for i := 0; i < 5; i++ {
		cluster.RegisterFunc(fmt.Sprintf("race_func_%d", i), func(data interface{}) (interface{}, error) {
			return i, nil
		})
	}

	var wg sync.WaitGroup
	var stop atomic.Int32

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for stop.Load() == 0 {
				cluster.RegisterFunc(fmt.Sprintf("race_func_%d", id), func(data interface{}) (interface{}, error) {
					return id, nil
				})
			}
		}(i)
	}

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for stop.Load() == 0 {
				f := &FuncSpec{
					funcName: fmt.Sprintf("race_func_%d", id),
					nodeName: cluster.GetMyName(),
					timeout:  100 * time.Millisecond,
				}
				cluster.callLocalFunc(f)
			}
		}(i)
	}

	time.Sleep(500 * time.Millisecond)
	stop.Store(1)
	wg.Wait()
}

func TestRaceCondition_CloseWithoutStart(t *testing.T) {
	c1 := Config{Enable: "true", Mode: modeSingle}
	cluster, err := NewClusterWithArgs(c1, WithLogger(logger.NewLogger(&logger.Config{
		Level: "FATAL",
	})))
	if err != nil {
		t.Fatalf("failed to create cluster: %v", err)
	}
	cluster.Close()
}

func TestRaceCondition_TriggerReconnectDedup(t *testing.T) {
	cluster1, cluster2, cluster3 := common()
	_ = cluster1.Start()
	_ = cluster2.Start()
	_ = cluster3.Start()
	waitAllForReady(t, cluster1, cluster2, cluster3)
	defer cluster1.Close()
	defer cluster2.Close()
	defer cluster3.Close()

	var wg sync.WaitGroup
	var stop atomic.Int32

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for stop.Load() == 0 {
				cluster1.triggerReconnect()
				time.Sleep(10 * time.Millisecond)
			}
		}()
	}

	time.Sleep(2 * time.Second)
	stop.Store(1)
	wg.Wait()
}

func TestRaceCondition_ConcurrentClose(t *testing.T) {
	c1 := Config{Enable: "true", Mode: modeSingle}
	cluster, err := NewClusterWithArgs(c1, WithLogger(logger.NewLogger(&logger.Config{
		Level: "FATAL",
	})))
	if err != nil {
		t.Fatalf("failed to create cluster: %v", err)
	}
	if err := cluster.Start(); err != nil {
		t.Fatalf("failed to start cluster: %v", err)
	}
	time.Sleep(100 * time.Millisecond)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cluster.Close()
		}()
	}
	wg.Wait()
}

func TestRaceCondition_MultiClusterConcurrentClose(t *testing.T) {
	cluster1, cluster2, cluster3 := common()
	_ = cluster1.Start()
	_ = cluster2.Start()
	_ = cluster3.Start()
	waitAllForReady(t, cluster1, cluster2, cluster3)

	var wg sync.WaitGroup
	for _, c := range []*Cluster{cluster1, cluster2, cluster3} {
		wg.Add(1)
		go func(cl *Cluster) {
			defer wg.Done()
			cl.Close()
		}(c)
	}
	wg.Wait()
}

func TestRaceCondition_AliveNodesConcurrentReadWrite(t *testing.T) {
	cluster1, cluster2, cluster3 := common()
	_ = cluster1.Start()
	_ = cluster2.Start()
	_ = cluster3.Start()
	waitAllForReady(t, cluster1, cluster2, cluster3)

	var wg sync.WaitGroup
	var stop atomic.Int32

	for _, c := range []*Cluster{cluster1, cluster2, cluster3} {
		cc := c
		wg.Add(1)
		go func() {
			defer wg.Done()
			for stop.Load() == 0 {
				_ = cc.GetAliveNodeNames()
				_ = cc.GetAliveNodeCount()
				_ = cc.GetLostNodeNames()
			}
		}()
	}

	time.Sleep(500 * time.Millisecond)
	cluster2.Close()

	time.Sleep(1 * time.Second)
	stop.Store(1)
	wg.Wait()

	cluster1.Close()
	cluster3.Close()
}

func TestRaceCondition_EventsChannelSendAndClose(t *testing.T) {
	c1 := Config{Enable: "true", Mode: modeSingle}
	cluster, err := NewClusterWithArgs(c1, WithLogger(logger.NewLogger(&logger.Config{
		Level: "FATAL",
	})))
	if err != nil {
		t.Fatalf("failed to create cluster: %v", err)
	}
	if err := cluster.Start(); err != nil {
		t.Fatalf("failed to start cluster: %v", err)
	}
	time.Sleep(100 * time.Millisecond)

	var wg sync.WaitGroup
	var stop atomic.Int32

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for stop.Load() == 0 {
				cluster.signLeader(cluster.GetMyNode(), id)
			}
		}(i)
	}

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for stop.Load() == 0 {
				cluster.releaseLeader()
			}
		}()
	}

	time.Sleep(200 * time.Millisecond)
	cluster.Close()
	stop.Store(1)
	wg.Wait()
}

func TestRaceCondition_CallFuncRemoteWithNodeChange(t *testing.T) {
	cluster1, cluster2, cluster3 := common()
	_ = cluster1.Start()
	_ = cluster2.Start()
	_ = cluster3.Start()
	waitAllForReady(t, cluster1, cluster2, cluster3)

	for _, c := range []*Cluster{cluster1, cluster2, cluster3} {
		c.RegisterFunc("remote_race_func", func(data interface{}) (interface{}, error) {
			return data, nil
		})
	}

	var wg sync.WaitGroup
	var stop atomic.Int32

	for _, c := range []*Cluster{cluster1, cluster3} {
		cc := c
		wg.Add(1)
		go func() {
			defer wg.Done()
			for stop.Load() == 0 {
				f := NewFuncSpec(cluster2.GetMyName(), "remote_race_func", "test", 100*time.Millisecond)
				_, _ = cc.CallFunc(f)
			}
		}()
	}

	time.Sleep(500 * time.Millisecond)
	cluster2.Close()

	time.Sleep(1 * time.Second)
	stop.Store(1)
	wg.Wait()

	cluster1.Close()
	cluster3.Close()
}

func TestRaceCondition_ReleaseWithNodeNameAndSignLeader(t *testing.T) {
	c1 := Config{Enable: "true", Mode: modeSingle}
	cluster, err := NewClusterWithArgs(c1, WithLogger(logger.NewLogger(&logger.Config{
		Level: "FATAL",
	})))
	if err != nil {
		t.Fatalf("failed to create cluster: %v", err)
	}
	if err := cluster.Start(); err != nil {
		t.Fatalf("failed to start cluster: %v", err)
	}
	time.Sleep(100 * time.Millisecond)

	var wg sync.WaitGroup
	var stop atomic.Int32

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for stop.Load() == 0 {
				cluster.signLeader(cluster.GetMyNode(), id)
			}
		}(i)
	}

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for stop.Load() == 0 {
				cluster.releaseWithNodeName(cluster.GetMyName())
			}
		}()
	}

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for stop.Load() == 0 {
				_ = cluster.GetLeaderNode()
				_ = cluster.GetLeaderName()
			}
		}()
	}

	time.Sleep(500 * time.Millisecond)
	stop.Store(1)
	wg.Wait()
	cluster.Close()
}

func TestRaceCondition_StateTransitionWithConcurrentReads(t *testing.T) {
	cluster1, cluster2, cluster3 := common()
	_ = cluster1.Start()
	_ = cluster2.Start()
	_ = cluster3.Start()
	waitAllForReady(t, cluster1, cluster2, cluster3)

	var wg sync.WaitGroup
	var stop atomic.Int32

	clusters := []*Cluster{cluster1, cluster2, cluster3}
	for _, c := range clusters {
		cc := c
		wg.Add(1)
		go func() {
			defer wg.Done()
			for stop.Load() == 0 {
				_ = cc.IsLeader()
				_ = cc.IsFollower()
				_ = cc.IsCandidate()
				_ = cc.IsReady()
			}
		}()
	}

	time.Sleep(500 * time.Millisecond)

	leaderIdx := -1
	for i, c := range clusters {
		if c.IsLeader() {
			leaderIdx = i
			break
		}
	}
	if leaderIdx >= 0 {
		clusters[leaderIdx].Close()
	}

	time.Sleep(2 * time.Second)
	stop.Store(1)
	wg.Wait()

	for i, c := range clusters {
		if i != leaderIdx {
			c.Close()
		}
	}
}

func TestRaceCondition_ReconnectAndCloseConcurrent(t *testing.T) {
	cluster1, cluster2, cluster3 := common()
	_ = cluster1.Start()
	_ = cluster2.Start()
	_ = cluster3.Start()
	waitAllForReady(t, cluster1, cluster2, cluster3)

	var wg sync.WaitGroup
	var stop atomic.Int32

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for stop.Load() == 0 {
				cluster1.triggerReconnect()
				time.Sleep(10 * time.Millisecond)
			}
		}()
	}

	time.Sleep(500 * time.Millisecond)
	cluster1.Close()
	stop.Store(1)
	wg.Wait()

	cluster2.Close()
	cluster3.Close()
}

func TestRaceCondition_GetAliveNodeInfoWithNodeChange(t *testing.T) {
	cluster1, cluster2, cluster3 := common()
	_ = cluster1.Start()
	_ = cluster2.Start()
	_ = cluster3.Start()
	waitAllForReady(t, cluster1, cluster2, cluster3)

	var wg sync.WaitGroup
	var stop atomic.Int32

	wg.Add(1)
	go func() {
		defer wg.Done()
		for stop.Load() == 0 {
			_ = cluster1.GetAliveNodeNames()
			_ = cluster1.GetAliveNodeCount()
			_ = cluster1.GetLostNodeNames()
		}
	}()

	time.Sleep(500 * time.Millisecond)
	cluster2.Close()

	time.Sleep(1 * time.Second)
	stop.Store(1)
	wg.Wait()

	cluster1.Close()
	cluster3.Close()
}

type mockJobTracker struct {
	name string
}

func (m *mockJobTracker) Name() string                         { return m.name }
func (m *mockJobTracker) OnStartedLeading()                    {}
func (m *mockJobTracker) OnStoppedLeading()                    {}
func (m *mockJobTracker) OnStartedFollowing(leaderName string) {}
func (m *mockJobTracker) OnStoppedFollowing()                  {}

func TestRaceCondition_JobTrackerConcurrentAccess(t *testing.T) {
	c1 := Config{Enable: "true", Mode: modeSingle}
	cluster, err := NewClusterWithArgs(c1, WithLogger(logger.NewLogger(&logger.Config{
		Level: "FATAL",
	})))
	if err != nil {
		t.Fatalf("failed to create cluster: %v", err)
	}
	if err := cluster.Start(); err != nil {
		t.Fatalf("failed to start cluster: %v", err)
	}
	time.Sleep(100 * time.Millisecond)

	var wg sync.WaitGroup
	var stop atomic.Int32

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for stop.Load() == 0 {
				_ = cluster.AddJobTracker(&mockJobTracker{name: fmt.Sprintf("tracker_%d", id)})
			}
		}(i)
	}

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for stop.Load() == 0 {
				cluster.RemoveJobTracker(&mockJobTracker{name: fmt.Sprintf("tracker_%d", id)})
			}
		}(i)
	}

	time.Sleep(500 * time.Millisecond)
	cluster.Close()
	stop.Store(1)
	wg.Wait()
}

func TestRaceCondition_GetGRPCClientWithNodeChange(t *testing.T) {
	cluster1, cluster2, cluster3 := common()
	_ = cluster1.Start()
	_ = cluster2.Start()
	_ = cluster3.Start()
	waitAllForReady(t, cluster1, cluster2, cluster3)

	var wg sync.WaitGroup
	var stop atomic.Int32

	wg.Add(1)
	go func() {
		defer wg.Done()
		for stop.Load() == 0 {
			_, _ = cluster1.GetGRPCClient(cluster2.GetMyName())
			_, _ = cluster1.GetGRPCClient(cluster3.GetMyName())
		}
	}()

	time.Sleep(500 * time.Millisecond)
	cluster2.Close()

	time.Sleep(1 * time.Second)
	stop.Store(1)
	wg.Wait()

	cluster1.Close()
	cluster3.Close()
}

func TestRaceCondition_VotesMapConcurrentAccess(t *testing.T) {
	c1 := Config{Enable: "true", Mode: modeSingle}
	cluster, err := NewClusterWithArgs(c1, WithLogger(logger.NewLogger(&logger.Config{
		Level: "FATAL",
	})))
	if err != nil {
		t.Fatalf("failed to create cluster: %v", err)
	}
	if err := cluster.Start(); err != nil {
		t.Fatalf("failed to start cluster: %v", err)
	}
	defer cluster.Close()
	time.Sleep(100 * time.Millisecond)

	var wg sync.WaitGroup
	var stop atomic.Int32

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for stop.Load() == 0 {
				_ = cluster.voteNode(id, fmt.Sprintf("voter_%d", id))
			}
		}(i)
	}

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for stop.Load() == 0 {
				_ = cluster.getVoteNodeName(id, fmt.Sprintf("getter_%d", id))
			}
		}(i)
	}

	time.Sleep(500 * time.Millisecond)
	stop.Store(1)
	wg.Wait()
}

func TestRaceCondition_LeaderNodeMultiNodeConcurrent(t *testing.T) {
	cluster1, cluster2, cluster3 := common()
	_ = cluster1.Start()
	_ = cluster2.Start()
	_ = cluster3.Start()
	waitAllForReady(t, cluster1, cluster2, cluster3)

	var wg sync.WaitGroup
	var stop atomic.Int32

	for _, c := range []*Cluster{cluster1, cluster2, cluster3} {
		cc := c
		wg.Add(1)
		go func() {
			defer wg.Done()
			for stop.Load() == 0 {
				_ = cc.GetLeaderNode()
				_ = cc.GetLeaderName()
				_ = cc.GetMyTerm()
			}
		}()
	}

	time.Sleep(500 * time.Millisecond)

	leaderIdx := -1
	clusters := []*Cluster{cluster1, cluster2, cluster3}
	for i, c := range clusters {
		if c.IsLeader() {
			leaderIdx = i
			clusters[i].Close()
			break
		}
	}

	time.Sleep(2 * time.Second)
	stop.Store(1)
	wg.Wait()

	for i, c := range clusters {
		if i != leaderIdx {
			c.Close()
		}
	}
}

func TestRaceCondition_ConcurrentCallFuncSameNode(t *testing.T) {
	cluster1, cluster2, cluster3 := common()
	_ = cluster1.Start()
	_ = cluster2.Start()
	_ = cluster3.Start()
	waitAllForReady(t, cluster1, cluster2, cluster3)

	cluster2.RegisterFunc("concurrent_func", func(data interface{}) (interface{}, error) {
		return data, nil
	})

	var wg sync.WaitGroup
	var stop atomic.Int32

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for stop.Load() == 0 {
				f := NewFuncSpec(cluster2.GetMyName(), "concurrent_func", fmt.Sprintf("data_%d", id), 200*time.Millisecond)
				_, _ = cluster1.CallFunc(f)
			}
		}(i)
	}

	time.Sleep(2 * time.Second)
	stop.Store(1)
	wg.Wait()

	cluster1.Close()
	cluster2.Close()
	cluster3.Close()
}

func TestRaceCondition_MultipleNodesFailSimultaneously(t *testing.T) {
	cluster1, cluster2, cluster3 := common()
	_ = cluster1.Start()
	_ = cluster2.Start()
	_ = cluster3.Start()
	waitAllForReady(t, cluster1, cluster2, cluster3)

	var wg sync.WaitGroup
	var stop atomic.Int32

	wg.Add(1)
	go func() {
		defer wg.Done()
		for stop.Load() == 0 {
			_ = cluster1.GetAliveNodeNames()
			_ = cluster1.GetAliveNodeCount()
			_ = cluster1.GetLeaderName()
			_ = cluster1.IsReady()
		}
	}()

	time.Sleep(500 * time.Millisecond)

	var closeWg sync.WaitGroup
	closeWg.Add(2)
	go func() {
		defer closeWg.Done()
		cluster2.Close()
	}()
	go func() {
		defer closeWg.Done()
		cluster3.Close()
	}()
	closeWg.Wait()

	time.Sleep(2 * time.Second)
	stop.Store(1)
	wg.Wait()

	cluster1.Close()
}

func TestRaceCondition_GetAllNodeInfoWithNodeChange(t *testing.T) {
	cluster1, cluster2, cluster3 := common()
	_ = cluster1.Start()
	_ = cluster2.Start()
	_ = cluster3.Start()
	waitAllForReady(t, cluster1, cluster2, cluster3)

	var wg sync.WaitGroup
	var stop atomic.Int32

	wg.Add(1)
	go func() {
		defer wg.Done()
		for stop.Load() == 0 {
			_ = cluster1.GetAllNodeNames()
			_ = cluster1.GetAllNodeCount()
			_ = cluster1.GetNodeByName(cluster2.GetMyName())
		}
	}()

	time.Sleep(500 * time.Millisecond)
	cluster2.Close()

	time.Sleep(1 * time.Second)
	stop.Store(1)
	wg.Wait()

	cluster1.Close()
	cluster3.Close()
}

func TestRaceCondition_CallFuncWithNodeClose(t *testing.T) {
	cluster1, cluster2, cluster3 := common()
	_ = cluster1.Start()
	_ = cluster2.Start()
	_ = cluster3.Start()
	waitAllForReady(t, cluster1, cluster2, cluster3)

	cluster2.RegisterFunc("slow_func", func(data interface{}) (interface{}, error) {
		time.Sleep(50 * time.Millisecond)
		return data, nil
	})

	var wg sync.WaitGroup
	var stop atomic.Int32

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for stop.Load() == 0 {
				f := NewFuncSpec(cluster2.GetMyName(), "slow_func", fmt.Sprintf("data_%d", id), 500*time.Millisecond)
				_, _ = cluster1.CallFunc(f)
			}
		}(i)
	}

	time.Sleep(300 * time.Millisecond)
	cluster2.Close()

	time.Sleep(2 * time.Second)
	stop.Store(1)
	wg.Wait()

	cluster1.Close()
	cluster3.Close()
}

func TestRaceCondition_GetMyInfoConcurrentWithClose(t *testing.T) {
	cluster1, cluster2, cluster3 := common()
	_ = cluster1.Start()
	_ = cluster2.Start()
	_ = cluster3.Start()
	waitAllForReady(t, cluster1, cluster2, cluster3)

	var wg sync.WaitGroup
	var stop atomic.Int32

	for _, c := range []*Cluster{cluster1, cluster2, cluster3} {
		cc := c
		wg.Add(1)
		go func() {
			defer wg.Done()
			for stop.Load() == 0 {
				_ = cc.GetMyName()
				_ = cc.GetMyAddress()
				_ = cc.GetMyTerm()
				_ = cc.IsClose()
				_ = cc.IsClosed()
			}
		}()
	}

	time.Sleep(500 * time.Millisecond)

	var closeWg sync.WaitGroup
	closeWg.Add(3)
	for _, c := range []*Cluster{cluster1, cluster2, cluster3} {
		cc := c
		go func() {
			defer closeWg.Done()
			cc.Close()
		}()
	}
	closeWg.Wait()
	stop.Store(1)
	wg.Wait()
}
