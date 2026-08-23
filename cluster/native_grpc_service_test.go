package cluster

import (
	"context"
	"fmt"
	"runtime"
	"testing"
	"time"

	"github.com/caiflower/common-tools/cluster/proto"
	"github.com/stretchr/testify/assert"
)

type testGreeterService struct {
	proto.UnimplementedTestGreeterServer
}

func (t *testGreeterService) SayHello(ctx context.Context, req *proto.TestHelloRequest) (*proto.TestHelloResponse, error) {
	if req.Name == "error" {
		return nil, fmt.Errorf("name cannot be 'error'")
	}
	return &proto.TestHelloResponse{Message: "hello " + req.Name}, nil
}

func getOtherNodeExcludeSelf(c ICluster) string {
	for _, name := range c.GetAliveNodeNames() {
		if name != c.GetMyName() {
			return name
		}
	}
	return ""
}

func waitForClustersReady(clusters ...*Cluster) {
	for {
		allReady := true
		for _, c := range clusters {
			if !c.IsReady() {
				allReady = false
				break
			}
		}
		if !allReady {
			runtime.Gosched()
			continue
		}

		leaderConsistent := true
		termConsistent := true
		firstLeader := clusters[0].GetLeaderName()
		firstTerm := clusters[0].GetMyTerm()
		for _, c := range clusters[1:] {
			if c.GetLeaderName() != firstLeader {
				leaderConsistent = false
			}
			if c.GetMyTerm() != firstTerm {
				termConsistent = false
			}
		}
		if leaderConsistent && termConsistent {
			break
		}
		runtime.Gosched()
	}
}

func registerGreeterService(clusters ...*Cluster) error {
	for _, c := range clusters {
		if err := c.RegisterGRPCService(&proto.TestGreeter_ServiceDesc, &testGreeterService{}); err != nil {
			return err
		}
	}
	return nil
}

func startClusters(clusters ...*Cluster) {
	for _, c := range clusters {
		_ = c.Start()
	}
}

func closeClusters(clusters ...*Cluster) {
	for _, c := range clusters {
		c.Close()
	}
}

func TestRegisterGRPCServiceBeforeStart(t *testing.T) {
	cluster1, cluster2, cluster3 := common()

	registerGreeterService(cluster1, cluster2, cluster3)

	startClusters(cluster1, cluster2, cluster3)
	defer closeClusters(cluster1, cluster2, cluster3)
	waitAllForReady(t, cluster1, cluster2, cluster3)

	target := getOtherNodeExcludeSelf(cluster1)
	conn, err := cluster1.GetGRPCClient(target)
	assert.Nil(t, err)
	assert.NotNil(t, conn)

	client := proto.NewTestGreeterClient(conn)
	resp, err := client.SayHello(context.Background(), &proto.TestHelloRequest{Name: "world"})
	assert.Nil(t, err)
	assert.Equal(t, "hello world", resp.Message)
}

func TestRegisterGRPCServiceAfterStart(t *testing.T) {
	cluster1, _, _ := common()

	_ = cluster1.Start()
	defer cluster1.Close()

	err := cluster1.RegisterGRPCService(&proto.TestGreeter_ServiceDesc, &testGreeterService{})
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "cannot be registered after cluster has started")
}

func TestRegisterGRPCServiceDuplicate(t *testing.T) {
	cluster1, _, _ := common()

	err := cluster1.RegisterGRPCService(&proto.TestGreeter_ServiceDesc, &testGreeterService{})
	assert.Nil(t, err)

	err = cluster1.RegisterGRPCService(&proto.TestGreeter_ServiceDesc, &testGreeterService{})
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "already registered")
}

func TestGetGRPCClientAliveNode(t *testing.T) {
	cluster1, cluster2, cluster3 := common()

	registerGreeterService(cluster1, cluster2, cluster3)

	startClusters(cluster1, cluster2, cluster3)
	defer closeClusters(cluster1, cluster2, cluster3)
	waitAllForReady(t, cluster1, cluster2, cluster3)

	target := getOtherNodeExcludeSelf(cluster1)
	conn, err := cluster1.GetGRPCClient(target)
	assert.Nil(t, err)
	assert.NotNil(t, conn)

	client := proto.NewTestGreeterClient(conn)
	resp, err := client.SayHello(context.Background(), &proto.TestHelloRequest{Name: "native-grpc"})
	assert.Nil(t, err)
	assert.Equal(t, "hello native-grpc", resp.Message)
}

