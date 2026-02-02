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
	"testing"
	"time"

	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/stretchr/testify/assert"
)

type TestJobTracker struct {
	Cluster           ICluster
	name              string
	leaderName        string
	leaderStartTime   time.Time
	leaderEndTime     time.Time
	followerStartTime time.Time
	followerEndTime   time.Time
}

func (t *TestJobTracker) Name() string {
	return t.name
}

func (t *TestJobTracker) OnStartedLeading() {
	t.leaderStartTime = time.Now()
}

func (t *TestJobTracker) OnStoppedLeading() {
	t.leaderEndTime = time.Now()
}

func (t *TestJobTracker) OnStoppedFollowing() {
	t.followerEndTime = time.Now()
}

func (t *TestJobTracker) OnStartedFollowing(leaderName string) {
	t.leaderName = leaderName
	t.followerStartTime = time.Now()
}

func TestClusterJobTracker(t *testing.T) {
	var cluster1, cluster2, cluster3 *Cluster

	c1 := Config{Enable: "true"}
	c2 := Config{Enable: "true"}
	c3 := Config{Enable: "true"}

	c1.Nodes = append(c1.Nodes,
		&struct {
			Name  string
			Ip    string
			Port  int
			Local bool
		}{
			Ip:   "127.0.0.1",
			Name: "localhost1",
			Port: 8088,
		},
		&struct {
			Name  string
			Ip    string
			Port  int
			Local bool
		}{
			Ip:   "127.0.0.1",
			Name: "localhost2",
			Port: 8089,
		}, &struct {
			Name  string
			Ip    string
			Port  int
			Local bool
		}{
			Ip:   "127.0.0.1",
			Name: "localhost3",
			Port: 8090,
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
			Port: 8088,
		},
		&struct {
			Name  string
			Ip    string
			Port  int
			Local bool
		}{
			Ip:   "127.0.0.1",
			Name: "localhost2",
			Port: 8089,
		}, &struct {
			Name  string
			Ip    string
			Port  int
			Local bool
		}{
			Ip:   "127.0.0.1",
			Name: "localhost3",
			Port: 8090,
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
			Port: 8088,
		},
		&struct {
			Name  string
			Ip    string
			Port  int
			Local bool
		}{
			Ip:   "127.0.0.1",
			Name: "localhost2",
			Port: 8089,
		}, &struct {
			Name  string
			Ip    string
			Port  int
			Local bool
		}{
			Ip:   "127.0.0.1",
			Name: "localhost3",
			Port: 8090,
		})

	var t1, t2, t3 TestJobTracker

	c1.Nodes[0].Local = true
	cluster1, err := NewClusterWithArgs(c1, logger.NewLogger(&logger.Config{
		Level: "INFO",
	}))
	if err != nil {
		panic(err)
	}
	t1.name = "t1"
	t1.Cluster = cluster1
	_ = cluster1.AddJobTracker(&t1)

	_ = cluster1.Start()
	defer cluster1.Close()

	c2.Nodes[1].Local = true
	cluster2, err = NewClusterWithArgs(c2, logger.NewLogger(&logger.Config{
		Level: "INFO",
	}))
	if err != nil {
		panic(err)
	}
	t2.name = "t2"
	t2.Cluster = cluster2
	_ = cluster2.AddJobTracker(&t2)

	_ = cluster2.Start()
	defer cluster2.Close()

	c3.Nodes[2].Local = true
	cluster3, err = NewClusterWithArgs(c3, logger.NewLogger(&logger.Config{
		Level: "INFO",
	}))
	if err != nil {
		panic(err)
	}
	t3.name = "t3"
	t3.Cluster = cluster3
	_ = cluster3.AddJobTracker(&t3)

	_ = cluster3.Start()
	defer cluster3.Close()

	for {
		if cluster1.IsReady() && cluster2.IsReady() && cluster3.IsReady() {
			break
		}
	}

	assert.Equal(t, cluster1.GetLeaderName(), cluster3.GetLeaderName())
	assert.Equal(t, cluster1.GetLeaderName(), cluster2.GetLeaderName())
	assert.Equal(t, cluster1.GetMyTerm(), cluster2.GetMyTerm())
	assert.Equal(t, cluster1.GetMyTerm(), cluster2.GetMyTerm())

	judge := func(leader, f1, f2 TestJobTracker) {
		assert.Equal(t, false, leader.leaderStartTime.IsZero())
		assert.Equal(t, true, leader.leaderEndTime.IsZero())
		assert.Equal(t, true, leader.followerStartTime.IsZero())
		assert.Equal(t, true, leader.followerEndTime.IsZero())

		assert.Equal(t, true, f1.leaderStartTime.IsZero())
		assert.Equal(t, true, f1.leaderEndTime.IsZero())
		assert.Equal(t, false, f1.followerStartTime.IsZero())

		assert.Equal(t, true, f2.leaderStartTime.IsZero())
		assert.Equal(t, true, f2.leaderEndTime.IsZero())
		assert.Equal(t, false, f2.followerStartTime.IsZero())
	}

	switch cluster1.GetLeaderName() {
	case "localhost1":
		assert.Equal(t, t2.leaderName, "localhost1")
		assert.Equal(t, t3.leaderName, "localhost1")

		time.Sleep(5 * time.Second)
		judge(t1, t2, t3)

		cluster1.Close()

		time.Sleep(10 * time.Second)
		assert.Equal(t, true, t1.leaderEndTime.After(t1.leaderStartTime))
		_ = cluster1.Start()
	case "localhost2":
		assert.Equal(t, t1.leaderName, "localhost2")
		assert.Equal(t, t3.leaderName, "localhost2")

		time.Sleep(5 * time.Second)
		judge(t2, t1, t3)

		cluster2.Close()

		time.Sleep(10 * time.Second)
		assert.Equal(t, true, t2.leaderEndTime.After(t2.leaderStartTime))
		_ = cluster2.Start()
	case "localhost3":
		assert.Equal(t, t1.leaderName, "localhost3")
		assert.Equal(t, t2.leaderName, "localhost3")

		time.Sleep(5 * time.Second)
		judge(t3, t1, t2)

		cluster3.Close()

		time.Sleep(10 * time.Second)
		assert.Equal(t, true, t3.leaderEndTime.After(t3.leaderStartTime))
		_ = cluster3.Start()
	}

	for {
		if cluster1.IsReady() && cluster2.IsReady() && cluster3.IsReady() {
			break
		}
	}

	assert.Equal(t, cluster1.GetLeaderName(), cluster3.GetLeaderName())
	assert.Equal(t, cluster1.GetLeaderName(), cluster2.GetLeaderName())
	assert.Equal(t, cluster1.GetMyTerm(), cluster2.GetMyTerm())
	assert.Equal(t, cluster1.GetMyTerm(), cluster2.GetMyTerm())
}

