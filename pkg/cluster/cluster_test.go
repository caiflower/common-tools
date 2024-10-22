package cluster

import (
	"fmt"
	"github.com/caiflower/common-tools/pkg/logger"
	"testing"
	"time"
)

func TestCluster(t *testing.T) {

	c := &Config{}

	c.Nodes = append(c.Nodes,
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

	c.Nodes[0].Local = true
	cluster1, err := NewClusterWithArgs(c, logger.NewLogger(&logger.Config{
		Level: "DebugLevel",
	}))
	if err != nil {
		panic(err)
	}

	go cluster1.StartUp()

	c.Nodes[0].Local = false
	c.Nodes[1].Local = true
	cluster2, err := NewClusterWithArgs(c, logger.NewLogger(&logger.Config{
		Level: "DebugLevel",
	}))
	if err != nil {
		panic(err)
	}

	go cluster2.StartUp()

	c.Nodes[0].Local = false
	c.Nodes[1].Local = false
	c.Nodes[2].Local = true
	cluster3, err := NewClusterWithArgs(c, logger.NewLogger(&logger.Config{
		Level: "DebugLevel",
	}))
	if err != nil {
		panic(err)
	}

	go cluster3.StartUp()

	time.Sleep(50 * time.Second)

	fmt.Printf("clusterName: %s term:%d leader: %s\n", cluster1.GetMyName(), cluster1.GetMyTerm(), cluster1.GetLeaderName())
	fmt.Printf("clusterName: %s term:%d leader: %s\n", cluster2.GetMyName(), cluster1.GetMyTerm(), cluster1.GetLeaderName())
	fmt.Printf("clusterName: %s term:%d leader: %s\n", cluster3.GetMyName(), cluster1.GetMyTerm(), cluster1.GetLeaderName())
}
