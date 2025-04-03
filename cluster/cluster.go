package cluster

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/caiflower/common-tools/global"
	"github.com/caiflower/common-tools/global/env"
	"github.com/caiflower/common-tools/pkg/cache"
	golocalv1 "github.com/caiflower/common-tools/pkg/golocal/v1"
	"github.com/caiflower/common-tools/pkg/nio"

	"github.com/caiflower/common-tools/pkg/e"
	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/caiflower/common-tools/pkg/syncx"
	"github.com/caiflower/common-tools/pkg/tools"
)

type ICluster interface {
	StartUp()                                                                     // 启动
	Close()                                                                       // 关闭
	IsFighting() bool                                                             // 集群是否正在选举
	IsClose() bool                                                                // 集群是否关闭
	IsReady() bool                                                                // 集群是否就绪
	IsLeader() bool                                                               // 当前节点是否是领导人
	IsCandidate() bool                                                            // 当前节点是否是候选人
	IsFollower() bool                                                             // 当前节点是否是群众
	GetLeaderNode() *Node                                                         // 获取当前的主节点
	GetLeaderName() string                                                        // 获取leader名称
	GetMyNode() *Node                                                             // 获取当前本节点
	GetNodeByName(name string) *Node                                              // 根据名称获取节点
	GetMyAddress() string                                                         // 获取当前节点通信地址
	GetMyName() string                                                            // 获得当前本节点名称
	GetMyTerm() int                                                               // 获取当前本节点任期
	GetAllNodeNames() []string                                                    // 获取所有节点名称
	GetAllNodeCount() int                                                         // 获取所有节点的数量
	GetAliveNodeNames() []string                                                  // 获取所有活着的节点名称
	GetAliveNodeCount() int                                                       // 获取在线节点数量
	GetLostNodeNames() []string                                                   // 获取所有失联节点名称
	AddJobTracker(v JobTracker)                                                   // add scheduler
	RemoveJobTracker(v JobTracker)                                                // remove scheduler
	RegisterFunc(funcName string, fn func(data interface{}) (interface{}, error)) // registerFunc
	CallFunc(fc *FuncSpec) (interface{}, error)                                   // callFunc
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
	Mode    string `yaml:"mode" default:"cluster"`
	Timeout int    `yaml:"timeout" default:"10"`
	Enable  string `yaml:"enable" default:"false"`
	Nodes   []*struct {
		Name  string
		Ip    string
		Port  int
		Local bool //true表示当前进程与当前node匹配。适用本机测试等情况。线上为了配置文件一致性尽量不要使用。
	}
}

type Cluster struct {
	lock           sync.Locker                                            // 启动关闭锁
	fightingLock   sync.Locker                                            // 竞争锁
	config         *Config                                                // 配置文件
	curNode        *Node                                                  // 当前节点
	leaderNode     *Node                                                  // 领导节点
	leaderName     string                                                 // 领导节点名称
	leaderLock     sync.Locker                                            // leader锁
	lostLeaderTime time.Time                                              // 没有leader的时间
	allNode        *sync.Map                                              // 所有的节点
	aliveNodes     *sync.Map                                              // 所有存活的节点
	term           int                                                    // 当前任期
	sate           stat                                                   // 集群状态
	server         nio.IServer                                            // 服务端口
	logger         logger.ILog                                            // 日志框架
	msgChan        chan *Message                                          // 消息通信chan
	votesMap       map[int]string                                         // 投票map term->nodeName
	votesLock      sync.Locker                                            // 投票锁
	localFuncs     map[string]func(data interface{}) (interface{}, error) // 本地函数
	ctx            context.Context
	cancelFunc     context.CancelFunc
	events         chan *event
	jobTrackers    *sync.Map
}

