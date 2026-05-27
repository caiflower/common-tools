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
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/caiflower/common-tools/cluster/proto"
)

type Node struct {
	address           string // 节点通信地址, ip:port
	name              string // 节点名称
	connectLock       sync.RWMutex
	grpcClient        *grpcNodeClient // gRPC 客户端连接
	heartbreakLock    sync.RWMutex
	heartbeat         time.Time // 主节点发送给自己的心跳时间
	lastHeartbeatOk   bool      // 上次心跳是否成功
	heartbeatFailures int       // 心跳失败次数统计
	heartbeatInterval float64
}

func newNode(address, name string, heartbeatInterval float64) *Node {
	return &Node{
		address:           address,
		name:              name,
		lastHeartbeatOk:   true,
		heartbeatFailures: 0,
		heartbeatInterval: heartbeatInterval,
	}
}

func (n *Node) clean() {
	n.heartbreakLock.Lock()
	defer n.heartbreakLock.Unlock()

	n.heartbeat = time.Time{}
	n.lastHeartbeatOk = false
	n.heartbeatFailures = 0
}

func (n *Node) updateHeartbeat() {
	n.heartbreakLock.Lock()
	defer n.heartbreakLock.Unlock()

	n.heartbeat = time.Now()
	n.lastHeartbeatOk = true
	n.heartbeatFailures = 0
}

func (n *Node) updateHeartbeatFailed() {
	n.heartbreakLock.Lock()
	defer n.heartbreakLock.Unlock()

	n.lastHeartbeatOk = false
	if n.heartbeatFailures < 10 {
		n.heartbeatFailures++
	}
}

func (n *Node) resetHeartbeatOnLeaderChange() {
	n.heartbreakLock.Lock()
	defer n.heartbreakLock.Unlock()

	n.lastHeartbeatOk = true
	n.heartbeatFailures = 0
}

func (n *Node) getHeartbeatFailures() int {
	n.heartbreakLock.RLock()
	defer n.heartbreakLock.RUnlock()

	return n.heartbeatFailures
}

func (n *Node) isHeartbeatZero() bool {
	n.heartbreakLock.RLock()
	defer n.heartbreakLock.RUnlock()

	return n.heartbeat.IsZero()
}

func (n *Node) getHealthScore() int {
	n.heartbreakLock.RLock()
	defer n.heartbreakLock.RUnlock()

	if n.heartbeat.IsZero() {
		return 0
	}

	if n.heartbeatFailures > 3 {
		return max(0, 100-n.heartbeatFailures*20)
	}

	if !n.lastHeartbeatOk {
		return 50
	}

	return 100
}

func (n *Node) setGrpcClient(client *grpcNodeClient) {
	n.connectLock.Lock()
	defer n.connectLock.Unlock()
	n.grpcClient = client
}

func (n *Node) getGrpcClient() *grpcNodeClient {
	n.connectLock.RLock()
	defer n.connectLock.RUnlock()
	return n.grpcClient
}

func (n *Node) askLeader(ctx context.Context, req *proto.AskLeaderRequest) (*proto.AskLeaderResponse, error) {
	client := n.getGrpcClient()
	if client == nil {
		return nil, fmt.Errorf("connection for cluster node %s is not ready", n.name)
	}
	return client.AskLeader(ctx, req)
}

func (n *Node) askVote(ctx context.Context, req *proto.AskVoteRequest) (*proto.AskVoteResponse, error) {
	client := n.getGrpcClient()
	if client == nil {
		return nil, fmt.Errorf("connection for cluster node %s is not ready", n.name)
	}
	return client.AskVote(ctx, req)
}

func (n *Node) broadcastLeader(ctx context.Context, req *proto.BroadcastLeaderRequest) (*proto.BroadcastLeaderResponse, error) {
	client := n.getGrpcClient()
	if client == nil {
		return nil, fmt.Errorf("connection for cluster node %s is not ready", n.name)
	}
	return client.BroadcastLeader(ctx, req)
}

func (n *Node) remoteCall(ctx context.Context, req *proto.RemoteCallRequest) (*proto.RemoteCallResponse, error) {
	client := n.getGrpcClient()
	if client == nil {
		return nil, fmt.Errorf("connection for cluster node %s is not ready", n.name)
	}
	return client.RemoteCall(ctx, req)
}

func (n *Node) heartbeatStream(ctx context.Context) (proto.ClusterService_HeartbeatClient, error) {
	client := n.getGrpcClient()
	if client == nil {
		return nil, fmt.Errorf("connection for cluster node %s is not ready", n.name)
	}
	return client.Heartbeat(ctx)
}

func (n *Node) close() {
	n.connectLock.Lock()
	defer n.connectLock.Unlock()
	if n.grpcClient != nil {
		_ = n.grpcClient.Close()
		n.grpcClient = nil
	}
}

func (n *Node) isReady(timeout time.Duration) bool {
	n.heartbreakLock.RLock()
	defer n.heartbreakLock.RUnlock()

	return n.heartbeat.Add(timeout).After(time.Now())
}
