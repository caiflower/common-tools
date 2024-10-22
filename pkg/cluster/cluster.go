package cluster

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/caiflower/common-tools/client/nio"
	"github.com/caiflower/common-tools/pkg/env"
	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/caiflower/common-tools/pkg/syncx"
	"github.com/caiflower/common-tools/pkg/tools"
)

type ICluster interface {
	StartUp()                        // 启动
	Close()                          // 关闭
	IsFighting() bool                // 集群是否正在选举
	IsClose() bool                   // 集群是否关闭
	IsReady() bool                   // 集群是否就绪
	IsLeader() bool                  // 当前节点是否是领导人
	IsCandidate() bool               // 当前节点是否是候选人
	IsFollower() bool                // 当前节点是否是群众
	GetLeaderNode() *Node            // 获取当前的主节点
	GetLeaderName() string           // 获取leader名称
	GetMyNode() *Node                // 获取当前本节点
	GetNodeByName(name string) *Node // 根据名称获取节点
	GetMyAddress() string            // 获取当前节点通信地址
	GetMyName() string               // 获得当前本节点名称
	GetMyTerm() int                  // 获取当前本节点任期
	GetAllNodeNames() []string       // 获取所有节点名称
	GetAllNodeCount() int            // 获取所有节点的数量
	GetAliveNodeNames() []string     // 获取所有活着的节点名称
	GetAliveNodeCount() int          // 获取在线节点数量
	GetLostNodeNames() []string      // 获取所有失联节点名称
}

type stat uint8

const (
	_init stat = iota
	fighting
	leader
	candidate
	follower
	closed

	modeCluster = "cluster"
	modeSingle  = "single"
)

type Config struct {
	Mode    string `yaml:"mode"`
	Timeout int    `yaml:"timeout"`
	Nodes   []*struct {
		Name  string
		Ip    string
		Port  int
		Local bool //true表示当前进程与当前node匹配。适用本机测试等情况。线上为了配置文件一致性尽量不要使用。
	}
}

type Cluster struct {
	lock           sync.Locker
	config         *Config
	curNode        *Node // 当前节点
	leaderNode     *Node // 领导节点
	leaderLock     sync.Locker
	lostLeaderTime time.Time      // 没有leader的时间
	allNode        *sync.Map      // 所有的节点
	aliveNodes     *sync.Map      // 所有存活的节点
	term           int            // 当前任期
	sate           stat           // 集群状态
	server         nio.IServer    // 服务端口
	logger         logger.ILog    // 日志框架
	msgChan        chan *Message  // 消息通信chan
	votesMap       map[int]string // 投票map term->nodeName
	votesLock      sync.Locker
}

func NewClusterWithArgs(config *Config, logger logger.ILog) (*Cluster, error) {
	if config.Timeout <= 0 {
		config.Timeout = 10
	}
	if logger == nil {
		return nil, errors.New("logger required")
	}

	switch config.Mode {
	case modeCluster, modeSingle:
	default:
		config.Mode = modeSingle
	}

	cluster := &Cluster{
		config:     config,
		allNode:    &sync.Map{},
		aliveNodes: &sync.Map{},
		term:       0,
		sate:       _init,
		logger:     logger,
		lock:       syncx.NewSpinLock(),
		votesMap:   make(map[int]string),
		votesLock:  syncx.NewSpinLock(),
		leaderLock: syncx.NewSpinLock(),
		msgChan:    make(chan *Message, 100),
	}

	// 初始化节点信息
	cluster.loadNodes()
	cluster.logger.Info("[cluster] loadNodes success, nodes = %+v", cluster.GetAllNodeNames())

	// find curNode
	cluster.findCurNode()

	if cluster.curNode == nil {
		return nil, errors.New("can not find current node")
	} else {
		cluster.logger.Info("[cluster] curNode address: %s", cluster.curNode.address)
	}

	return cluster, nil
}

func NewCluster(config *Config) (*Cluster, error) {
	return NewClusterWithArgs(config, logger.DefaultLogger())
}

func (c *Cluster) StartUp() {
	c.lock.Lock()
	defer c.lock.Unlock()

	// 只允许启动一次
	if c.sate > _init {
		return
	}

	// 开启服务监听端口
	c.listen()

	// 集群建立连接
	c.reconnect()

	// 开始选举
	go c.fighting()

	// 开始心跳
	go c.heartbeat()
}