func NewClusterWithArgs(config Config, logger logger.ILog) (*Cluster, error) {
	tools.DoTagFunc(&config, nil, []func(reflect.StructField, reflect.Value, interface{}) error{tools.SetDefaultValueIfNil})

	if config.Timeout <= 0 {
		config.Timeout = 10
	}
	if config.Enable == "" {
		config.Enable = "True"
	}
	if logger == nil {
		return nil, errors.New("logger required")
	}

	switch config.Mode {
	case modeCluster, modeSingle:
	default:
		config.Mode = modeCluster
	}

	cluster := &Cluster{
		config:       &config,
		allNode:      &sync.Map{},
		aliveNodes:   &sync.Map{},
		term:         0,
		sate:         _init,
		logger:       logger,
		lock:         syncx.NewSpinLock(),
		fightingLock: syncx.NewSpinLock(),
		votesMap:     make(map[int]string),
		votesLock:    syncx.NewSpinLock(),
		leaderLock:   syncx.NewSpinLock(),
		events:       make(chan *event, 20),
		jobTrackers:  &sync.Map{},
		localFuncs:   make(map[string]func(data interface{}) (interface{}, error)),
	}

	// 初始化节点信息
	cluster.loadNodes()
	enable, _ := strconv.ParseBool(config.Enable)
	if enable {
		cluster.logger.Info("[cluster] loadNodes success, nodes = %+v", cluster.GetAllNodeNames())
	}

	// find curNode
	cluster.findCurNode()
	cluster.aliveNodes.Store(cluster.GetMyName(), cluster.GetMyNode())

	if cluster.curNode == nil {
		return nil, errors.New("can not find current node")
	} else {
		if enable {
			cluster.logger.Info("[cluster] curNode address: %s", cluster.curNode.address)
		}
	}

	return cluster, nil
}

func NewCluster(config Config) (*Cluster, error) {
	return NewClusterWithArgs(config, logger.DefaultLogger())
}

func (c *Cluster) StartUp() {
	c.lock.Lock()
	defer c.lock.Unlock()

	// 只允许启动一次
	enable, _ := strconv.ParseBool(c.config.Enable)
	if !enable || (c.sate > _init && c.sate != closed) {
		return
	}
	c.term = 0
	c.sate = _init

	ctx, cancelFunc := context.WithCancel(context.Background())
	c.cancelFunc = cancelFunc
	c.ctx = ctx

	if c.config.Mode != modeSingle {
		// 开启服务监听端口
		c.listen()
		// 集群建立连接
		c.reconnect()
	}

	// 开始选举
	go c.fighting()

	if c.config.Mode != modeSingle {
		// 开始心跳
		go c.heartbeat()
	}

	// 开始消费事件
	go c.consumeEvent()

	c.logger.Info("[cluster] startup success. ")

	go c.createEvent(eventNameStartUp, "")

	global.DefaultResourceManger.Add(c)
}

func (c *Cluster) Close() {
	if c.IsClosed() {
		return
	}

	c.lock.Lock()
	defer c.lock.Unlock()

	if c.IsClosed() {
		return
	}

	if c.cancelFunc != nil {
		c.cancelFunc()
	}
	c.curNode.clean()
	c.releaseLeader()

	c.aliveNodes.Range(func(key, value interface{}) bool {
		node := value.(*Node)
		if node.connection != nil {
			go node.connection.Close()
		}
		return true
	})

	if c.server != nil {
		go c.server.Close()
	}

	go c.createEvent(eventNameClose, "")
	c.sate = closed
	c.logger.Info("[cluster] close success. ")
}

func (c *Cluster) IsFighting() bool {
	return c.sate == fighting
}

func (c *Cluster) IsClosed() bool {
	return c.sate == closed
}

func (c *Cluster) IsReady() bool {
	return c.sate == leader || (c.sate == follower && c.curNode.heartbeat.Add(time.Duration(c.config.Timeout)*time.Second).After(time.Now()))
}

func (c *Cluster) IsLeader() bool {
	return c.sate == leader
}

func (c *Cluster) IsCandidate() bool {
	return c.sate == candidate
}

