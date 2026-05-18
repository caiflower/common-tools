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
	"fmt"
	"sync"
	"time"

	"github.com/caiflower/common-tools/pkg/nio"
)

type Node struct {
	address           string // 节点通信地址, ip:port
	name              string // 节点名称
	connectLock       sync.RWMutex
	connection        nio.IClient // 连接
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

// 更新心跳失败状态
func (n *Node) updateHeartbeatFailed() {
	n.heartbreakLock.Lock()
	defer n.heartbreakLock.Unlock()

	n.lastHeartbeatOk = false
	n.heartbeatFailures++
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

// 获取节点健康评分 (0-100)
func (n *Node) getHealthScore() int {
	n.heartbreakLock.RLock()
	defer n.heartbreakLock.RUnlock()

	if n.heartbeat.IsZero() {
		return 0
	}

	// 基础分数：基于时间的新鲜度
	//age := time.Since(n.heartbeat).Seconds() - n.heartbeatInterval
	//timeScore := 100 - int(age*10)
	//if timeScore < 0 {
	//	timeScore = 0
	//}
	//
	//if n.connection != nil && n.lastHeartbeatOk {
	//	timeScore += 20
	//}
	//
	//// 失败次数扣分
	//failurePenalty := n.heartbeatFailures * 15
	//timeScore -= failurePenalty
	//
	//if timeScore < 0 {
	//	timeScore = 0
	//}
	//if timeScore > 100 {
	//	timeScore = 100
	//}

	return 100
}

func (n *Node) setConnection(conn nio.IClient) {
	n.connectLock.Lock()
	defer n.connectLock.Unlock()
	n.connection = conn
}

func (n *Node) sendMessage(flag uint8, data interface{}) error {
	n.connectLock.RLock()
	defer n.connectLock.RUnlock()

	if n.connection == nil {
		return fmt.Errorf("connection for cluster node %s is not ready", n.name)
	}

	if err := n.connection.Write(flag, data); err != nil {
		return err
	}
	return nil
}

func (n *Node) close() {
	n.connectLock.Lock()
	defer n.connectLock.Unlock()
	if n.connection != nil {
		n.connection.Close()
		n.connection = nil
	}
}

func (n *Node) isReady(timeout time.Duration) bool {
	n.heartbreakLock.RLock()
	defer n.heartbreakLock.RUnlock()

	return n.heartbeat.Add(timeout).After(time.Now())
}