func (c *Cluster) Close() {

}

func (c *Cluster) IsFighting() bool {
	c.leaderLock.Lock()
	c.leaderLock.Unlock()
	return c.sate == fighting
}

func (c *Cluster) IsClosed() bool {
	c.leaderLock.Lock()
	c.leaderLock.Unlock()
	return c.sate == closed
}

func (c *Cluster) IsReady() bool {
	c.leaderLock.Lock()
	defer c.leaderLock.Unlock()
	return c.sate == leader || (c.sate == follower && c.curNode.heartbeat.Add(time.Duration(c.config.Timeout)*time.Second).After(time.Now()))
}

func (c *Cluster) IsLeader() bool {
	c.leaderLock.Lock()
	defer c.leaderLock.Unlock()
	return c.sate == leader
}

func (c *Cluster) IsCandidate() bool {
	c.leaderLock.Lock()
	c.leaderLock.Unlock()
	return c.sate == candidate
}

func (c *Cluster) IsFollower() bool {
	c.leaderLock.Lock()
	c.leaderLock.Unlock()
	return c.sate == follower
}

func (c *Cluster) GetLeaderNode() *Node {
	return c.leaderNode
}

func (c *Cluster) GetMyNode() *Node {
	return c.curNode
}

func (c *Cluster) GetMyName() string {
	return c.curNode.name
}

func (c *Cluster) GetMyTerm() int {
	return c.term
}

func (c *Cluster) GetLeaderName() string {
	c.leaderLock.Lock()
	defer c.leaderLock.Unlock()
	if c.leaderNode != nil {
		return c.leaderNode.name
	} else {
		return ""
	}
}

func (c *Cluster) GetAllNodeNames() (allNames []string) {
	c.allNode.Range(func(key, value interface{}) bool {
		allNames = append(allNames, key.(string))
		return true
	})
	return
}

func (c *Cluster) GetAllNodeCount() int {
	return len(c.GetAllNodeNames())
}

func (c *Cluster) GetAliveNodeNames() (aliveNames []string) {
	c.aliveNodes.Range(func(key, value interface{}) bool {
		aliveNames = append(aliveNames, key.(string))
		return true
	})
	return
}

func (c *Cluster) GetAliveNodeCount() int {
	return len(c.GetAliveNodeNames())
}

func (c *Cluster) GetLostNodeNames() (lostNames []string) {
	c.allNode.Range(func(key, value interface{}) bool {
		if _, ok := c.aliveNodes.Load(key); !ok {
			lostNames = append(lostNames, key.(string))
		}
		return true
	})
	return
}

func (c *Cluster) IsClose() bool {
	return c.sate == closed
}

func (c *Cluster) GetMyAddress() string {
	return c.curNode.address
}

func (c *Cluster) GetNodeByName(name string) (node *Node) {
	c.allNode.Range(func(key, value interface{}) bool {
		if key.(string) == name {
			node = value.(*Node)
			return false
		}
		return true
	})
	return
}

func (c *Cluster) loadNodes() {
	for _, n := range c.config.Nodes {
		node := newNode(n.Ip+":"+strconv.Itoa(n.Port), n.Name)
		c.allNode.Store(n.Name, node)

		// 如果开启了调试
		if n.Local {
			c.curNode = node
		}
	}
}

func (c *Cluster) findCurNode() {
	if c.curNode == nil { // 说明没有开启调试
		ip := env.GetLocalHostIP()
		c.allNode.Range(func(key, value interface{}) bool {
			node := value.(*Node)
			if strings.Contains(node.address, ip) {
				c.curNode = node
				return false
			}
			return true
		})
	}
}

