package cluster

import (
	"time"

	"github.com/caiflower/common-tools/client/nio"
)

type Node struct {
	address    string      // 节点通信地址, ip:port
	name       string      // 节点名称
	connection nio.IClient // 连接
	heartbeat  time.Time   // 主节点发送给自己的心跳时间
}

func newNode(address, name string) *Node {
	return &Node{
		address: address,
		name:    name,
	}
}

func (n *Node) clean() {
	n.heartbeat = time.Time{}
}

func (n *Node) updateHeartbeat() {
	n.heartbeat = time.Now()
}
