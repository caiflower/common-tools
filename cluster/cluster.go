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
	"errors"
	"fmt"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/caiflower/common-tools/global/env"
	"github.com/caiflower/common-tools/pkg/bean"
	redisv1 "github.com/caiflower/common-tools/redis/v1"

	"github.com/caiflower/common-tools/pkg/cache"
	golocalv1 "github.com/caiflower/common-tools/pkg/golocal/v1"
	"github.com/caiflower/common-tools/pkg/nio"

	"github.com/caiflower/common-tools/pkg/e"
	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/caiflower/common-tools/pkg/syncx"
	"github.com/caiflower/common-tools/pkg/tools"
)

type ICluster interface {
	Name() string                                                                 // 名称
	Start() error                                                                 // 启动
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
	AddJobTracker(v JobTracker) error                                             // add scheduler
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
	modeRedis   = "redis"

	remoteFuncNameOfReloadAllNodes = "cluster.ReloadAllNodes"
)

type Config struct {
	Mode    string `yaml:"mode" default:"cluster" json:"mode"`
	Timeout int    `yaml:"timeout" default:"10" json:"timeout"`
	Enable  string `yaml:"enable" default:"false" json:"enable"`
	Nodes   []*struct {
		Name  string
		Ip    string
		Port  int
		Local bool //true表示当前进程与当前node匹配。适用本机测试等情况。线上为了配置文件一致性尽量不要使用。
	} `yaml:"nodes" json:"nodes"`
	RedisDiscovery    RedisDiscovery    `yaml:"redisDiscovery" json:"redisDiscovery"`
	ReplicasDiscovery ReplicasDiscovery `yaml:"replicasDiscovery" json:"replicasDiscovery"`
}

type RedisDiscovery struct {
	BeanName           string        `yaml:"beanName"`                         // 如果为空，则Ioc分配
	DataPath           string        `yaml:"dataPath"`                         // redis key
	ElectionInterval   time.Duration `yaml:"electionInterval" default:"15s"`   //多久进行一次选主/续约
	ElectionPeriod     time.Duration `yaml:"electionPeriod" default:"30s"`     //选主/续约后有效租期时间
	SyncLeaderInterval time.Duration `yaml:"syncLeaderInterval" default:"10s"` //多久同步一次leader
}

type ReplicasDiscovery struct {
	DomainPatten string `yaml:"domainPatten"`
	Port         int    `yaml:"port" default:"8081"`
	Replicas     int    `yaml:"replicas"`
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
	Redis          redisv1.RedisClient                                    `autowired:"" conditional_on_property:"default.cluster.mode=redis"` // redis
	ctx            context.Context
	cancelFunc     context.CancelFunc
	events         chan *event
	jobTrackers    *sync.Map
}

func NewClusterWithArgs(config Config, logger logger.ILog) (*Cluster, error) {
	_ = tools.DoTagFunc(&config, []tools.FnObj{{Fn: tools.SetDefaultValueIfNil}})

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
	case modeCluster, modeSingle, modeRedis:
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
	// find curNode
	cluster.findCurNode()
	cluster.aliveNodes.Store(cluster.GetMyName(), cluster.GetMyNode())

	if cluster.curNode == nil {
		return nil, errors.New("can not find current node")
	}

	cluster.logger.Info("[cluster] curNode address: %s", cluster.curNode.address)

	// redis beanName
	if config.Mode == modeRedis && config.RedisDiscovery.BeanName != "" {
		if b := bean.GetBean(config.RedisDiscovery.BeanName); b == nil {
			panic(fmt.Sprintf("[cluster] redis mode, can not find redis bean %s." + config.RedisDiscovery.BeanName))
		} else {
			cluster.Redis = b.(redisv1.RedisClient)
		}
	}

	// register remote func
	cluster.RegisterFunc(remoteFuncNameOfReloadAllNodes, cluster.reloadAllNodes)

	return cluster, nil
}

func NewCluster(config Config) (*Cluster, error) {
	return NewClusterWithArgs(config, logger.DefaultLogger())
}

func (c *Cluster) Name() string {
	return fmt.Sprintf("Cluster:%s", c.config.Mode)
}

