package cluster

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/caiflower/common-tools/cluster/proto"
	golocalv1 "github.com/caiflower/common-tools/pkg/golocal/v1"
	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/stretchr/testify/assert"
)

type testGreeterServer struct {
	proto.UnimplementedTestGreeterServer
}

func (s *testGreeterServer) SayHello(ctx context.Context, req *proto.TestHelloRequest) (*proto.TestHelloResponse, error) {
	traceId := golocalv1.GetTraceID()
	return &proto.TestHelloResponse{
		Message: fmt.Sprintf("Hello, %s!", req.Name),
		TraceId: traceId,
	}, nil
}

func buildCommonClustersWithGreeter(traceIdKey string) (cluster1, cluster2, cluster3 *Cluster) {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	port1 := rng.Intn(10000) + 8000
	port2 := rng.Intn(10000) + 8000
	port3 := rng.Intn(10000) + 8000

	node1 := &struct {
		Name  string
		Ip    string
		Port  int
		Local bool
	}{Ip: "127.0.0.1", Name: "localhost1", Port: port1}
	node2 := &struct {
		Name  string
		Ip    string
		Port  int
		Local bool
	}{Ip: "127.0.0.1", Name: "localhost2", Port: port2}
	node3 := &struct {
		Name  string
		Ip    string
		Port  int
		Local bool
	}{Ip: "127.0.0.1", Name: "localhost3", Port: port3}

	log := logger.NewLogger(&logger.Config{Level: "FATAL"})

	makeConfig := func(localIdx int) Config {
		cfg := Config{Enable: "true", Mode: modeCluster, TraceIdKey: traceIdKey}
		nodes := []*struct {
			Name  string
			Ip    string
			Port  int
			Local bool
		}{
			{Ip: node1.Ip, Name: node1.Name, Port: node1.Port, Local: localIdx == 0},
			{Ip: node2.Ip, Name: node2.Name, Port: node2.Port, Local: localIdx == 1},
			{Ip: node3.Ip, Name: node3.Name, Port: node3.Port, Local: localIdx == 2},
		}
		cfg.Nodes = nodes
		return cfg
	}

	cluster1, _ = NewClusterWithArgs(makeConfig(0), log)
	cluster2, _ = NewClusterWithArgs(makeConfig(1), log)
	cluster3, _ = NewClusterWithArgs(makeConfig(2), log)

	_ = cluster2.RegisterGRPCService(&proto.TestGreeter_ServiceDesc, &testGreeterServer{})
	return
}

func TestTraceIdPropagationViaInterceptor(t *testing.T) {
	cluster1, cluster2, cluster3 := buildCommonClustersWithGreeter("")
	_ = cluster1.Start()
	_ = cluster2.Start()
	_ = cluster3.Start()
	defer cluster1.Close()
	defer cluster2.Close()
	defer cluster3.Close()
	waitAllForReady(t, cluster1, cluster2, cluster3)

	conn, err := cluster1.GetGRPCClient(cluster2.GetMyName())
	assert.Nil(t, err, "GetGRPCClient should succeed")

	client := proto.NewTestGreeterClient(conn)

	golocalv1.PutTraceID("test-trace-123")
	defer golocalv1.Clean()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	resp, err := client.SayHello(ctx, &proto.TestHelloRequest{Name: "World"})
	assert.Nil(t, err, "SayHello should succeed")
	assert.Equal(t, "Hello, World!", resp.GetMessage())
	assert.Equal(t, "test-trace-123", resp.GetTraceId(), "traceId should be propagated from client to server via gRPC metadata")
}

func TestTraceIdPropagationWithCustomKey(t *testing.T) {
	cluster1, cluster2, cluster3 := buildCommonClustersWithGreeter("x-custom-trace-id")
	_ = cluster1.Start()
	_ = cluster2.Start()
	_ = cluster3.Start()
	defer cluster1.Close()
	defer cluster2.Close()
	defer cluster3.Close()
	waitAllForReady(t, cluster1, cluster2, cluster3)

	conn, err := cluster1.GetGRPCClient(cluster2.GetMyName())
	assert.Nil(t, err, "GetGRPCClient should succeed")

	client := proto.NewTestGreeterClient(conn)

	golocalv1.PutTraceID("custom-key-trace-456")
	defer golocalv1.Clean()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	resp, err := client.SayHello(ctx, &proto.TestHelloRequest{Name: "CustomKey"})
	assert.Nil(t, err, "SayHello should succeed")
	assert.Equal(t, "custom-key-trace-456", resp.GetTraceId(), "traceId should be propagated with custom metadata key")
}

func TestTraceIdPropagationViaRemoteCall(t *testing.T) {
	cluster1, cluster2, cluster3 := common()
	cluster1.RegisterFunc("traceFunc", func(data interface{}) (interface{}, error) {
		traceId := golocalv1.GetTraceID()
		return traceId, nil
	})
	cluster2.RegisterFunc("traceFunc", func(data interface{}) (interface{}, error) {
		traceId := golocalv1.GetTraceID()
		return traceId, nil
	})
	cluster3.RegisterFunc("traceFunc", func(data interface{}) (interface{}, error) {
		traceId := golocalv1.GetTraceID()
		return traceId, nil
	})
	_ = cluster1.Start()
	_ = cluster2.Start()
	_ = cluster3.Start()
	defer cluster1.Close()
	defer cluster2.Close()
	defer cluster3.Close()
	waitAllForReady(t, cluster1, cluster2, cluster3)

	clusters := []*Cluster{cluster1, cluster2, cluster3}
	var caller *Cluster
	var targetName string
	for i, c := range clusters {
		if !c.IsLeader() {
			for j, other := range clusters {
				if i != j {
					caller = c
					targetName = other.GetMyName()
					break
				}
			}
			break
		}
	}
	if caller == nil {
		t.Fatal("no follower found to test remote call")
	}

	result, err := CallFuncAs[string](caller, NewFuncSpec(targetName, "traceFunc", nil, time.Second*3).SetTraceId("remote-trace-789"))
	assert.Nil(t, err)
	assert.Equal(t, "remote-trace-789", result, "traceId should be propagated via RemoteCall")
}
