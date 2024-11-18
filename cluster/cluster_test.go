package cluster

import (
	"fmt"
	"testing"
	"time"

	"github.com/caiflower/common-tools/global"

	"github.com/caiflower/common-tools/pkg/logger"
)

func TestCluster(t *testing.T) {
	cluster1, cluster2, cluster3 := common()

	for i := 0; i < 3; i++ {
		fmt.Printf("模拟重启\n")
		// 模拟重启
		cluster1.Close()
		fmt.Println("localhost1 closed")
		time.Sleep(20 * time.Second)
		cluster2.Close()
		fmt.Println("localhost2 closed")
		time.Sleep(20 * time.Second)
		fmt.Println("localhost3 closed")
		cluster3.Close()

		time.Sleep(20 * time.Second)

		fmt.Println("localhost1 start")
		cluster1.StartUp()
		time.Sleep(20 * time.Second)
		fmt.Println("localhost2 start")
		cluster2.StartUp()
		time.Sleep(20 * time.Second)
		fmt.Println("localhost3 start")
		cluster3.StartUp()
		fmt.Printf("模拟重启完成\n")

		time.Sleep(50 * time.Second)

		fmt.Printf("clusterName: %s term:%d leader: %s isready: %v\n", cluster1.GetMyName(), cluster1.GetMyTerm(), cluster1.GetLeaderName(), cluster1.IsReady())
		fmt.Printf("clusterName: %s term:%d leader: %s isready: %v\n", cluster2.GetMyName(), cluster1.GetMyTerm(), cluster2.GetLeaderName(), cluster2.IsReady())
		fmt.Printf("clusterName: %s term:%d leader: %s isready: %v\n", cluster3.GetMyName(), cluster1.GetMyTerm(), cluster3.GetLeaderName(), cluster3.IsReady())
	}
}

func Test2(t *testing.T) {
	cluster1, cluster2, cluster3 := common()

	fmt.Printf("开始模拟主节点宕机，一段时间后被拉起\n")
	// 模拟主节点宕机
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
	fmt.Printf("结束模拟主节点宕机，一段时间后被拉起\n")

	time.Sleep(20 * time.Second)
	fmt.Printf("clusterName: %s term:%d leader: %s isready: %v\n", cluster1.GetMyName(), cluster1.GetMyTerm(), cluster1.GetLeaderName(), cluster1.IsReady())
	fmt.Printf("clusterName: %s term:%d leader: %s isready: %v\n", cluster2.GetMyName(), cluster1.GetMyTerm(), cluster2.GetLeaderName(), cluster2.IsReady())
	fmt.Printf("clusterName: %s term:%d leader: %s isready: %v\n", cluster3.GetMyName(), cluster1.GetMyTerm(), cluster3.GetLeaderName(), cluster3.IsReady())
}

func common() (cluster1, cluster2, cluster3 *Cluster) {
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

	c1.Nodes[0].Local = true
	cluster1, err := NewClusterWithArgs(c1, logger.NewLogger(&logger.Config{
		Level: "DebugLevel",
	}))
	if err != nil {
		panic(err)
	}

	go cluster1.StartUp()

	c2.Nodes[1].Local = true
	cluster2, err = NewClusterWithArgs(c2, logger.NewLogger(&logger.Config{
		Level: "DebugLevel",
	}))
	if err != nil {
		panic(err)
	}

	go cluster2.StartUp()

	c3.Nodes[2].Local = true
	cluster3, err = NewClusterWithArgs(c3, logger.NewLogger(&logger.Config{
		Level: "DebugLevel",
	}))
	if err != nil {
		panic(err)
	}

	go cluster3.StartUp()

	time.Sleep(10 * time.Second)

	fmt.Printf("clusterName: %s term:%d leader: %s isready: %v\n", cluster1.GetMyName(), cluster1.GetMyTerm(), cluster1.GetLeaderName(), cluster1.IsReady())
	fmt.Printf("clusterName: %s term:%d leader: %s isready: %v\n", cluster2.GetMyName(), cluster1.GetMyTerm(), cluster2.GetLeaderName(), cluster2.IsReady())
	fmt.Printf("clusterName: %s term:%d leader: %s isready: %v\n", cluster3.GetMyName(), cluster1.GetMyTerm(), cluster3.GetLeaderName(), cluster3.IsReady())
	return cluster1, cluster2, cluster3
}

func TestSignalCluster(t *testing.T) {
	common()

	global.DefaultResourceManger.Signal()
}

func TestSingleCluster(t *testing.T) {
	c1 := Config{Enable: "true", Mode: modeSingle}

	if cluster, err := NewClusterWithArgs(c1, logger.NewLogger(&logger.Config{
		Level: "Debug",
	})); err != nil {
		panic(err)
	} else {
		cluster.StartUp()
		time.Sleep(20 * time.Second)

		cluster.Close()

		time.Sleep(10 * time.Second)
	}

}
