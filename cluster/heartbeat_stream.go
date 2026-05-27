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
	"time"

	"github.com/caiflower/common-tools/cluster/proto"
	"github.com/caiflower/common-tools/pkg/e"
	"github.com/caiflower/common-tools/pkg/safego"
)

type heartbeatStreamManager struct {
	cluster *Cluster
	streams sync.Map
}

func newHeartbeatStreamManager(c *Cluster) *heartbeatStreamManager {
	return &heartbeatStreamManager{cluster: c}
}

func (m *heartbeatStreamManager) startStream(node *Node) {
	ctx, cancel := context.WithCancel(m.cluster.ctx)

	entry := &streamEntry{
		node:   node,
		cancel: cancel,
	}
	if _, loaded := m.streams.LoadOrStore(node.name, entry); loaded {
		cancel()
		return
	}

	safego.Go(func() {
		defer e.OnError("heartbeat stream")
		m.runStream(ctx, node)
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
	entry.sendLock.Lock()
	defer entry.sendLock.Unlock()

	if entry.stream == nil {
		return false
	}

	err := entry.stream.Send(&proto.HeartbeatRequest{
		NodeName: m.cluster.GetMyName(),
		Term:     term,
	})
	return err == nil
}

func (m *heartbeatStreamManager) runStream(ctx context.Context, node *Node) {
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
		entry.stream = stream

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
				c.logger.Debug("[cluster] heartbeat from %s, leader changed: %s -> %s",
					node.name, c.GetLeaderName(), resp.LeaderNodeName)
			} else {
				node.updateHeartbeatFailed()
			}
		}

		entry.stream = nil

		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		backoff = min(backoff*2, 30*time.Second)
	}
}

type streamEntry struct {
	node     *Node
	stream   proto.ClusterService_HeartbeatClient
	cancel   context.CancelFunc
	sendLock sync.Mutex
}