type TestCaller struct {
	Name string
}

func (t *TestCaller) MasterCall() {
	fmt.Printf("%s MasterCall time: %s \n", t.Name, time.Now().Format("2006-01-02 15:04:05"))
}

func (t *TestCaller) OnStoppedLeading() {

}
func (t *TestCaller) OnStartedLeading() {

}

func (t *TestCaller) OnStoppedFollowing() {}

func (t *TestCaller) OnStartedFollowing(leaderName string) {

}

func (t *TestCaller) SlaverCall(leaderName string) {
	fmt.Printf("%s SlaverCall time: %s \n", t.Name, time.Now().Format("2006-01-02 15:04:05"))
}

func TestDefaultJobTracker(t *testing.T) {
	c1 := Config{Enable: "true"}
	c2 := Config{Enable: "true"}
	c3 := Config{Enable: "true"}

	c1.Nodes = append(c1.Nodes,
		&struct {
			Name  string
			Ip    string
			Port  int
			Local bool
		}{
			Ip:   "127.0.0.1",
			Name: "localhost1",
			Port: 8090,
		},
		&struct {
			Name  string
			Ip    string
			Port  int
			Local bool
		}{
			Ip:   "127.0.0.1",
			Name: "localhost2",
			Port: 8091,
		}, &struct {
			Name  string
			Ip    string
			Port  int
			Local bool
		}{
			Ip:   "127.0.0.1",
			Name: "localhost3",
			Port: 8092,
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
			Port: 8090,
		},
		&struct {
			Name  string
			Ip    string
			Port  int
			Local bool
		}{
			Ip:   "127.0.0.1",
			Name: "localhost2",
			Port: 8091,
		}, &struct {
			Name  string
			Ip    string
			Port  int
			Local bool
		}{
			Ip:   "127.0.0.1",
			Name: "localhost3",
			Port: 8092,
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
			Port: 8090,
		},
		&struct {
			Name  string
			Ip    string
			Port  int
			Local bool
		}{
			Ip:   "127.0.0.1",
			Name: "localhost2",
			Port: 8091,
		}, &struct {
			Name  string
			Ip    string
			Port  int
			Local bool
		}{
			Ip:   "127.0.0.1",
			Name: "localhost3",
			Port: 8092,
		})

	testCaller1 := &TestCaller{
		Name: "Localhost1",
	}
	c1.Nodes[0].Local = true

	cluster1, err := NewCluster(c1)
	if err != nil {
		panic(err)
	}
	tracker1 := NewDefaultJobTracker(10, testCaller1)
	_ = cluster1.AddJobTracker(tracker1)
	_ = cluster1.Start()
	defer cluster1.Close()

	testCaller2 := &TestCaller{
		Name: "Localhost2",
	}
	c2.Nodes[1].Local = true

	cluster2, err := NewCluster(c2)
	if err != nil {
		panic(err)
	}
	tracker2 := NewDefaultJobTracker(10, testCaller2)
	_ = cluster2.AddJobTracker(tracker2)
	_ = cluster2.Start()
	defer cluster2.Close()

	testCaller3 := &TestCaller{
		Name: "Localhost3",
	}
	c3.Nodes[2].Local = true
	cluster3, err := NewCluster(c3)
	if err != nil {
		panic(err)
	}
	tracker3 := NewDefaultJobTracker(10, testCaller3)
	_ = cluster3.AddJobTracker(tracker3)
	_ = cluster3.Start()
	defer cluster3.Close()

	time.Sleep(20 * time.Second)
}
