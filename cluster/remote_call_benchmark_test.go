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
	benchFuncName = "benchRemoteCallFunc"
)

func benchRemoteCallFn(data interface{}) (interface{}, error) {
	return data, nil
}

func buildBenchClusters(n int) ([]*Cluster, func()) {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	ports := make([]int, n)
	used := map[int]bool{}
	for i := range n {
		for {
			p := rng.Intn(20000) + 40000
			if !used[p] {
				used[p] = true
				ports[i] = p
				break
			}
		}
	}

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
			Name: fmt.Sprintf("bench-node-%03d", i+1),
			Port: ports[i],
		}
	}

	log := logger.NewLogger(&logger.Config{Level: "FATAL"})
	clusters := make([]*Cluster, n)

	for i := range n {
		cfg := Config{}
		for j, nd := range allNodes {
			entry := &struct {
				Name  string
				IP    string
				Port  int
				Local bool
			}{
				IP:    nd.Ip,
				Name:  nd.Name,
				Port:  nd.Port,
				Local: j == i,
			}
			cfg.Nodes = append(cfg.Nodes, entry)
		}

		c, err := NewClusterWithArgs(cfg, WithLogger(log))
		if err != nil {
			panic(fmt.Sprintf("create bench cluster node %d failed: %v", i, err))
		}
		c.RegisterFunc(benchFuncName, benchRemoteCallFn)
		clusters[i] = c
	}

	for _, c := range clusters {
		if err := c.Start(); err != nil {
			panic(fmt.Sprintf("start bench cluster failed: %v", err))
		}
	}

	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		ready := true
		for _, c := range clusters {
			if !c.IsReady() {
				ready = false
				break
			}
		}
		if ready {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	cleanup := func() {
		for _, c := range clusters {
			c.Close()
		}
	}

	return clusters, cleanup
}

func findBenchLeader(clusters []*Cluster) *Cluster {
	for _, c := range clusters {
		if c.IsLeader() {
			return c
		}
	}
	return nil
}

func findBenchFollower(clusters []*Cluster) *Cluster {
	for _, c := range clusters {
		if c.IsFollower() {
			return c
		}
	}
	return nil
}

func BenchmarkRemoteCallSyncString(b *testing.B) {
	clusters, cleanup := buildBenchClusters(3)
	defer cleanup()

	leader := findBenchLeader(clusters)
	if leader == nil {
		b.Fatal("no leader found")
	}

	target := ""
	for _, c := range clusters {
		if c.GetMyName() != leader.GetMyName() {
			target = c.GetMyName()
			break
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := CallFuncAs[string](leader, NewFuncSpec(target, benchFuncName, "bench-param", 3*time.Second))
		if err != nil {
			b.Fatalf("remote call failed: %v", err)
		}
	}
}

func BenchmarkRemoteCallSyncBytes(b *testing.B) {
	clusters, cleanup := buildBenchClusters(3)
	defer cleanup()

	leader := findBenchLeader(clusters)
	if leader == nil {
		b.Fatal("no leader found")
	}

	target := ""
	for _, c := range clusters {
		if c.GetMyName() != leader.GetMyName() {
			target = c.GetMyName()
			break
		}
	}

	input := []byte("benchmark-payload-data")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := CallFuncAs[[]byte](leader, NewFuncSpec(target, benchFuncName, input, 3*time.Second))
		if err != nil {
			b.Fatalf("remote call failed: %v", err)
		}
	}
}

func BenchmarkRemoteCallSyncObject(b *testing.B) {
	clusters, cleanup := buildBenchClusters(3)
	defer cleanup()

	leader := findBenchLeader(clusters)
	if leader == nil {
		b.Fatal("no leader found")
	}

	target := ""
	for _, c := range clusters {
		if c.GetMyName() != leader.GetMyName() {
			target = c.GetMyName()
			break
		}
	}

	input := testUser{Name: "bench-user", Age: 25}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := CallFuncAs[testUser](leader, NewFuncSpec(target, benchFuncName, input, 3*time.Second))
		if err != nil {
			b.Fatalf("remote call failed: %v", err)
		}
	}
}

func BenchmarkRemoteCallParallel(b *testing.B) {
	clusters, cleanup := buildBenchClusters(3)
	defer cleanup()

	leader := findBenchLeader(clusters)
	if leader == nil {
		b.Fatal("no leader found")
	}

	target := ""
	for _, c := range clusters {
		if c.GetMyName() != leader.GetMyName() {
			target = c.GetMyName()
			break
		}
	}

	var ops atomic.Int64

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := CallFuncAs[string](leader, NewFuncSpec(target, benchFuncName, "parallel-param", 3*time.Second))
			if err != nil {
				b.Errorf("remote call failed: %v", err)
				return
			}
			ops.Add(1)
		}
	})
}

func BenchmarkRemoteCallConcurrent10(b *testing.B) {
	benchmarkRemoteCallConcurrentN(b, 10)
}

func BenchmarkRemoteCallConcurrent50(b *testing.B) {
	benchmarkRemoteCallConcurrentN(b, 50)
}

func BenchmarkRemoteCallConcurrent100(b *testing.B) {
	benchmarkRemoteCallConcurrentN(b, 100)
}

func benchmarkRemoteCallConcurrentN(b *testing.B, concurrency int) {
	clusters, cleanup := buildBenchClusters(3)
	defer cleanup()

	leader := findBenchLeader(clusters)
	if leader == nil {
		b.Fatal("no leader found")
	}

	target := ""
	for _, c := range clusters {
		if c.GetMyName() != leader.GetMyName() {
			target = c.GetMyName()
			break
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		var wg sync.WaitGroup
		errCh := make(chan error, concurrency)

		for j := 0; j < concurrency; j++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, err := CallFuncAs[string](leader, NewFuncSpec(target, benchFuncName, "concurrent-param", 3*time.Second))
				if err != nil {
					select {
					case errCh <- err:
					default:
					}
				}
			}()
		}

		wg.Wait()
		close(errCh)

		for err := range errCh {
			if err != nil {
				b.Fatalf("concurrent remote call failed: %v", err)
			}
		}
	}
}

func BenchmarkLocalCallSync(b *testing.B) {
	clusters, cleanup := buildBenchClusters(3)
	defer cleanup()

	follower := findBenchFollower(clusters)
	if follower == nil {
		b.Fatal("no follower found")
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := CallFuncAs[string](follower, NewFuncSpec(follower.GetMyName(), benchFuncName, "local-param", 3*time.Second))
		if err != nil {
			b.Fatalf("local call failed: %v", err)
		}
	}
}
