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
	"testing"
	"time"

	redisv2 "github.com/caiflower/common-tools/redis/v2"
	"github.com/stretchr/testify/assert"

	"github.com/caiflower/common-tools/pkg/logger"
)

func common() (cluster1, cluster2, cluster3 *Cluster) {
	var err error
	c1 := Config{}
	c2 := Config{}
	c3 := Config{}

	rand.Seed(time.Now().UnixNano())
	port1 := rand.Intn(10000) + 8000
	port2 := rand.Intn(10000) + 8000
	port3 := rand.Intn(10000) + 8000
	c1.Nodes = append(c1.Nodes,
		&struct {
			Name  string
			Ip    string
			Port  int
			Local bool
		}{
			Ip:   "127.0.0.1",
			Name: "localhost1",
			Port: port1,
		},
		&struct {
			Name  string
			Ip    string
			Port  int
			Local bool
		}{
			Ip:   "127.0.0.1",
			Name: "localhost2",
			Port: port2,
		}, &struct {
			Name  string
			Ip    string
			Port  int
			Local bool
		}{
			Ip:   "127.0.0.1",
			Name: "localhost3",
			Port: port3,
		})

	c2.Nodes = append(c2.Nodes,
		&struct {
			Name  string
			Ip    string
			Port  int
			Local bool
		}{
			Ip:   "127.0.0.1",
			Name: "localhost1",
			Port: port1,
		},
		&struct {
			Name  string
			Ip    string
			Port  int
			Local bool
		}{
			Ip:   "127.0.0.1",
			Name: "localhost2",
			Port: port2,
		}, &struct {
			Name  string
			Ip    string
			Port  int
			Local bool
		}{
			Ip:   "127.0.0.1",
			Name: "localhost3",
			Port: port3,
		})

	c3.Nodes = append(c3.Nodes,
		&struct {
			Name  string
			Ip    string
			Port  int
			Local bool
		}{
			Ip:   "127.0.0.1",
			Name: "localhost1",
			Port: port1,
		},
		&struct {
			Name  string
			Ip    string
			Port  int
			Local bool
		}{
			Ip:   "127.0.0.1",
			Name: "localhost2",
			Port: port2,
		}, &struct {
			Name  string
			Ip    string
			Port  int
			Local bool
		}{
			Ip:   "127.0.0.1",
			Name: "localhost3",
			Port: port3,
		})

	c1.Nodes[0].Local = true
	cluster1, err = NewClusterWithArgs(c1, logger.NewLogger(&logger.Config{
		Level: "FATAL",
	}))
	if err != nil {
		panic(err)
	}

	c2.Nodes[1].Local = true
	cluster2, err = NewClusterWithArgs(c2, logger.NewLogger(&logger.Config{
		Level: "FATAL",
	}))
	if err != nil {
		panic(err)
	}

	c3.Nodes[2].Local = true
	cluster3, err = NewClusterWithArgs(c3, logger.NewLogger(&logger.Config{
		Level: "FATAL",
	}))
	if err != nil {
		panic(err)
	}

	return cluster1, cluster2, cluster3
}

func TestSingleCluster(t *testing.T) {
	c1 := Config{Enable: "true", Mode: modeSingle}

	if cluster, err := NewClusterWithArgs(c1, logger.NewLogger(&logger.Config{
		Level: "TRACE",
	})); err != nil {
		panic(err)
	} else {
		err = cluster.Start()
		assert.Nil(t, err)
		time.Sleep(2 * time.Second)
		cluster.Close()
	}
}