func (c *Cluster) GetClientHandler(nodeName string) *nio.Handler {
	handler := &nio.Handler{}

	handler.OnSessionConnected = func(session *nio.Session) {
		c.aliveNodes.Store(nodeName, c.GetNodeByName(nodeName))
	}

	handler.OnSessionClosed = func(session *nio.Session) {
		c.aliveNodes.Delete(nodeName)
	}

	handler.OnMessageReceived = func(session *nio.Session, message *nio.Msg) {
		nodeMsg := new(Message)
		if err := message.Unmarshal(nodeMsg); err != nil {
			c.logger.Warn("[cluster] unmarshal msg error: %s", err.Error())
		}
		if nodeMsg.Term < c.GetMyTerm() {
			// 落后的消息直接忽略
			return
		}

		switch message.Flag() {
		case messageAskLeaderRes:
			if nodeMsg.Success {
				c.logger.Debug("[cluster] messageAskLeaderRes nodeName=%s, get leaderNode=%s", nodeMsg.NodeName, nodeMsg.LeaderNodeName)

				if nodeMsg.LeaderNodeName == "" {
					return
				}

				if v, ok := c.aliveNodes.Load(nodeMsg.LeaderNodeName); nodeMsg.LeaderNodeName != "" && ok {
					node := v.(*Node)
					// 标记leader节点
					c.signLeader(node, nodeMsg.Term)
				}
			}
		case messageAskVoteRes:
			c.logger.Debug("[cluster] messageAskVoteRes term=%d nodeName=%s, vote for %s", nodeMsg.Term, nodeMsg.NodeName, nodeMsg.VoteNodeName)

			c.msgChan <- nodeMsg
		case messageBroadcastLeaderRes:
			if nodeMsg.Success {
				c.logger.Debug("[cluster] messageBroadcastLeaderRes nodeName=%s, leaderName=%s", nodeMsg.NodeName, nodeMsg.LeaderNodeName)
			}
			c.msgChan <- nodeMsg
		case messageHeartbeatRes:
			if !nodeMsg.Success {
				c.releaseLeader()
			}
		default:
			c.logger.Warn("[cluster] unknown message type: %s", message.Flag())
		}
	}

	return handler
}

func (c *Cluster) GetServerHandler() *nio.Handler {
	handler := &nio.Handler{}

	handler.OnMessageReceived = func(session *nio.Session, message *nio.Msg) {
		nodeMsg := new(Message)
		if err := message.Unmarshal(nodeMsg); err != nil {
			c.logger.Warn("[cluster] unmarshal msg error: %s", err.Error())
		}

		switch message.Flag() {
		case messageAskLeaderReq:
			// 如果当前集群就绪且当前周期大于等于消息的周期
			msg := new(Message)
			msg.NodeName = c.GetMyName()
			msg.Term = nodeMsg.Term
			msg.Success = false
			if c.IsReady() && nodeMsg.Term <= c.GetMyTerm() {
				msg.Term = c.GetMyTerm()
				msg.LeaderNodeName = c.GetLeaderName()
				msg.Success = true
			}

			if err := session.WriteMsg(nio.NewMsg(messageAskLeaderRes, msg)); err != nil {
				c.logger.Warn("[cluster] write msg error: %s", err.Error())
			}
		case messageAskVoteReq:
			msg := new(Message)
			msg.NodeName = c.GetMyName()
			msg.Term = nodeMsg.Term
			msg.Success = true
			if !c.IsReady() || nodeMsg.Term > c.GetMyTerm() {
				c.releaseLeader()
			}
			msg.VoteNodeName = c.getVoteNodeName(nodeMsg.Term, nodeMsg.NodeName)

			if err := session.WriteMsg(nio.NewMsg(messageAskVoteRes, msg)); err != nil {
				c.logger.Warn("[cluster] write msg error: %s", err.Error())
			}
		case messageBroadcastLeaderReq:
			msg := new(Message)
			msg.NodeName = c.GetMyName()
			msg.Term = nodeMsg.Term
			msg.Success = true
			if c.GetMyTerm() > nodeMsg.Term {
				msg.Term = c.GetMyTerm()
				msg.Success = false
			} else {
				if v, ok := c.aliveNodes.Load(nodeMsg.LeaderNodeName); !ok {
					msg.Success = false
				} else {
					c.curNode.heartbeat = time.Now()
					c.signLeader(v.(*Node), nodeMsg.Term)
					msg.LeaderNodeName = nodeMsg.LeaderNodeName
				}
			}

			if err := session.WriteMsg(nio.NewMsg(messageBroadcastLeaderRes, msg)); err != nil {
				c.logger.Warn("[cluster] write msg error: %s", err.Error())
			}
		case messageHeartbeatReq:
			msg := new(Message)
			msg.NodeName = c.GetMyName()
			msg.Term = nodeMsg.Term
			msg.Success = true
			if c.GetMyTerm() > nodeMsg.Term {
				msg.Term = c.GetMyTerm()
				msg.Success = false
			} else if c.GetLeaderName() == nodeMsg.NodeName {
				c.curNode.heartbeat = time.Now()
			}

			if err := session.WriteMsg(nio.NewMsg(messageHeartbeatRes, msg)); err != nil {
				c.logger.Warn("[cluster] write msg error: %s", err.Error())
			}
		default:
			c.logger.Warn("[cluster] unknown message type: %s", message.Flag())
		}
	}

	return handler
}

