package cluster

import (
	"fmt"
	"testing"
	"time"

	"github.com/caiflower/common-tools/pkg/logger"
)

type TestJobTracker struct {
	Cluster ICluster
	name    string
}

func (t *TestJobTracker) Name() string {
	return t.name
}

func (t *TestJobTracker) OnStartedLeading() {
	fmt.Println("leader start")
}

func (t *TestJobTracker) OnStoppedLeading() {
	fmt.Println("leader stop")
}

func (t *TestJobTracker) OnReleaseMaster() {
	fmt.Println("release master")
}

func (t *TestJobTracker) OnNewLeader(leaderName string) {
	fmt.Println("new leader", leaderName)
}

func TestClusterJobTracker(t *testing.T) {
	var cluster1, cluster2, cluster3 *Cluster

	c1 := &Config{Enable: "true"}
	c2 := &Config{Enable: "true"}
	c3 := &Config{Enable: "true"}

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
	cluster1.AddJobTracker(&t1)

	go cluster1.StartUp()

	c2.Nodes[1].Local = true
	cluster2, err = NewClusterWithArgs(c2, logger.NewLogger(&logger.Config{
		Level: "INFO",
	}))
	if err != nil {
		panic(err)
	}
	t2.name = "t2"
	t2.Cluster = cluster2
	cluster2.AddJobTracker(&t2)

	go cluster2.StartUp()

	c3.Nodes[2].Local = true
	cluster3, err = NewClusterWithArgs(c3, logger.NewLogger(&logger.Config{
		Level: "INFO",
	}))
	if err != nil {
		panic(err)
	}
	t3.name = "t3"
	t3.Cluster = cluster3
	cluster3.AddJobTracker(&t3)

	go cluster3.StartUp()

	time.Sleep(15 * time.Second)

	fmt.Printf("clusterName: %s term:%d leader: %s isready: %v\n", cluster1.GetMyName(), cluster1.GetMyTerm(), cluster1.GetLeaderName(), cluster1.IsReady())
	fmt.Printf("clusterName: %s term:%d leader: %s isready: %v\n", cluster2.GetMyName(), cluster1.GetMyTerm(), cluster2.GetLeaderName(), cluster2.IsReady())
	fmt.Printf("clusterName: %s term:%d leader: %s isready: %v\n", cluster3.GetMyName(), cluster1.GetMyTerm(), cluster3.GetLeaderName(), cluster3.IsReady())

	switch cluster1.GetLeaderName() {
	case "localhost1":
		fmt.Printf("localhost1 is closed\n")
		cluster1.Close()
		time.Sleep(20 * time.Second)
		fmt.Printf("localhost1 is start\n")
		cluster1.StartUp()
	case "localhost2":
		fmt.Printf("localhost2 is closed\n")
		cluster2.Close()
		time.Sleep(20 * time.Second)
		fmt.Printf("localhost2 is start\n")
		cluster2.StartUp()
	case "localhost3":
		fmt.Printf("localhost3 is closed\n")
		cluster3.Close()
		time.Sleep(20 * time.Second)
		fmt.Printf("localhost3 is start\n")
		cluster3.StartUp()
	default:
	}

	time.Sleep(20 * time.Second)

	fmt.Printf("clusterName: %s term:%d leader: %s isready: %v\n", cluster1.GetMyName(), cluster1.GetMyTerm(), cluster1.GetLeaderName(), cluster1.IsReady())
	fmt.Printf("clusterName: %s term:%d leader: %s isready: %v\n", cluster2.GetMyName(), cluster1.GetMyTerm(), cluster2.GetLeaderName(), cluster2.IsReady())
	fmt.Printf("clusterName: %s term:%d leader: %s isready: %v\n", cluster3.GetMyName(), cluster1.GetMyTerm(), cluster3.GetLeaderName(), cluster3.IsReady())
}
