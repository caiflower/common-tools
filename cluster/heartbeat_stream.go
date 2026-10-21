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
	"io"
	"sync"
	"sync/atomic"
	"time"

	"github.com/caiflower/common-tools/cluster/proto"
	"github.com/caiflower/common-tools/pkg/e"
	"github.com/caiflower/common-tools/pkg/safego"
)

type heartbeatStreamManager struct {
	cluster    *Cluster
	streams    sync.Map
	generation atomic.Int64
}

func newHeartbeatStreamManager(c *Cluster) *heartbeatStreamManager {
	return &heartbeatStreamManager{cluster: c}
}

func (m *heartbeatStreamManager) startStream(node *Node) {
	ctx, cancel := context.WithCancel(m.cluster.ctx)
	gen := m.generation.Add(1)

	entry := &streamEntry{
		node:       node,
		cancel:     cancel,
		generation: gen,
	}
	if _, loaded := m.streams.LoadOrStore(node.name, entry); loaded {
		cancel()
		return
	}

	safego.Go(func() {
		defer e.OnError("heartbeat stream")
		m.runStream(ctx, gen, node)
	})
}

func (m *heartbeatStreamManager) stopStream(nodeName string) {
	if v, ok := m.streams.LoadAndDelete(nodeName); ok {
		entry := v.(*streamEntry)
		entry.cancel()
	}
}

func (m *heartbeatStreamManager) stopAll() {
	m.streams.Range(func(key, value interface{}) bool {
		entry := value.(*streamEntry)
		entry.cancel()
		m.streams.Delete(key)
		return true
	})
}

func (m *heartbeatStreamManager) sendHeartbeat(nodeName string, term int32) bool {
	v, ok := m.streams.Load(nodeName)
	if !ok {
		return false
	}
	entry := v.(*streamEntry)
	entry.requestLock.Lock()
	defer entry.requestLock.Unlock()

	ackCh := make(chan *proto.HeartbeatResponse, 1)
	entry.sendLock.Lock()
	if entry.stream == nil {
		entry.sendLock.Unlock()
		return false
	}

	entry.ackCh = ackCh
	err := entry.stream.Send(&proto.HeartbeatRequest{
		NodeName: m.cluster.GetMyName(),
		Term:     term,
	})
	entry.sendLock.Unlock()
	if err != nil {
		entry.clearAck(ackCh)
		return false
	}

	timeout := m.cluster.config.Timeout / 3
	if timeout <= 0 {
		timeout = time.Second
	}

	select {
	case resp := <-ackCh:
		entry.clearAck(ackCh)
		return resp != nil && resp.Success
	case <-time.After(timeout):
		entry.clearAck(ackCh)
		// A late response on this stream cannot be safely correlated with the next request.
		// Recreate the stream so a stale acknowledgement cannot be attributed to a newer heartbeat.
		m.stopStream(nodeName)
		return false
	case <-m.cluster.ctx.Done():
		entry.clearAck(ackCh)
		return false
	}
}

func (m *heartbeatStreamManager) runStream(ctx context.Context, gen int64, node *Node) {
	c := m.cluster
	backoff := time.Second

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		stream, err := node.heartbeatStream(ctx)
		if err != nil {
			c.logger.Warn("[cluster] heartbeat stream to %s open failed: %v", node.name, err)
			c.markNodeUnavailable(node.name)

			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
			}
			backoff = min(backoff*2, 30*time.Second)
			continue
		}

		backoff = time.Second

		v, ok := m.streams.Load(node.name)
		if !ok {
			return
		}
		entry := v.(*streamEntry)
		if entry.generation != gen {
			return
		}
		entry.sendLock.Lock()
		entry.stream = stream
		entry.sendLock.Unlock()

		for {
			resp, err := stream.Recv()
			if err != nil {
				if err == io.EOF {
					c.logger.Trace("[cluster] heartbeat stream to %s closed by remote", node.name)
				} else {
					c.logger.Warn("[cluster] heartbeat stream to %s recv failed: %v", node.name, err)
				}
				c.markNodeUnavailable(node.name)
				break
			}

			if resp.Success {
				node.updateHeartbeat()
			} else if resp.LeaderNodeName != "" && resp.LeaderNodeName != c.GetLeaderName() {
				node.resetHeartbeatOnLeaderChange()
				previousLeader := c.GetLeaderName()
				if c.adoptLeaderFromHeartbeat(resp) {
					c.logger.Warn("[cluster] heartbeat conflict resolved: local leader %s stepped down for %s (term %d)",
						previousLeader, resp.LeaderNodeName, resp.Term)
				} else {
					c.logger.Debug("[cluster] heartbeat from %s, leader changed: %s -> %s",
						node.name, previousLeader, resp.LeaderNodeName)
				}
			} else {
				node.updateHeartbeatFailed()
			}
			entry.sendLock.Lock()
			if entry.ackCh != nil {
				select {
				case entry.ackCh <- resp:
				default:
				}
			}
			entry.sendLock.Unlock()
		}

		if v, ok := m.streams.Load(node.name); !ok || v.(*streamEntry).generation != gen {
			break
		}
		entry.sendLock.Lock()
		entry.stream = nil
		entry.sendLock.Unlock()

		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		backoff = min(backoff*2, 30*time.Second)
	}
}

func (c *Cluster) adoptLeaderFromHeartbeat(resp *proto.HeartbeatResponse) bool {
	if resp == nil || resp.LeaderNodeName == "" {
		return false
	}

	localLeader := c.GetLeaderName()
	if resp.LeaderNodeName == localLeader {
		return false
	}

	localTerm := c.GetMyTerm()
	remoteTerm := int(resp.Term)
	if remoteTerm < localTerm {
		return false
	}
	// The same term must converge deterministically to avoid two persistent leaders.
	if remoteTerm == localTerm && localLeader != "" && resp.LeaderNodeName > localLeader {
		return false
	}

	var leaderNode *Node
	if v, ok := c.aliveNodes.Load(resp.LeaderNodeName); ok {
		leaderNode = v.(*Node)
	} else if v, ok := c.allNode.Load(resp.LeaderNodeName); ok {
		leaderNode = v.(*Node)
	}
	if leaderNode == nil {
		return false
	}

	return c.signLeader(leaderNode, remoteTerm)
}

type streamEntry struct {
	node        *Node
	stream      proto.ClusterService_HeartbeatClient
	cancel      context.CancelFunc
	generation  int64
	sendLock    sync.Mutex
	requestLock sync.Mutex
	ackCh       chan *proto.HeartbeatResponse
}

func (e *streamEntry) clearAck(ackCh chan *proto.HeartbeatResponse) {
	e.sendLock.Lock()
	defer e.sendLock.Unlock()
	if e.ackCh == ackCh {
		e.ackCh = nil
	}
}