func (c *Cluster) needReconnect() (need bool) {
	c.allNode.Range(func(key, value interface{}) bool {
		if _, e := c.aliveNodes.Load(key); !e {
			need = true
			return false
		}
		return true
	})

	return
}

// connect 集群建立连接
func (c *Cluster) reconnect() {
	c.allNode.Range(func(key, value interface{}) bool {
		// 排除自己
		nodeName := key.(string)
		if _, e := c.aliveNodes.Load(nodeName); c.curNode.name != nodeName && !e {
			node := value.(*Node)
			client := nio.NewClient(&nio.Config{
				Addr:    node.address,
				Timeout: c.config.Timeout,
			}, c.GetClientHandler(nodeName))

			if err := client.Connect(); err != nil {
				c.logger.Error("[cluster] connect node %s error: %v", node.address, err)
			}

			node.connection = client
		}
		return true
	})
}

func (c *Cluster) listen() {
	server := nio.NewServer(&nio.Config{
		Addr: c.curNode.address,
	}, c.GetServerHandler())

	if err := server.Open(); err != nil {
		c.logger.Error("[cluster] open server %s error: %v", c.curNode.address, err)
	} else {
		c.server = server
	}
}

func (c *Cluster) fighting() {
	if c.IsFighting() {
		return
	}
	// 保证同时只有一个竞选过程在执行
	c.lock.Lock()
	defer c.lock.Unlock()
	c.sate = fighting

	defer func() {
		if c.leaderNode != nil {
			logger.Info("[cluster] node name: %s term %d fighting finished. leader name: %s", c.curNode.name, c.term, c.leaderNode.name)
		} else {
			c.sate = follower
			// 没有找到主节点，自动开启新一轮竞选
			go c.fighting()
		}
	}()

	// 随机休眠一下，防止所有节点同时开始竞选
	time.Sleep(time.Duration(tools.RandInt(1, 3)) * time.Second)

	// 如果集群已经就绪或者关闭了，那么直接返回即可
	if c.IsReady() || c.IsClose() {
		return
	}

	c.curNode.clean()
	c.releaseLeader()
	sleepTimes := 1

	// 向其他节点查询是否现在已经有主节点了，如果有的话标记主节点。
	for {
		if c.IsReady() || c.IsClose() {
			return
		}

		count := c.GetAliveNodeCount()
		if count > c.GetAllNodeCount()/2 { // 说明该节点可能已经失联
			c.sendMsgWhitTimeout(2*time.Second, messageAskLeaderReq, &Message{NodeName: c.curNode.name, Term: c.term})
			break
		} else {
			if sleepTimes%10 == 0 {
				c.logger.Warn("[cluster] aliveNode len: %d, no enough node to fighting.", count)
			}
		}

		// 重新加载在线节点
		c.reconnect()
		sleepTimes++
		time.Sleep(1 * time.Second)
	}

	// 等待查询结果
	time.Sleep(500 * time.Millisecond)

	// 如果集群已经就绪或者关闭了，那么直接返回即可
	if c.IsReady() || c.IsClose() {
		return
	}

	nextTerm := c.term + 1
	c.leaderLock.Lock()
	c.term = nextTerm
	c.leaderLock.Unlock()
	c.logger.Info("[cluster] node name: %s term: %d begin get votes. ", c.curNode.name, nextTerm)

	// 开始获取选票
	for {
		if c.IsReady() || c.IsClose() {
			return
		}

		c.sate = candidate
		count := c.GetAliveNodeCount()
		if count > c.GetAllNodeCount()/2 { // 说明该节点可能已经失联
			// 先给自己投一票
			if !c.voteNode(nextTerm, c.curNode.name) {
				// 说明已经投票给别人或者自己了，直接进入下一个周期
				return
			}
			// 向其他节点获取投票
			messages := c.sendMsgWhitTimeout(2*time.Second, messageAskVoteReq, &Message{NodeName: c.curNode.name, Term: nextTerm})
			votesCount := 0
			for _, message := range messages {
				if message.VoteNodeName == c.curNode.name {
					votesCount++
				}
			}
			if votesCount+1 > count/2 {
				// 开始广播自己为leader
				messages1 := c.sendMsgWhitTimeout(2*time.Second, messageBroadcastLeaderReq, &Message{NodeName: c.curNode.name, Term: nextTerm, LeaderNodeName: c.curNode.name})
				if len(messages1) != c.GetAliveNodeCount() {
					return
				}
				sign := true
				for _, message := range messages1 {
					if message.Success == false {
						sign = false
					}
				}
				if sign {
					logger.Info("[cluster] sign myself: %s to be leader. ", c.GetMyName())
					c.signLeader(c.curNode, nextTerm)
				}
			}
			break
		} else {
			if sleepTimes%10 == 0 {
				c.logger.Warn("[cluster] aliveNode len: %d, no enough node to fighting.", count)
			}
		}

		// 重新加载在线节点
		c.reconnect()
		sleepTimes++
		time.Sleep(1 * time.Second)
	}
}

