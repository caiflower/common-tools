package cluster

import (
	"context"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/caiflower/common-tools/cluster/proto"
	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func TestClusterGetQuorumCountsAliveNodesOutsideMembership(t *testing.T) {
	c := &Cluster{
		allNode:    &sync.Map{},
		aliveNodes: &sync.Map{},
	}
	c.allNode.Store("node-0", newNode("127.0.0.1:9000", "node-0", 1))
	c.aliveNodes.Store("node-0", newNode("127.0.0.1:9000", "node-0", 1))
	c.aliveNodes.Store("node-1", newNode("127.0.0.1:9001", "node-1", 1))

	assert.Equal(t, 2, c.getQuorum())
}

func TestClusterEnsureCurrentNodeInMembership(t *testing.T) {
	current := newNode("127.0.0.1:9001", "node-1", 1)
	c := &Cluster{
		curNode:    current,
		allNode:    &sync.Map{},
		aliveNodes: &sync.Map{},
	}
	c.allNode.Store("node-0", newNode("127.0.0.1:9000", "node-0", 1))
	c.aliveNodes.Store("node-0", newNode("127.0.0.1:9000", "node-0", 1))
	c.aliveNodes.Store(current.name, current)

	c.ensureCurrentNodeInMembership()

	assert.Equal(t, 2, c.GetAllNodeCount())
	assert.Same(t, current, c.GetNodeByName(current.name))
}

type heartbeatTestStream struct {
	grpc.BidiStreamingClient[proto.HeartbeatRequest, proto.HeartbeatResponse]
	requests  chan *proto.HeartbeatRequest
	responses chan *proto.HeartbeatResponse
}

func (s *heartbeatTestStream) Send(req *proto.HeartbeatRequest) error {
	s.requests <- req
	return nil
}

func (s *heartbeatTestStream) Recv() (*proto.HeartbeatResponse, error) {
	resp, ok := <-s.responses
	if !ok {
		return nil, io.EOF
	}
	return resp, nil
}

type heartbeatTestClient struct {
	proto.ClusterServiceClient
	stream *heartbeatTestStream
}

func (c *heartbeatTestClient) Heartbeat(context.Context, ...grpc.CallOption) (grpc.BidiStreamingClient[proto.HeartbeatRequest, proto.HeartbeatResponse], error) {
	return c.stream, nil
}

func TestHeartbeatSendRequiresRemoteSuccess(t *testing.T) {
	local := newNode("127.0.0.1:9000", "node-b", 1)
	remote := newNode("127.0.0.1:9001", "node-a", 1)
	stream := &heartbeatTestStream{
		requests:  make(chan *proto.HeartbeatRequest, 1),
		responses: make(chan *proto.HeartbeatResponse, 1),
	}
	remote.setGrpcClient(&grpcNodeClient{
		client: &heartbeatTestClient{stream: stream},
	})

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	c := &Cluster{
		config:     &Config{Timeout: time.Second},
		curNode:    local,
		allNode:    &sync.Map{},
		aliveNodes: &sync.Map{},
		ctx:        ctx,
		cancelFunc: cancel,
		logger:     logger.NewLogger(&logger.Config{Level: "FATAL"}),
	}
	c.allNode.Store(remote.name, remote)
	c.aliveNodes.Store(local.name, local)
	c.aliveNodes.Store(remote.name, remote)
	manager := newHeartbeatStreamManager(c)
	c.heartbeatStreams.Store(manager)
	manager.startStream(remote)
	t.Cleanup(manager.stopAll)
	t.Cleanup(func() { close(stream.responses) })

	require.Eventually(t, func() bool {
		entry, ok := manager.streams.Load(remote.name)
		if !ok {
			return false
		}
		streamEntry := entry.(*streamEntry)
		streamEntry.sendLock.Lock()
		defer streamEntry.sendLock.Unlock()
		return streamEntry.stream != nil
	}, time.Second, time.Millisecond)

	go func() {
		req := <-stream.requests
		stream.responses <- &proto.HeartbeatResponse{
			NodeName:       remote.name,
			Term:           req.Term,
			LeaderNodeName: "node-other",
			Success:        false,
		}
	}()

	assert.False(t, manager.sendHeartbeat(remote.name, 1))
}

func TestHeartbeatTimeoutRecyclesStream(t *testing.T) {
	local := newNode("127.0.0.1:9000", "node-b", 1)
	remote := newNode("127.0.0.1:9001", "node-a", 1)
	stream := &heartbeatTestStream{
		requests:  make(chan *proto.HeartbeatRequest, 1),
		responses: make(chan *proto.HeartbeatResponse, 1),
	}
	remote.setGrpcClient(&grpcNodeClient{
		client: &heartbeatTestClient{stream: stream},
	})

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	c := &Cluster{
		config:     &Config{Timeout: 30 * time.Millisecond},
		curNode:    local,
		allNode:    &sync.Map{},
		aliveNodes: &sync.Map{},
		ctx:        ctx,
		cancelFunc: cancel,
		logger:     logger.NewLogger(&logger.Config{Level: "FATAL"}),
	}
	c.allNode.Store(remote.name, remote)
	c.aliveNodes.Store(local.name, local)
	c.aliveNodes.Store(remote.name, remote)
	manager := newHeartbeatStreamManager(c)
	c.heartbeatStreams.Store(manager)
	manager.startStream(remote)
	t.Cleanup(manager.stopAll)
	t.Cleanup(func() { close(stream.responses) })

	require.Eventually(t, func() bool {
		entry, ok := manager.streams.Load(remote.name)
		if !ok {
			return false
		}
		streamEntry := entry.(*streamEntry)
		streamEntry.sendLock.Lock()
		defer streamEntry.sendLock.Unlock()
		return streamEntry.stream != nil
	}, time.Second, time.Millisecond)

	assert.False(t, manager.sendHeartbeat(remote.name, 1))
	_, exists := manager.streams.Load(remote.name)
	assert.False(t, exists, "timed out stream must be recycled before reusing the heartbeat manager")
}

func TestHeartbeatConflictingLeadersConverge(t *testing.T) {
	local := newNode("127.0.0.1:9000", "node-b", 1)
	remote := newNode("127.0.0.1:9001", "node-a", 1)
	stream := &heartbeatTestStream{
		requests:  make(chan *proto.HeartbeatRequest, 1),
		responses: make(chan *proto.HeartbeatResponse, 1),
	}
	remote.setGrpcClient(&grpcNodeClient{
		client: &heartbeatTestClient{stream: stream},
	})

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	c := &Cluster{
		config:     &Config{Timeout: time.Second},
		curNode:    local,
		allNode:    &sync.Map{},
		aliveNodes: &sync.Map{},
		ctx:        ctx,
		cancelFunc: cancel,
		logger:     logger.NewLogger(&logger.Config{Level: "FATAL"}),
	}
	c.allNode.Store(local.name, local)
	c.allNode.Store(remote.name, remote)
	c.aliveNodes.Store(local.name, local)
	c.aliveNodes.Store(remote.name, remote)
	require.True(t, c.signLeader(local, 1))

	manager := newHeartbeatStreamManager(c)
	c.heartbeatStreams.Store(manager)
	manager.startStream(remote)
	t.Cleanup(manager.stopAll)
	t.Cleanup(func() { close(stream.responses) })

	stream.responses <- &proto.HeartbeatResponse{
		NodeName:       remote.name,
		Term:           1,
		LeaderNodeName: remote.name,
		Success:        false,
	}

	require.Eventually(t, func() bool {
		return c.IsFollower() && c.GetLeaderName() == remote.name
	}, time.Second, time.Millisecond)
}