func redisCommon() (cluster1, cluster2, cluster3 *Cluster) {
	redisClient, err := redisv2.NewRedisClient(redisv2.Config{
		Addrs:    []string{"redis-master.app.svc.cluster.local:6379"},
		Password: "",
		DB:       0,
	})
	if err != nil {
		panic(err)
	}

	redisDiscovery := RedisDiscovery{
		DataPath:           "/test/redis",
		ElectionInterval:   5 * time.Second,
		ElectionPeriod:     10 * time.Second,
		SyncLeaderInterval: 5 * time.Second,
	}

	c1 := Config{Mode: "redis", Enable: "true", RedisDiscovery: redisDiscovery}
	c2 := Config{Mode: "redis", Enable: "true", RedisDiscovery: redisDiscovery}
	c3 := Config{Mode: "redis", Enable: "true", RedisDiscovery: redisDiscovery}

	c1.Nodes = append(c1.Nodes,
		&struct {
			Name  string
			Ip    string
			Port  int
			Local bool
		}{
			Ip:   "127.0.0.1",
			Name: "localhost1",
			Port: 8080,
		},
		&struct {
			Name  string
			Ip    string
			Port  int
			Local bool
		}{
			Ip:   "127.0.0.1",
			Name: "localhost2",
			Port: 8081,
		}, &struct {
			Name  string
			Ip    string
			Port  int
			Local bool
		}{
			Ip:   "127.0.0.1",
			Name: "localhost3",
			Port: 8082,
		})

	c2.Nodes = append(c2.Nodes,
		&struct {
			Name  string
			Ip    string
			Port  int
			Local bool
		}{
			Ip:   "127.0.0.1",
			Name: "localhost1",
			Port: 8080,
		},
		&struct {
			Name  string
			Ip    string
			Port  int
			Local bool
		}{
			Ip:   "127.0.0.1",
			Name: "localhost2",
			Port: 8081,
		}, &struct {
			Name  string
			Ip    string
			Port  int
			Local bool
		}{
			Ip:   "127.0.0.1",
			Name: "localhost3",
			Port: 8082,
		})

	c3.Nodes = append(c3.Nodes,
		&struct {
			Name  string
			Ip    string
			Port  int
			Local bool
		}{
			Ip:   "127.0.0.1",
			Name: "localhost1",
			Port: 8080,
		},
		&struct {
			Name  string
			Ip    string
			Port  int
			Local bool
		}{
			Ip:   "127.0.0.1",
			Name: "localhost2",
			Port: 8081,
		}, &struct {
			Name  string
			Ip    string
			Port  int
			Local bool
		}{
			Ip:   "127.0.0.1",
			Name: "localhost3",
			Port: 8082,
		})

	c1.Nodes[0].Local = true
	cluster1, err = NewClusterWithArgs(c1, logger.NewLogger(&logger.Config{
		Level: "TRACE",
	}))
	if err != nil {
		panic(err)
	}

	cluster1.Redis = redisClient
	_ = cluster1.Start()

	c2.Nodes[1].Local = true
	cluster2, err = NewClusterWithArgs(c2, logger.NewLogger(&logger.Config{
		Level: "TRACE",
	}))
	if err != nil {
		panic(err)
	}

	cluster2.Redis = redisClient
	_ = cluster2.Start()

	c3.Nodes[2].Local = true
	cluster3, err = NewClusterWithArgs(c3, logger.NewLogger(&logger.Config{
		Level: "TRACE",
	}))
	if err != nil {
		panic(err)
	}

	cluster3.Redis = redisClient
	_ = cluster3.Start()

	time.Sleep(10 * time.Second)

	fmt.Printf("clusterName: %s term:%d leader: %s isready: %v\n", cluster1.GetMyName(), cluster1.GetMyTerm(), cluster1.GetLeaderName(), cluster1.IsReady())
	fmt.Printf("clusterName: %s term:%d leader: %s isready: %v\n", cluster2.GetMyName(), cluster1.GetMyTerm(), cluster2.GetLeaderName(), cluster2.IsReady())
	fmt.Printf("clusterName: %s term:%d leader: %s isready: %v\n", cluster3.GetMyName(), cluster1.GetMyTerm(), cluster3.GetLeaderName(), cluster3.IsReady())
	return cluster1, cluster2, cluster3
}

func TestDisable(t *testing.T) {
	c1 := Config{Enable: "false"}
	c, err := NewCluster(c1)
	assert.Nil(t, err, "new cluster err expected nil")
	err = c.Start()
	assert.Nil(t, err, "start cluster err expected nil")
	c.Close()
}

func TestMockApplication(t *testing.T) {
	cluster1, cluster2, cluster3 := common()
	_ = cluster1.Start()
	_ = cluster2.Start()
	_ = cluster3.Start()
	waitAllForReady(t, cluster1, cluster2, cluster3)
	defer cluster1.Close()
	defer cluster2.Close()
	defer cluster3.Close()

	waitForReady := func(c1, c2 *Cluster) {
		for !c1.IsReady() || !c2.IsReady() {
			time.Sleep(200 * time.Microsecond)
		}
		assert.Equal(t, true, c1.GetLeaderName() == c2.GetLeaderName())
		assert.Equal(t, true, c1.GetMyTerm() == c2.GetMyTerm())
	}

	waitNextTerm := func(oldTerm int, c1, c2 *Cluster) {
		for c1.GetMyTerm() <= oldTerm || c2.GetMyTerm() <= oldTerm {
			time.Sleep(200 * time.Microsecond)
		}
	}

	if cluster1.IsLeader() {
		cluster1.Close()
		waitNextTerm(cluster1.GetMyTerm(), cluster2, cluster3)
		waitForReady(cluster2, cluster3)
		_ = cluster1.Start()
	} else if cluster2.IsLeader() {
		cluster2.Close()
		waitNextTerm(cluster2.GetMyTerm(), cluster1, cluster3)
		waitForReady(cluster1, cluster3)
		_ = cluster2.Start()
	} else {
		cluster3.Close()
		waitNextTerm(cluster3.GetMyTerm(), cluster1, cluster2)
		waitForReady(cluster1, cluster2)
		_ = cluster3.Start()
	}

	waitAllForReady(t, cluster1, cluster2, cluster3)
}

func waitAllForReady(t *testing.T, cluster1, cluster2, cluster3 *Cluster) {
	for {
		if cluster1.IsReady() && cluster2.IsReady() && cluster3.IsReady() {
			break
		}
	}

	assert.Equal(t, true, cluster1.GetLeaderName() == cluster2.GetLeaderName())
	assert.Equal(t, true, cluster1.GetLeaderName() == cluster3.GetLeaderName())
	assert.Equal(t, true, cluster1.GetMyTerm() == cluster2.GetMyTerm())
	assert.Equal(t, true, cluster1.GetMyTerm() == cluster3.GetMyTerm())
}