func (c *Cluster) heartbeat() {
	for {
		if c.IsLeader() {
			time.Sleep(time.Duration(c.config.Timeout/4) * time.Second)
			c.logger.Debug("[cluster] leader %s send heartbeat.", c.GetMyName())
			// 向所有follower节点发送心跳
			c.sendMsgWhitTimeout(2*time.Second, messageHeartbeatReq, &Message{NodeName: c.curNode.name, Term: c.term})
		} else if c.IsFollower() {
			time.Sleep(time.Duration(c.config.Timeout/3) * time.Second)
			// 如果集群不就绪
			if !c.IsReady() {
				logger.Info("[cluster] follower %s is not ready. go fighting.", c.GetMyName())
				go c.fighting()
			}
		}
	}
}

func (c *Cluster) sendMsgWhitTimeout(timeout time.Duration, flag uint8, msg *Message) []*Message {
	withTimeout, _ := context.WithTimeout(context.Background(), timeout)

	c.aliveNodes.Range(func(key, value interface{}) bool {
		node := value.(*Node)
		go func() {
			if err := node.connection.Write(flag, msg); err != nil {
				c.logger.Error("[cluster] send message to %s error: %v", node.address, err)
			}
		}()
		return true
	})

	// 每次都重新生成，防止上次请求超时消息到这次里面来了
	c.msgChan = make(chan *Message, c.GetAliveNodeCount())
	msgResponseList := make([]*Message, 0)
	for {
		select {
		case <-withTimeout.Done():
			return msgResponseList
		case m := <-c.msgChan:
			msgResponseList = append(msgResponseList, m)
		default:
			if len(msgResponseList) == c.GetAliveNodeCount() {
				return msgResponseList
			}
		}
	}
}

func (c *Cluster) signLeader(node *Node, term int) {
	c.leaderLock.Lock()
	defer c.leaderLock.Unlock()
	if c.GetMyTerm() <= term {
		if node.name == c.curNode.name {
			c.sate = leader
		} else {
			c.sate = follower
		}
		c.lostLeaderTime = time.Time{}
		c.leaderNode = node
		c.term = term
	}
}

func (c *Cluster) releaseLeader() {
	c.leaderLock.Lock()
	defer c.leaderLock.Unlock()
	c.sate = follower
	if c.leaderNode != nil {
		c.lostLeaderTime = time.Now()
	}
	c.leaderNode = nil
}

// getVoteNodeName 根据term获取我投票给的节点名称。每个term只能投给一个node
func (c *Cluster) getVoteNodeName(term int, nodeName string) string {
	c.votesLock.Lock()
	defer c.votesLock.Unlock()

	if v, ok := c.votesMap[term]; !ok {
		c.votesMap[term] = nodeName
		return nodeName
	} else {
		return v
	}
}

func (c *Cluster) voteNode(term int, nodeName string) bool {
	c.votesLock.Lock()
	defer c.votesLock.Unlock()

	if _, ok := c.votesMap[term]; !ok {
		c.votesMap[term] = nodeName
		return true
	} else {
		return false
	}
}