func TestGetGRPCClientNonExistentNode(t *testing.T) {
	cluster1, cluster2, cluster3 := common()
	startClusters(cluster1, cluster2, cluster3)
	defer closeClusters(cluster1, cluster2, cluster3)
	waitAllForReady(t, cluster1, cluster2, cluster3)

	conn, err := cluster1.GetGRPCClient("non-existent-node")
	assert.NotNil(t, err)
	assert.Nil(t, conn)
	assert.Contains(t, err.Error(), "does not exist")
}

func TestGetGRPCClientUnreachableNode(t *testing.T) {
	cluster1, cluster2, cluster3 := common()
	registerGreeterService(cluster1, cluster2, cluster3)
	startClusters(cluster1, cluster2, cluster3)
	waitAllForReady(t, cluster1, cluster2, cluster3)

	target := getOtherNodeExcludeSelf(cluster1)
	conn, err := cluster1.GetGRPCClient(target)
	assert.Nil(t, err)
	assert.NotNil(t, conn)

	client := proto.NewTestGreeterClient(conn)
	resp, err := client.SayHello(context.Background(), &proto.TestHelloRequest{Name: "before-close"})
	assert.Nil(t, err)
	assert.Equal(t, "hello before-close", resp.Message)

	for _, c := range []*Cluster{cluster1, cluster2, cluster3} {
		if c.GetMyName() == target {
			c.Close()
			break
		}
	}

	_, err = client.SayHello(context.Background(), &proto.TestHelloRequest{Name: "after-close"})
	assert.NotNil(t, err)

	for _, c := range []*Cluster{cluster1, cluster2, cluster3} {
		if c.GetMyName() == cluster1.GetMyName() {
			continue
		}
		if !c.IsClosed() {
			c.Close()
		}
	}
}

func TestGetGRPCClientSelfNode(t *testing.T) {
	cluster1, cluster2, cluster3 := common()
	startClusters(cluster1, cluster2, cluster3)
	defer closeClusters(cluster1, cluster2, cluster3)
	waitAllForReady(t, cluster1, cluster2, cluster3)

	conn, err := cluster1.GetGRPCClient(cluster1.GetMyName())
	assert.NotNil(t, err)
	assert.Nil(t, conn)
	assert.Contains(t, err.Error(), "self node")
}

func BenchmarkNativeGRPCCall(b *testing.B) {
	cluster1, cluster2, cluster3 := common()

	if err := registerGreeterService(cluster1, cluster2, cluster3); err != nil {
		b.Fatal(err)
	}

	startClusters(cluster1, cluster2, cluster3)
	defer closeClusters(cluster1, cluster2, cluster3)
	waitForClustersReady(cluster1, cluster2, cluster3)

	target := getOtherNodeExcludeSelf(cluster1)
	conn, err := cluster1.GetGRPCClient(target)
	if err != nil {
		b.Fatal(err)
	}
	client := proto.NewTestGreeterClient(conn)

	b.ReportAllocs()

	for b.Loop() {
		_, err := client.SayHello(context.Background(), &proto.TestHelloRequest{Name: "bench"})
		if err != nil {
			b.Fatalf("native gRPC call failed: %v", err)
		}
	}
}

func BenchmarkCallFuncAsVsNativeGRPC(b *testing.B) {
	cluster1, cluster2, cluster3 := common()

	cluster1.RegisterFunc(benchFuncName, func(data interface{}) (interface{}, error) { return data, nil })
	cluster2.RegisterFunc(benchFuncName, func(data interface{}) (interface{}, error) { return data, nil })
	cluster3.RegisterFunc(benchFuncName, func(data interface{}) (interface{}, error) { return data, nil })

	if err := registerGreeterService(cluster1, cluster2, cluster3); err != nil {
		b.Fatal(err)
	}

	startClusters(cluster1, cluster2, cluster3)
	defer closeClusters(cluster1, cluster2, cluster3)
	waitForClustersReady(cluster1, cluster2, cluster3)

	target := getOtherNodeExcludeSelf(cluster1)

	conn, err := cluster1.GetGRPCClient(target)
	if err != nil {
		b.Fatal(err)
	}
	nativeClient := proto.NewTestGreeterClient(conn)

	b.Run("CallFuncAs", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, err := CallFuncAs[string](cluster1, NewFuncSpec(target, benchFuncName, "bench", 3*time.Second))
			if err != nil {
				b.Fatalf("CallFuncAs failed: %v", err)
			}
		}
	})

	b.Run("NativeGRPC", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, err := nativeClient.SayHello(context.Background(), &proto.TestHelloRequest{Name: "bench"})
			if err != nil {
				b.Fatalf("native gRPC call failed: %v", err)
			}
		}
	})
}