func (c *Cluster) Start() error {
	c.lock.Lock()
	defer c.lock.Unlock()

	// 只允许启动一次
	enable, _ := strconv.ParseBool(c.config.Enable)
	if !enable || (c.sate > _init && c.sate != closed) {
		return nil
	}
	if c.GetMyNode() == nil {
		return errors.New("start cluster failed. can not find current node")
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

	// 扩容情况
	if c.enableReplicasDiscovery() && env.Kubernetes {
		for _, v := range c.GetAliveNodeNames() {
			if v != c.GetMyName() {
				if _, err := c.CallFunc(NewFuncSpec(v, remoteFuncNameOfReloadAllNodes, c.config.ReplicasDiscovery.Replicas, 2*time.Second).IgnoreNotReady()); err != nil {
					logger.Error("[cluster] call %s reload nodes failed. %s", v, err.Error())
				}
			}
		}
	}

	switch c.config.Mode {
	case modeCluster:
		// 开始选举
		go c.fighting()
		// 开始心跳
		go c.heartbeat()
	case modeSingle:
		c.signLeader(c.GetMyNode(), 0)
	case modeRedis:
		c.redisClusterStartUp()
	default:
		go c.fighting()
		go c.heartbeat()
	}

	// 开始消费事件
	go c.consumeEvent()

	c.logger.Info("[cluster] startup success. ")

	c.createEvent(eventNameStartUp, "")

	return nil
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

	// 缩容或者重启
	if c.enableReplicasDiscovery() && env.Kubernetes {
		for _, v := range c.GetAliveNodeNames() {
			if v != c.GetMyName() {
				if _, err := c.CallFunc(NewFuncSpec(v, remoteFuncNameOfReloadAllNodes, c.config.ReplicasDiscovery.Replicas, 2*time.Second)); err != nil {
					logger.Error("[cluster] call %s reload nodes failed. %s", v, err.Error())
				}
			}
		}
	}

	if c.cancelFunc != nil {
		c.cancelFunc()
	}
	c.curNode.clean()
	c.releaseLeader()

	c.aliveNodes.Range(func(key, value interface{}) bool {
		node := value.(*Node)
		if node.name != c.GetMyName() && node.connection != nil {
			go node.connection.Close()
		}
		return true
	})

	if c.server != nil {
		go c.server.Close()
	}

	c.createEvent(eventNameClose, "")
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
	switch c.config.Mode {
	case modeCluster:
		return c.sate == leader || (c.sate == follower && c.curNode.heartbeat.Add(time.Duration(c.config.Timeout)*time.Second).After(time.Now()))
	case modeRedis:
		return c.GetLeaderName() != ""
	default:
		return c.sate == leader || (c.sate == follower && c.curNode.heartbeat.Add(time.Duration(c.config.Timeout)*time.Second).After(time.Now()))
	}
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
	if c.curNode == nil {
		return ""
	}
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

func (c *Cluster) AddJobTracker(v JobTracker) error {
	if v == nil {
		return errors.New("invalid job tracker")
	}
	c.jobTrackers.Store(v.Name(), v)
	return nil
}

func (c *Cluster) RemoveJobTracker(v JobTracker) {
	if v == nil {
		return
	}
	c.jobTrackers.Delete(v.Name())
}

func (c *Cluster) loadNodes() {
	var keys []interface{}
	c.allNode.Range(func(key, value interface{}) bool {
		keys = append(keys, key)
		return true
	})
	for _, key := range keys {
		c.allNode.Delete(key)
	}

	for _, n := range c.config.Nodes {
		node := newNode(n.Ip+":"+strconv.Itoa(n.Port), n.Name)
		c.allNode.Store(n.Name, node)

		// 如果开启了调试
		if n.Local {
			c.curNode = node
		}
	}

	var addresses []string
	replicasDiscovery := &c.config.ReplicasDiscovery
	if c.enableReplicasDiscovery() {
		if replicasDiscovery.Replicas <= 0 {
			replicasDiscovery.Replicas = env.GetReplicas()
		}
		for i := 0; i < replicasDiscovery.Replicas; i++ {
			domain := strings.Replace(replicasDiscovery.DomainPatten, "{suf}", strconv.Itoa(i), 1)
			address := fmt.Sprintf("%s:%d", domain, replicasDiscovery.Port)
			node := newNode(address, domain)
			c.allNode.Store(domain, node)
			addresses = append(addresses, address)
		}
	}

	enable, _ := strconv.ParseBool(c.config.Enable)
	if enable {
		c.logger.Info("[cluster] loadNodes success, nodes = %+v, addresses: %+v", c.GetAllNodeNames(), addresses)
	}
}

func (c *Cluster) enableReplicasDiscovery() bool {
	return c.config.ReplicasDiscovery.DomainPatten != "" && c.config.ReplicasDiscovery.Port > 0
}

func (c *Cluster) findCurNode() {
	if c.curNode == nil { // 说明没有开启调试
		if c.config.Mode == modeSingle {
			c.curNode = newNode("127.0.0.1:10000", "single")
		} else {
			dns := env.GetLocalDNS()
			ip := env.GetLocalHostIP()
			c.allNode.Range(func(key, value interface{}) bool {
				node := value.(*Node)
				if dns != "" && strings.Contains(node.address, dns) {
					c.curNode = node
					return false
				}
				if ip != "" && strings.Contains(node.address, ip) {
					c.curNode = node
					return false
				}
				return true
			})
		}
	}
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
	defer e.OnError("cluster fighting")

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
			c.logger.Info("[cluster] do fighting again.")
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

		// 重新加载在线节点
		c.reconnect()

		count := c.GetAliveNodeCount()
		haftAllNodeCount := c.GetAllNodeCount() / 2
		logger.Info("[cluster] node name: %s, alive node count: %d, half count: %d", c.curNode.name, count, haftAllNodeCount)

		if count > haftAllNodeCount { // 如果否，说明该节点可能已经失联
			messages := c.sendMsgWhitTimeout(2*time.Second, messageAskLeaderReq, &Message{NodeName: c.curNode.name, Term: c.term})
			if len(messages) < haftAllNodeCount {
				// 再问一遍
				continue
			}

			leaderNode := ""
			term := 0
			for _, message := range messages {
				// 以 term 大的为主
				if message.Term >= term {
					term = message.Term
					leaderNode = message.LeaderNodeName
				}
			}

			if leaderNode != "" {
				var node *Node
				if v, ok := c.aliveNodes.Load(leaderNode); ok {
					node, ok = v.(*Node)
				} else if leaderNode == c.curNode.name {
					node = c.curNode
				}

				if c.signLeader(node, term) {
					return
				}
			}

			// 加速周期
			if term >= c.GetMyTerm() {
				c.logger.Info("[cluster] sign my term to %d from other node.", term)
				c.term = term
				break
			}
		}

		if sleepTimes%10 == 0 {
			c.logger.Warn("[cluster] aliveNode len: %d, no enough node to fighting.", count)
		}
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
	c.createEvent(eventNameElectionStart, "")
	defer func() {
		c.logger.Info("[cluster] get votes finished. alive node count: %d, node[%v]; all node count: %d", c.GetAliveNodeCount(), c.GetAliveNodeNames(), c.GetAllNodeCount())
		c.createEvent(eventNameElectionFinish, c.GetLeaderName())
	}()

	// 开始获取选票
	for {
		if c.IsReady() || c.IsClose() {
			logger.Info("cluster is ready or close")
			return
		}

		c.reconnect()

		count := c.GetAliveNodeCount()
		haftAllNodeCount := c.GetAllNodeCount() / 2
		logger.Info("[cluster] node name: %s, alive node count: %d, half count: %d", c.curNode.name, count, haftAllNodeCount)

		if count > haftAllNodeCount {
			myNodeName := c.GetMyName()
			c.sate = candidate

			// vote myself first
			if !c.voteNode(nextTerm, myNodeName) {
				c.logger.Info("[cluster] vote myself failed, term: %d already vote for other", nextTerm)
				return
			}

			// get vote from other node
			votesCount := 1
			messages := c.sendMsgWhitTimeout(2*time.Second, messageAskVoteReq, &Message{NodeName: myNodeName, Term: nextTerm})
			for _, message := range messages {
				if message.VoteNodeName == myNodeName {
					votesCount++
				}
			}
			c.logger.Info("[cluster] term %d vote myself finished, accept vote number %d", nextTerm, votesCount)

			if votesCount > haftAllNodeCount {
				// 开始广播自己为 leader
				messages1 := c.sendMsgWhitTimeout(2*time.Second, messageBroadcastLeaderReq, &Message{NodeName: myNodeName, Term: nextTerm, LeaderNodeName: c.curNode.name})
				if len(messages1) < haftAllNodeCount {
					logger.Info("[cluster] send leader broadcast failed, count: %d", len(messages1))
					return
				}

				success := true
				for _, message := range messages1 {
					if message.Success == false {
						success = false
					}
				}

				if success && c.signLeader(c.curNode, nextTerm) {
					c.logger.Info("[cluster] sign myself: %s to be leader. ", c.GetMyName())
					return
				}

				c.logger.Info("[cluster] broadcast myself to leader failed, go to next term...")
			}

			// fix bug, go to next term
			return
		}

		if sleepTimes%10 == 0 {
			c.logger.Warn("[cluster] aliveNode len: %d, no enough node to fighting.", count)
		}

		sleepTimes++
		time.Sleep(2 * time.Second)
	}
}

func (c *Cluster) heartbeat() {
	defer e.OnError("cluster heartbeat")

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
					c.releaseWithNodeName(c.GetMyName())
				} else {
					success := 0
					for _, message := range messages {
						if message.Success {
							success++
						}
					}
					if success < leastCnt {
						c.logger.Info("[cluster] heartbeat len: %d, but least need: %d, releaseLeader %s", success, leastCnt, c.GetMyName())
						c.releaseWithNodeName(c.GetMyName())
					}
				}
			} else if c.IsFollower() {
				// 如果集群不就绪
				if !c.IsReady() || c.leaderNode == nil {
					c.logger.Info("[cluster] follower %s is not ready. go fighting.", c.GetMyName())
					go c.fighting()
				}
			}
		}
	}
}