func (c *Cluster) IsFollower() bool {
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
	return c.leaderName
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

func (c *Cluster) AddJobTracker(v JobTracker) {
	if v == nil {
		return
	}
	c.jobTrackers.Store(v.Name(), v)
}

func (c *Cluster) RemoveJobTracker(v JobTracker) {
	if v == nil {
		return
	}
	c.jobTrackers.Delete(v.Name())
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
	if c.config.Mode == modeSingle {
		if c.curNode == nil {
			c.curNode = newNode("127.0.0.1:10000", "single")
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

// getClientHandler client contact
func (c *Cluster) getClientHandler(nodeName string) *nio.Handler {
	handler := &nio.Handler{}

	handler.OnSessionConnected = func(session *nio.Session) {
		c.aliveNodes.Store(nodeName, c.GetNodeByName(nodeName))
	}

	handler.OnSessionClosed = func(session *nio.Session) {
		c.aliveNodes.Delete(nodeName)
	}

	handler.OnMessageReceived = func(session *nio.Session, message *nio.Msg) {
		// 其他协议，不受任期影响
		if message.Flag() == messageRemoteCallRes {
			rcMsg := new(remoteCallMessage)
			err := message.Unmarshal(rcMsg)
			if err != nil {
				c.logger.Warn("[cluster] unmarshal remoteCallMessage error: %s", err.Error())
			}

			c.logger.Debug("[cluster] remote [%s] func return", session.GetRemoteAddr())
			if f, ok := cache.LocalCache.Get(remoteCall + rcMsg.UUID); ok && f != nil {
				f.(*FuncSpec).setResult(rcMsg.Result, rcMsg.Err)
				// remote return, then delete cache
				cache.LocalCache.Delete(remoteCall + rcMsg.UUID)
			}
			return
		}

		// 集群基本协议
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

				c.msgChan <- nodeMsg
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
			c.msgChan <- nodeMsg
		default:
			c.logger.Warn("[cluster] unknown message type: %s", message.Flag())
		}
	}

	return handler
}

// getServerHandler server contact
func (c *Cluster) getServerHandler() *nio.Handler {
	handler := &nio.Handler{}

	handler.OnMessageReceived = func(session *nio.Session, message *nio.Msg) {
		// 其他协议，不受任期影响
		if message.Flag() == messageRemoteCallReq {
			rcMsg := new(remoteCallMessage)
			err := message.Unmarshal(rcMsg)
			if err != nil {
				c.logger.Warn("[cluster] unmarshal remoteCallMessage error: %s", err.Error())
			}
			f := &FuncSpec{
				traceId:  rcMsg.TraceID,
				uuid:     rcMsg.UUID,
				funcName: rcMsg.FuncName,
				param:    rcMsg.Param,
				sync:     rcMsg.Sync,
			}
			msg := new(remoteCallMessage)
			msg.TraceID = f.traceId
			msg.UUID = f.uuid
			msg.FuncName = f.funcName
			msg.Param = f.param
			msg.Sync = f.sync

			c.logger.Debug("[cluster] '%s' exec remote func", c.GetMyName())
			c.callLocalFunc(f)
			msg.Result = f.result
			msg.Err = f.err

			if err = session.WriteMsg(nio.NewMsg(messageRemoteCallRes, msg)); err != nil {
				c.logger.Error("[cluster] [remote call] %s failed. %s Cause of %s.", f.uuid, f.funcName, err)
			}
			return
		}

		// 集群基本协议
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
			if _, ok := c.aliveNodes.Load(nodeMsg.NodeName); !ok {
				// 重新建立集群连接
				go c.reconnect()
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
				if !c.curNode.heartbeat.IsZero() {
					c.curNode.heartbeat = time.Now()
				} else {
					// 说明follower已经开始竞选了
					msg.Success = false
				}
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
		if _, ex := c.aliveNodes.Load(key); !ex {
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
		if _, ex := c.aliveNodes.Load(nodeName); c.curNode.name != nodeName && !ex {
			node := value.(*Node)
			client := nio.NewClient(&nio.Config{
				Addr:    node.address,
				Timeout: 1,
			}, c.getClientHandler(nodeName))

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
	}, c.getServerHandler())

	if err := server.Open(); err != nil {
		c.logger.Error("[cluster] open server %s error: %v", c.curNode.address, err)
	} else {
		c.server = server
	}
}

func (c *Cluster) fighting() {
	defer func() {
		e.OnError("fighting")
	}()

	if c.config.Mode == modeSingle {
		c.signLeader(c.curNode, 0)
		return
	}

	if c.IsFighting() || c.IsReady() || c.IsClose() {
		return
	}

	// 保证同时只有一个竞选过程在执行
	c.fightingLock.Lock()
	defer c.fightingLock.Unlock()

	// 如果集群已经就绪或者关闭了，那么直接返回即可
	if c.IsReady() || c.IsClose() {
		return
	}
	c.sate = fighting

	defer func() {
		if c.leaderNode != nil {
			c.logger.Info("[cluster] node name: %s term %d fighting finished. leader name: %s", c.curNode.name, c.term, c.leaderName)
		} else {
			if !c.IsClose() {
				c.sate = follower
			}
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
		if count > c.GetAllNodeCount()/2 { // 如果否，说明该节点可能已经失联
			messages := c.sendMsgWhitTimeout(2*time.Second, messageAskLeaderReq, &Message{NodeName: c.curNode.name, Term: c.term})
			if len(messages)+1 < count/2 {
				// 再问一遍
				continue
			}
			leaderNode := ""
			term := 0
			ansCount := 0
			for _, message := range messages {
				if message.Success {
					if leaderNode == "" {
						leaderNode = message.LeaderNodeName
						term = message.Term
						ansCount++
					} else {
						if message.LeaderNodeName != leaderNode || message.Term != term {
							leaderNode = ""
							break
						} else {
							ansCount++
						}
					}
				}
			}
			if leaderNode != "" && ansCount+1 > count/2 {
				if v, ok := c.aliveNodes.Load(leaderNode); ok {
					node := v.(*Node)
					c.curNode.heartbeat = time.Now()
					// 标记leader节点
					c.signLeader(node, term)
				} else if leaderNode == c.curNode.name {
					c.signLeader(c.curNode, term)
				}
			}
			if term > c.GetMyTerm() {
				// 加速周期
				c.term = term
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
		time.Sleep(2 * time.Second)
	}

	// 等待查询结果
	time.Sleep(500 * time.Millisecond)

	// 如果集群已经就绪或者关闭了，那么直接返回即可
	if c.IsReady() || c.IsClose() {
		return
	}

	nextTerm := c.term + 1
	c.term = nextTerm
	c.logger.Info("[cluster] node name: %s term: %d begin get votes. ", c.curNode.name, nextTerm)
	go c.createEvent(eventNameElectionStart, "")
	defer func() {
		go c.createEvent(eventNameElectionFinish, c.GetLeaderName())
	}()

	// 开始获取选票
	for {
		if c.IsReady() || c.IsClose() {
			return
		}

		count := c.GetAliveNodeCount()
		if count > c.GetAllNodeCount()/2 { // 如果否，说明该节点可能已经失联
			c.sate = candidate
			// 先给自己投一票
			if !c.voteNode(nextTerm, c.curNode.name) {
				// 说明已经投票给别人或者自己了，直接进入下一个周期
				return
			}
			// 向其他节点获取投票
			messages := c.sendMsgWhitTimeout(2*time.Second, messageAskVoteReq, &Message{NodeName: c.curNode.name, Term: nextTerm})
			if len(messages)+1 < count/2 {
				// 说明没有同步成功，进入下一个周期
				return
			}
			votesCount := 0
			for _, message := range messages {
				if message.VoteNodeName == c.curNode.name {
					votesCount++
				}
			}
			if votesCount+1 > count/2 {
				// 开始广播自己为leader
				messages1 := c.sendMsgWhitTimeout(2*time.Second, messageBroadcastLeaderReq, &Message{NodeName: c.curNode.name, Term: nextTerm, LeaderNodeName: c.curNode.name})
				if len(messages1) < count/2 {
					// 说明没有同步成功，进入下一个周期
					return
				}
				sign := true
				for _, message := range messages1 {
					if message.Success == false {
						sign = false
					}
				}
				if sign {
					c.logger.Info("[cluster] sign myself: %s to be leader. ", c.GetMyName())
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
		time.Sleep(2 * time.Second)
	}
}

func (c *Cluster) heartbeat() {
	defer func() {
		e.OnError("heartbeat")
	}()

	ticker := time.NewTicker(time.Second * time.Duration(c.config.Timeout/4))
	defer ticker.Stop()
	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			if c.IsLeader() {
				c.logger.Debug("[cluster] leader %s send heartbeat.", c.GetMyName())
				// 向所有follower节点发送心跳
				messages := c.sendMsgWhitTimeout(2*time.Second, messageHeartbeatReq, &Message{NodeName: c.curNode.name, Term: c.term})
				count := c.GetAllNodeCount()
				leastCnt := (count - 1) / 2 // 去掉自己
				if len(messages) < leastCnt {
					c.logger.Info("[cluster] heartbeat len: %d, but least need: %d, releaseLeader %s", len(messages), leastCnt, c.GetMyName())
					c.releaseLeader()
				} else {
					success := 0
					for _, message := range messages {
						if message.Success {
							success++
						}
					}
					if success < leastCnt {
						c.logger.Info("[cluster] heartbeat len: %d, but least need: %d, releaseLeader %s", success, leastCnt, c.GetMyName())
						c.releaseLeader()
					}
				}
			} else if c.IsFollower() {
				// 如果集群不就绪
				if !c.IsReady() || c.leaderNode == nil {
					c.logger.Info("[cluster] follower %s is not ready. go fighting.", c.GetMyName())
					go c.fighting()
				}
			}
		default:
		}
	}
}

func (c *Cluster) sendMsgWhitTimeout(timeout time.Duration, flag uint8, msg *Message) []*Message {
	withTimeout, _ := context.WithTimeout(context.Background(), timeout)

	c.aliveNodes.Range(func(key, value interface{}) bool {
		node := value.(*Node)
		if node.name == c.GetMyName() {
			return true
		}
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
			go c.createEvent(eventNameSignMaster, node.name)
			// 单集群模式自己就是Follower
			if c.config.Mode == modeSingle {
				go c.createEvent(eventNameSignFollower, node.name)
			}
		} else {
			c.sate = follower
			go c.createEvent(eventNameSignFollower, node.name)
		}
		c.lostLeaderTime = time.Time{}
		c.leaderNode = node
		c.leaderName = node.name
		c.term = term
	}
}

func (c *Cluster) releaseLeader() {
	c.leaderLock.Lock()
	defer c.leaderLock.Unlock()
	if c.sate != closed {
		c.sate = follower
	}
	if c.leaderNode != nil {
		c.lostLeaderTime = time.Now()
		if c.leaderName == c.GetMyName() {
			go c.createEvent(eventNameStopMaster, "")
		} else {
			go c.createEvent(eventNameReleaseMaster, "")
		}
	}
	c.leaderNode = nil
	c.leaderName = ""
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

func (c *Cluster) createEvent(name, leaderName string) {
	c.events <- &event{name, c.sate, c.GetMyName(), leaderName}
}

func (c *Cluster) consumeEvent() {
	defer func() {
		e.OnError("consumeEvent")
	}()

	for {
		select {
		case ev := <-c.events:
			switch ev.name {
			case eventNameStartUp:
				c.logger.Debug("[cluster] %s start up event, cluster status: %s", ev.nodeName, getStatusName(ev.clusterStat))
			case eventNameSignFollower:
				c.logger.Debug("[cluster] %s sign follower event, cluster status: %s", ev.nodeName, getStatusName(ev.clusterStat))
				c.jobTrackers.Range(func(key, value interface{}) bool {
					jobTracker := value.(JobTracker)
					jobTracker.OnNewLeader(ev.leaderName)
					return true
				})
			case eventNameSignMaster:
				c.logger.Debug("[cluster] %s sign master event, cluster status: %s", ev.nodeName, getStatusName(ev.clusterStat))
				c.jobTrackers.Range(func(key, value interface{}) bool {
					jobTracker := value.(JobTracker)
					jobTracker.OnStartedLeading()
					return true
				})
			case eventNameStopMaster:
				c.logger.Debug("[cluster] %s stop master event, cluster status: %s", ev.nodeName, getStatusName(ev.clusterStat))
				c.jobTrackers.Range(func(key, value interface{}) bool {
					jobTracker := value.(JobTracker)
					jobTracker.OnStoppedLeading()
					return true
				})
			case eventNameElectionStart:
				c.logger.Debug("[cluster] %s election start event, cluster status: %s", ev.nodeName, getStatusName(ev.clusterStat))
			case eventNameElectionFinish:
				c.logger.Debug("[cluster] %s election finish event, cluster status: %s", ev.nodeName, getStatusName(ev.clusterStat))
			case eventNameReleaseMaster:
				c.logger.Debug("[cluster] %s release master event, cluster status: %s", ev.nodeName, getStatusName(ev.clusterStat))
				c.jobTrackers.Range(func(key, value interface{}) bool {
					tracker := value.(JobTracker)
					tracker.OnReleaseMaster()
					return true
				})
			case eventNameClose:
				c.logger.Debug("[cluster] %s close event, cluster status: %s", ev.nodeName, getStatusName(ev.clusterStat))
			default:
				c.logger.Warn("[cluster] unknown type %s event", ev.name)
			}
		default:

		}
	}
}

func getStatusName(s stat) string {
	switch s {
	case closed:
		return "closed"
	case _init:
		return "init"
	case fighting:
		return "fighting"
	case candidate:
		return "candidate"
	case follower:
		return "follower"
	case leader:
		return "leader"
	default:
		return ""
	}
}

func (c *Cluster) RegisterFunc(funcName string, fn func(data interface{}) (interface{}, error)) {
	c.localFuncs[funcName] = fn
}

func (c *Cluster) CallFunc(f *FuncSpec) (interface{}, error) {

	// 本地调用
	if c.GetMyNode().name == f.nodeName {
		c.logger.Debug("[%s] call local func '%s'", f.uuid, f.funcName)
		go c.callLocalFunc(f)

	} else { // 远程调用
		c.logger.Debug("[%s] call remote func '%s - %s'", f.uuid, f.nodeName, f.funcName)
		cache.LocalCache.Set(remoteCall+f.uuid, f, f.timeout+(5*time.Second)) //写缓存，TTL时间比超时时间富余一些。
		c.callRemoteFunc(f)
	}
	f.wait()
	return f.result, f.err
}

func (c *Cluster) callLocalFunc(f *FuncSpec) {
	if golocalv1.GetTraceID() == "" {
		defer golocalv1.Clean()
		golocalv1.PutTraceID(f.traceId)
	}
	fc := c.localFuncs[f.funcName]
	if fc == nil {
		err := fmt.Errorf("not such function '%s' in the cluster", f.funcName)
		c.logger.Error("[cluster] [remote call] failed. %s Cause of %s", f.uuid, err)
		f.setResult(nil, err)
		return
	}
	defer func() {
		if r := recover(); r != nil {
			c.logger.Error("Got a runtime error %s. [remote call]\n%s", r, string(debug.Stack()))
			f.setResult(nil, fmt.Errorf("%s", r))
		}
	}()
	f.setResult(fc(f.param))
}

func (c *Cluster) callRemoteFunc(f *FuncSpec) {
	var send bool
	msg := &remoteCallMessage{
		TraceID: f.traceId, UUID: f.uuid, FuncName: f.funcName, Param: f.param, Sync: f.sync,
	}
	c.aliveNodes.Range(func(key, val interface{}) bool {
		if _node, ok := val.(*Node); ok && _node.name == f.nodeName {
			send = true
			if err := _node.SendMessage(messageRemoteCallReq, msg); err != nil {
				f.setResult(nil, fmt.Errorf("remote call failed. %w", err))
				c.logger.Error("[cluster] [remote call] %s failed. %s Cause of %s.", f.uuid, f.funcName, err)
			}
			return false
		}
		return true
	})
	if !send {
		f.setResult(nil, fmt.Errorf("the node %s does not exist or is dead", f.nodeName))
	}
}
