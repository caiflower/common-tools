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

type TestCaller struct {
	Name string
}

func (t *TestCaller) MasterCall() {
	fmt.Printf("%s MasterCall time: %s \n", t.Name, time.Now().Format("2006-01-02 15:04:05"))
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

	testCaller1 := &TestCaller{
		Name: "Localhost1",
	}
	c1.Nodes[0].Local = true

	cluster1, err := NewCluster(c1)
	if err != nil {
		panic(err)
	}
	tracker1 := NewDefaultJobTracker(10, cluster1, testCaller1)
	go cluster1.StartUp()
	go tracker1.Start()

	testCaller2 := &TestCaller{
		Name: "Localhost2",
	}
	c2.Nodes[1].Local = true

	cluster2, err := NewCluster(c2)
	if err != nil {
		panic(err)
	}
	tracker2 := NewDefaultJobTracker(10, cluster2, testCaller2)
	go cluster2.StartUp()
	go tracker2.Start()

	testCaller3 := &TestCaller{
		Name: "Localhost3",
	}
	c3.Nodes[2].Local = true
	cluster3, err := NewCluster(c3)
	if err != nil {
		panic(err)
	}
	tracker3 := NewDefaultJobTracker(10, cluster3, testCaller3)

	go cluster3.StartUp()
	go tracker3.Start()

	time.Sleep(60 * time.Second)
}