func (c *Cluster) sendMsgWhitTimeout(timeout time.Duration, flag uint8, msg *Message) []*Message {
	withTimeout, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	c.aliveNodes.Range(func(key, value interface{}) bool {
		node := value.(*Node)
		if node.name == c.GetMyName() {
			return true
		}
		go func(n *Node) {
			if err := n.connection.Write(flag, msg); err != nil {
				c.logger.Error("[cluster] send message to %s error: %v", n.address, err)
			}
		}(node)
		return true
	})

	// 每次都重新生成，防止上次请求超时消息到这次里面来了
	c.msgChan = make(chan *Message, c.GetAliveNodeCount())
	msgResponseList := make([]*Message, 0)

	// 添加定时器避免忙等待
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-withTimeout.Done():
			return msgResponseList
		case m := <-c.msgChan:
			msgResponseList = append(msgResponseList, m)
			if len(msgResponseList) == c.GetAliveNodeCount() {
				return msgResponseList
			}
		case <-ticker.C:
			// 定期检查是否收集完所有响应
			if len(msgResponseList) == c.GetAliveNodeCount() {
				return msgResponseList
			}
		}
	}
}

func (c *Cluster) signLeader(node *Node, term int) bool {
	c.leaderLock.Lock()
	defer c.leaderLock.Unlock()

	if node == nil {
		return false
	}

	if c.GetMyTerm() <= term {
		c.releaseLeaderNoLock()

		if node.name == c.GetMyName() {
			c.sate = leader
			c.createEvent(eventNameSignMaster, node.name)
			if c.config.Mode == modeSingle {
				c.createEvent(eventNameSignFollower, node.name)
			}
		} else {
			c.sate = follower
			c.curNode.heartbeat = time.Now()
			c.createEvent(eventNameSignFollower, node.name)
		}

		c.lostLeaderTime = time.Time{}
		c.leaderNode = node
		c.leaderName = node.name
		c.term = term
		return true
	}

	return false
}

func (c *Cluster) releaseWithNodeName(name string) {
	c.leaderLock.Lock()
	defer c.leaderLock.Unlock()

	if c.sate != closed {
		c.sate = follower
	}

	if c.GetLeaderName() == name {
		c.lostLeaderTime = time.Now()
		if c.leaderName == c.GetMyName() {
			c.createEvent(eventNameStopMaster, "")
		} else {
			c.createEvent(eventNameReleaseMaster, "")
		}
		c.leaderNode = nil
		c.leaderName = ""
	}
}

func (c *Cluster) releaseLeader() {
	c.leaderLock.Lock()
	defer c.leaderLock.Unlock()
	if c.sate != closed {
		c.sate = follower
	}
	c.releaseLeaderNoLock()
}

func (c *Cluster) releaseLeaderNoLock() {
	if c.leaderNode != nil {
		c.lostLeaderTime = time.Now()
		if c.leaderName == c.GetMyName() {
			c.createEvent(eventNameStopMaster, "")
		} else {
			c.createEvent(eventNameReleaseMaster, "")
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

	if voteForNodeName, ok := c.votesMap[term]; !ok {
		c.votesMap[term] = nodeName
		return true
	} else {
		return voteForNodeName == nodeName
	}
}

func (c *Cluster) createEvent(name, leaderName string) {
	c.events <- &event{name, c.sate, c.GetMyName(), leaderName}
}

func (c *Cluster) consumeEvent() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("%s [ERROR] - Got a runtime error %s. %s\n%s", time.Now().Format("2006-01-02 15:04:05"), "consumeEvent", r, string(debug.Stack()))
			go c.consumeEvent()
		}
	}()

	for {
		select {
		case <-c.ctx.Done():
			return
		case ev := <-c.events:
			switch ev.name {
			case eventNameStartUp:
				c.logger.Debug("[cluster] %s start up event, cluster status: %s", ev.nodeName, getStatusName(ev.clusterStat))
			case eventNameSignFollower:
				c.logger.Debug("[cluster] %s sign follower event, cluster status: %s", ev.nodeName, getStatusName(ev.clusterStat))
				c.jobTrackers.Range(func(key, value interface{}) bool {
					jobTracker := value.(JobTracker)
					go jobTracker.OnNewLeader(ev.leaderName)
					return true
				})
			case eventNameSignMaster:
				c.logger.Debug("[cluster] %s sign master event, cluster status: %s", ev.nodeName, getStatusName(ev.clusterStat))
				c.jobTrackers.Range(func(key, value interface{}) bool {
					jobTracker := value.(JobTracker)
					go jobTracker.OnStartedLeading()
					return true
				})
			case eventNameStopMaster:
				c.logger.Debug("[cluster] %s stop master event, cluster status: %s", ev.nodeName, getStatusName(ev.clusterStat))
				c.jobTrackers.Range(func(key, value interface{}) bool {
					jobTracker := value.(JobTracker)
					go jobTracker.OnStoppedLeading()
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
					go tracker.OnReleaseMaster()
					return true
				})
			case eventNameClose:
				c.logger.Debug("[cluster] %s close event, cluster status: %s", ev.nodeName, getStatusName(ev.clusterStat))
			default:
				c.logger.Warn("[cluster] unknown type %s event", ev.name)
			}
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
	if !c.IsReady() && !f.ignoreClusterNotReady {
		return nil, errors.New("cluster is not ready")
	}

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

func (c *Cluster) reloadAllNodes(data interface{}) (interface{}, error) {
	if replicas, ok := data.(int); ok {
		c.config.ReplicasDiscovery.Replicas = replicas
	} else {
		err := tools.Unmarshal([]byte(tools.ToJson(data)), &c.config.ReplicasDiscovery.Replicas)
		if err != nil {
			c.logger.Error("[cluster] reloadAllNodes failed. Error: %v", err)
			return nil, errors.New(fmt.Sprintf("%s reloadAllNodes failed", c.GetMyName()))
		}
	}

	c.logger.Info("[cluster] do reloadAllNodes, replicas: %d", c.config.ReplicasDiscovery.Replicas)
	c.loadNodes()
	c.reconnect()
	return nil, nil
}
