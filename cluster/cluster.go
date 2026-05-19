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
	"math/rand"
	"net"
	"os"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/caiflower/common-tools/global"
	"github.com/caiflower/common-tools/global/env"
	"github.com/caiflower/common-tools/pkg/bean"
	"github.com/caiflower/common-tools/pkg/cache"
	"github.com/caiflower/common-tools/pkg/safego"
	"github.com/caiflower/common-tools/pkg/shell"
	redisv1 "github.com/caiflower/common-tools/redis/v1"

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

const (
	_init uint32 = iota
	follower
	candidate
	leader
	closed

	modeCluster = "cluster"
	modeSingle  = "single"
	modeRedis   = "redis"

	remoteFuncNameOfReloadAllNodes = "cluster.ReloadAllNodes"
)

type Config struct {
	Mode    string        `yaml:"mode" default:"cluster" json:"mode"`
	Timeout time.Duration `yaml:"timeout" default:"5s" json:"timeout"`
	Enable  string        `yaml:"enable" default:"true" json:"enable"`
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
	BeanName            string        `yaml:"beanName"`                          // 如果为空，则Ioc分配
	DataPath            string        `yaml:"dataPath"`                          // redis key 前缀
	Port                int           `yaml:"port" default:"8081"`               // 节点通信端口
	ElectionInterval    time.Duration `yaml:"electionInterval" default:"15s"`    //多久进行一次选主/续约
	ElectionPeriod      time.Duration `yaml:"electionPeriod" default:"30s"`      //选主/续约后有效租期时间
	SyncLeaderInterval  time.Duration `yaml:"syncLeaderInterval" default:"10s"`  //多久同步一次leader
	NodeRegisterTTL     time.Duration `yaml:"nodeRegisterTTL" default:"60s"`     // 节点注册信息的过期时间
	NodeSyncInterval    time.Duration `yaml:"nodeSyncInterval" default:"30s"`    // 多久同步一次节点信息
	NodeHeartbeatPeriod time.Duration `yaml:"nodeHeartbeatPeriod" default:"20s"` // 节点心跳续约周期
}

type ReplicasDiscovery struct {
	DomainPatten string        `yaml:"domainPatten"`
	Port         int           `yaml:"port" default:"8081"`
	PollInterval time.Duration `yaml:"pollInterval" default:"30s"` // HPA 支持：轮询 headless service DNS 的间隔
}

type Cluster struct {
	lock               sync.Locker                                            // 启动关闭锁
	fightingState      uint32                                                 // 选举状态锁（modeCluster使用）
	redisFightingState uint32                                                 // 选举状态锁（modeRedis使用）
	redisWatchDogState uint32                                                 // WatchDog状态锁（modeRedis使用）
	config             *Config                                                // 配置文件
	curNode            *Node                                                  // 当前节点
	leaderNode         *Node                                                  // 领导节点
	leaderLock         sync.RWMutex                                           // leader锁
	lostLeaderTime     time.Time                                              // 没有leader的时间
	allNode            *sync.Map                                              // 所有的节点
	aliveNodes         *sync.Map                                              // 所有存活的节点
	term               int                                                    // 当前任期
	sate               uint32                                                 // 集群状态
	server             nio.IServer                                            // 服务端口
	logger             logger.ILog                                            // 日志框架
	msgChan            atomic.Value                                           // 消息通信chan (存储 chan *Message，per-call)
	votesMap           map[int]string                                         // 投票map term->nodeName
	votesLock          sync.Locker                                            // 投票锁
	localFuncs         map[string]func(data interface{}) (interface{}, error) // 本地函数
	Redis              redisv1.RedisClient                                    `autowired:"" conditional_on_property:"default.cluster.mode=redis"` // redis
	ctx                context.Context
	cancelFunc         context.CancelFunc
	events             chan *event
	jobTrackers        *sync.Map
	closeSuccess       chan bool
	currentReplicas    atomic.Value
	connectLock        sync.Mutex
}

func NewClusterWithArgs(config Config, logger logger.ILog) (*Cluster, error) {
	_ = tools.DoTagFunc(&config, []tools.FnObj{{Fn: tools.SetDefaultValueIfNil}})

	if logger == nil {
		return nil, errors.New("logger required")
	}

	switch config.Mode {
	case modeCluster, modeSingle, modeRedis:
	default:
		config.Mode = modeCluster
	}

	cluster := &Cluster{
		config:      &config,
		allNode:     &sync.Map{},
		aliveNodes:  &sync.Map{},
		term:        0,
		sate:        _init,
		logger:      logger,
		lock:        syncx.NewSpinLock(),
		votesMap:    make(map[int]string),
		votesLock:   syncx.NewSpinLock(),
		jobTrackers: &sync.Map{},
		localFuncs:  make(map[string]func(data interface{}) (interface{}, error)),
	}

	if !cluster.IsEnable() {
		return cluster, nil
	}

	// 初始化节点信息
	cluster.loadNodes()
	// find curNode
	cluster.findCurNode()
	cluster.aliveNodes.Store(cluster.GetMyName(), cluster.GetMyNode())

	if cluster.curNode == nil {
		return nil, errors.New("can not find local node")
	}

	cluster.logger.Info("[cluster] local node address: %s", cluster.curNode.address)

	// redis beanName
	if config.Mode == modeRedis && config.RedisDiscovery.BeanName != "" {
		if b := bean.GetBean(config.RedisDiscovery.BeanName); b == nil {
			panic(fmt.Sprintf("[cluster] redis mode, can not find redis bean %s.", config.RedisDiscovery.BeanName))
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
	sate := atomic.LoadUint32(&c.sate)
	if !c.IsEnable() || (sate > _init && sate != closed) {
		return nil
	}
	if c.GetMyNode() == nil {
		return errors.New("start cluster failed. can not find current node")
	}

	c.term = 0
	atomic.StoreUint32(&c.sate, follower)

	ctx, cancelFunc := context.WithCancel(context.Background())
	c.cancelFunc = cancelFunc
	c.ctx = ctx

	if c.config.Mode != modeSingle {
		// 开启服务监听端口
		c.listen()
		// 集群建立连接
		c.reconnect()
	}

	// HPA 支持：启动时通过 headless service DNS 重新加载节点，并启动后台轮询
	if c.enableReplicasDiscovery() {
		c.reloadAllNodes(nil)
		go c.watchReplicas()

		for _, v := range c.GetAliveNodeNames() {
			if v != c.GetMyName() {
				if _, err := c.CallFunc(NewFuncSpec(v, remoteFuncNameOfReloadAllNodes, nil, 2*time.Second).IgnoreNotReady()); err != nil {
					c.logger.Error("[cluster] call %s reload nodes failed. %s", v, err.Error())
				}
			}
		}
	}

	c.events = make(chan *event, 100)

	switch c.config.Mode {
	case modeSingle:
		defer func() {
			// sign leader
			c.signLeader(c.GetMyNode(), 0)
			// sent follower event
			c.createEvent(eventNameSignFollower, c.curNode.name)
		}()

	case modeRedis:
		c.redisClusterStartUp()
	case modeCluster:
		fallthrough
	default:
		go c.fighting()
		go c.heartbeat()
	}

	// 开始消费事件
	go c.consumeEvent()

	c.closeSuccess = make(chan bool)
	c.createEvent(eventNameStartUp, "")

	c.logger.Info("[cluster] startup success. ")

	return nil
}

func (c *Cluster) Close() {
	if c.IsClosed() || !c.IsEnable() {
		return
	}

	c.lock.Lock()
	defer c.lock.Unlock()

	if c.IsClosed() {
		return
	}

	// 缩容或重启时，其他节点会通过各自的心跳/发现机制感知到拓扑变化，无需显式通知

	if c.cancelFunc != nil {
		c.cancelFunc()
	}
	c.curNode.clean()
	c.releaseLeader()

	atomic.StoreUint32(&c.sate, closed)

	// close nio
	c.aliveNodes.Range(func(key, value interface{}) bool {
		node := value.(*Node)
		if node.name != c.GetMyName() {
			node.close()
		}
		return true
	})

	if c.server != nil {
		c.server.Close()
	}

	c.createEvent(eventNameClose, "")
	close(c.events)

	c.jobTrackers.Range(func(key, value interface{}) bool {
		closer, ok := value.(global.Resource)
		if ok {
			closer.Close()
		}
		return true
	})

	<-c.closeSuccess
	c.logger.Info("[cluster] close success. ")
}

func (c *Cluster) IsClosed() bool {
	return atomic.LoadUint32(&c.sate) == closed
}

func (c *Cluster) IsReady() bool {
	sate := atomic.LoadUint32(&c.sate)
	switch c.config.Mode {
	case modeRedis:
		return c.GetLeaderName() != ""
	case modeCluster:
		fallthrough
	default:
		return sate == leader || (sate == follower && c.isNodeHealthy(c.curNode))
	}
}

// 增强的节点健康检查
func (c *Cluster) isNodeHealthy(node *Node) bool {
	if node == nil || node.isHeartbeatZero() {
		return false
	}

	if !node.isReady(c.config.Timeout) {
		return false
	}

	if node.getHealthScore() < 30 {
		return false
	}

	// 检查连续失败次数
	if node.getHeartbeatFailures() > 3 {
		return false
	}

	return true
}

// 获取节点健康状态详情
//func (c *Cluster) getNodeHealthStatus(nodeName string) map[string]interface{} {
//	node := c.GetNodeByName(nodeName)
//	if node == nil {
//		return map[string]interface{}{
//			"healthy": false,
//			"reason":  "node not found",
//		}
//	}
//
//	return map[string]interface{}{
//		"healthy":       c.isNodeHealthy(node),
//		"healthScore":   node.getHealthScore(),
//		"lastHeartbeat": node.heartbeat.Format("2006-01-02 15:04:05"),
//		"failures":      node.heartbeatFailures,
//		"lastOk":        node.lastHeartbeatOk,
//		"ageSeconds":    int(time.Since(node.heartbeat).Seconds()),
//		"hasConnection": node.connection != nil,
//	}
//}

func (c *Cluster) IsLeader() bool {
	return atomic.LoadUint32(&c.sate) == leader
}

func (c *Cluster) IsCandidate() bool {
	return atomic.LoadUint32(&c.sate) == candidate
}

func (c *Cluster) IsFollower() bool {
	return atomic.LoadUint32(&c.sate) == follower
}

func (c *Cluster) GetLeaderNode() *Node {
	c.leaderLock.RLock()
	defer c.leaderLock.RUnlock()
	return c.leaderNode
}

func (c *Cluster) GetLeaderName() string {
	node := c.GetLeaderNode()
	if node == nil {
		return ""
	}
	return node.name
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
	return atomic.LoadUint32(&c.sate) == closed
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
	// clear all node and reload
	c.allNode.Clear()

	var addresses []string
	replicasDiscovery := &c.config.ReplicasDiscovery

	// Redis 模式：节点信息从 Redis 中动态获取，跳过配置文件加载
	if c.config.Mode == modeRedis {
		c.logger.Info("[cluster] redis mode: nodes will be discovered from Redis dynamically")
		return
	}

	if c.enableReplicasDiscovery() {
		replicas := c.discoverReplicasFromDNS()
		if replicas <= 0 {
			replicas = env.GetReplicas()
		}
		c.currentReplicas.Store(replicas)

		c.logger.Info("replicas discovery enabled, current replicas: %d", replicas)
		for i := 0; i < replicas; i++ {
			domain := strings.Replace(replicasDiscovery.DomainPatten, "{suf}", strconv.Itoa(i), 1)
			address := fmt.Sprintf("%s:%d", domain, replicasDiscovery.Port)
			node := newNode(address, domain, c.config.Timeout.Seconds()/3)
			c.allNode.Store(domain, node)
			addresses = append(addresses, address)
		}
	} else {
		for _, n := range c.config.Nodes {
			address := n.Ip + ":" + strconv.Itoa(n.Port)
			node := newNode(address, n.Name, c.config.Timeout.Seconds()/3)
			c.allNode.Store(n.Name, node)
			addresses = append(addresses, address)

			// debug
			if n.Local {
				c.curNode = node
			}
		}
	}

	if c.IsEnable() {
		c.logger.Info("[cluster] loadNodes success, nodes = %+v, addresses: %+v", c.GetAllNodeNames(), addresses)
	}
}

func (c *Cluster) IsEnable() bool {
	enable, _ := strconv.ParseBool(c.config.Enable)
	return enable
}

func (c *Cluster) enableReplicasDiscovery() bool {
	return c.config.ReplicasDiscovery.DomainPatten != "" && c.config.ReplicasDiscovery.Port > 0 && env.Kubernetes
}

func (c *Cluster) getQuorum() int {
	totalNodes := c.GetAllNodeCount()
	if totalNodes == 0 {
		return 1
	}
	return totalNodes/2 + 1
}

func (c *Cluster) findCurNode() {
	if c.curNode == nil { // 说明没有开启调试
		if c.config.Mode == modeSingle {
			c.curNode = newNode("127.0.0.1:10000", "single", c.config.Timeout.Seconds()/3)
		} else if c.config.Mode == modeRedis {
			// Redis 模式：根据本地信息创建当前节点
			dns := env.GetLocalDNS()
			ip := env.GetLocalHostIP()

			// 优先使用 DNS 名称作为节点名
			nodeName := dns
			if nodeName == "" {
				nodeName = ip
			}
			if nodeName == "" {
				hostname, _ := os.Hostname()
				if hostname != "" {
					nodeName = hostname
				} else {
					nodeName = fmt.Sprintf("redis-node-%d", time.Now().Unix())
				}
			}

			// 使用 RedisDiscovery 中配置的端口
			port := c.config.RedisDiscovery.Port
			if port == 0 {
				port = 8081 // 默认端口
			}

			address := fmt.Sprintf("%s:%d", ip, port)
			node := newNode(address, nodeName, c.config.Timeout.Seconds()/3)
			c.allNode.Store(nodeName, node)
			c.curNode = node

			c.logger.Info("[cluster] redis mode: set current node, name=%s, address=%s", nodeName, address)
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

			if c.curNode == nil {
				domain := env.GetLocalDNS()
				address := fmt.Sprintf("%s:%d", domain, c.config.ReplicasDiscovery.Port)
				node := newNode(address, domain, c.config.Timeout.Seconds()/3)
				c.allNode.Store(domain, node)
				c.curNode = node

				c.logger.Info("[cluster] set default localhost, address = %s", address)
			}
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
	c.connectLock.Lock()
	defer c.connectLock.Unlock()

	c.allNode.Range(func(key, value interface{}) bool {
		// 排除自己
		nodeName := key.(string)
		if _, ex := c.aliveNodes.Load(nodeName); c.curNode.name != nodeName && !ex {
			node := value.(*Node)
			client := nio.NewClientWithAllArgs(&nio.Config{
				Addr:    node.address,
				Timeout: 1,
			}, syncx.NewSpinLock(), c.logger, c.getClientHandler(nodeName))

			if err := client.Connect(); err != nil {
				c.logger.Error("[cluster] connect node %s failed. error: %v", node.address, err)
				return true
			}

			node.setConnection(client)
		}
		return true
	})

	c.updateMetrics(c.IsLeader())
}

func (c *Cluster) listen() {
	server := nio.NewServerWithAllArgs(&nio.Config{
		Addr: c.curNode.address,
	}, syncx.NewSpinLock(), c.logger, c.getServerHandler())

	if err := server.Open(); err != nil {
		c.logger.Error("[cluster] open server %s error: %v", c.curNode.address, err)
	} else {
		c.server = server
	}
}

func (c *Cluster) fighting() {
	c.fightingWithRetry(0)
}

func (c *Cluster) fightingWithRetry(retryCount int) {
	const alertThreshold = 5

	defer e.OnError("cluster fighting")

	if c.IsReady() || c.IsClose() {
		return
	}

	// 保证同时只有一个竞选过程在执行
	if !atomic.CompareAndSwapUint32(&c.fightingState, 0, 1) {
		return
	}
	defer atomic.StoreUint32(&c.fightingState, 0)

	// 如果集群已经就绪或者关闭了，那么直接返回即可
	if c.IsReady() || c.IsClose() {
		return
	}

	// 重试计数器监控
	if retryCount >= alertThreshold {
		c.logger.Warn("[cluster] high fighting retry count: %d for node %s", retryCount, c.GetMyName())
	}

	defer func() {
		if c.GetLeaderNode() != nil {
			c.logger.Info("[cluster] node name: %s term %d fighting finished. leader name: %s", c.curNode.name, c.term, c.GetLeaderName())
		} else {
			atomic.CompareAndSwapUint32(&c.sate, candidate, follower)

			if !c.IsClose() {
				// 指数退避，上限 200ms，避免 100 个节点同时重试造成风暴
				backoff := time.Duration((retryCount+1)*(retryCount+1)) * 20 * time.Millisecond
				if backoff > 200*time.Millisecond {
					backoff = 200 * time.Millisecond
				}
				c.logger.Debug("[cluster] fighting failed, retry %d after %v", retryCount+1, backoff)
				time.Sleep(backoff)
				go c.fightingWithRetry(retryCount + 1)
			}
		}
	}()

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
		quorum := c.getQuorum()
		c.logger.Info("[cluster] node name: %s, alive node count: %d, quorum: %d", c.curNode.name, count, quorum)

		if count >= quorum {
			messages := c.sendMsgWhitTimeout(500*time.Millisecond, messageAskLeaderReq, &Message{NodeName: c.curNode.name, Term: c.term})
			if len(messages) < quorum {
				// 跳过
				break
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
					node = v.(*Node)
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
		} else {
			if sleepTimes%10 == 0 {
				c.logger.Warn("[cluster] aliveNode len: %d, no enough node to fighting.", count)
			}
			sleepTimes++
		}

		time.Sleep(time.Duration(rand.Intn(20)) * time.Millisecond)
	}

	// 随机休眠一下，防止所有节点同时开始竞选
	time.Sleep(time.Duration(rand.Intn(50)) * time.Millisecond)

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
			c.logger.Info("cluster is ready or close")
			return
		}

		c.reconnect()

		count := c.GetAliveNodeCount()
		quorum := c.getQuorum()
		c.logger.Info("[cluster] node name: %s, alive node count: %d, quorum: %d", c.curNode.name, count, quorum)

		if count >= quorum {
			myNodeName := c.GetMyName()
			// attempt to sign myself to candidate
			if !atomic.CompareAndSwapUint32(&c.sate, follower, candidate) {
				return
			}

			// vote myself first
			if !c.voteNode(nextTerm, myNodeName) {
				c.logger.Info("[cluster] vote myself failed, term: %d already vote for other", nextTerm)
				return
			}

			// get vote from other node
			votesCount := 1
			messages := c.sendMsgWhitTimeout(500*time.Millisecond, messageAskVoteReq, &Message{NodeName: myNodeName, Term: nextTerm})
			for _, message := range messages {
				if message.Success && message.VoteNodeName == myNodeName {
					votesCount++
				}
			}
			c.logger.Info("[cluster] term %d vote myself finished, accept vote number %d", nextTerm, votesCount)

			if votesCount >= quorum {
				// 开始广播自己为 leader
				messages1 := c.sendMsgWhitTimeout(1000*time.Millisecond, messageBroadcastLeaderReq, &Message{NodeName: myNodeName, Term: nextTerm, LeaderNodeName: c.curNode.name})
				successCount := 1 // 自己算一票
				for _, message := range messages1 {
					if message.Success {
						successCount++
					}
				}

				// 只要广播到达了 quorum 个节点（包括自己），leader 就可以上任
				// 未响应或拒绝的 follower 会通过后续心跳感知到 leader
				if successCount >= quorum {
					if c.signLeader(c.curNode, nextTerm) {
						c.logger.Info("[cluster] sign myself: %s to be leader (broadcast ack %d/%d).", c.GetMyName(), successCount, c.GetAliveNodeCount())
						return
					}
				}

				c.logger.Info("[cluster] broadcast myself to leader failed (ack %d/%d quorum %d), go to next term...", successCount, len(messages1)+1, quorum)
			}

			// fix bug, go to next term
			return
		}

		if sleepTimes%10 == 0 {
			c.logger.Warn("[cluster] aliveNode len: %d, no enough node to fighting.", count)
		}

		sleepTimes++
		time.Sleep(time.Duration(rand.Intn(100)) * time.Millisecond)
	}
}

func (c *Cluster) heartbeat() {
	defer e.OnError("cluster heartbeat")

	leaderHeartbeatInterval := c.config.Timeout / 3
	const followerCheckInterval = 500 * time.Millisecond

	ticker := time.NewTicker(followerCheckInterval)
	defer ticker.Stop()

	consecutiveFailures := 0
	maxConsecutiveFailures := 3

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			if c.IsLeader() {
				c.logger.Trace("[cluster] leader %s send heartbeat.", c.GetMyName())
				messages := c.sendMsgWithBackoffTimeout(messageHeartbeatReq, &Message{NodeName: c.curNode.name, Term: c.term})

				quorum := c.getQuorum()
				leastCnt := quorum - 1

				if len(messages) < leastCnt {
					consecutiveFailures++
					c.logger.Warn("[cluster] heartbeat response insufficient: %d/%d, consecutive failures: %d", len(messages), leastCnt, consecutiveFailures)

					if consecutiveFailures >= maxConsecutiveFailures {
						c.logger.Info("[cluster] too many consecutive heartbeat failures, releaseLeader %s", c.GetMyName())
						c.releaseWithNodeName(c.GetMyName())
						consecutiveFailures = 0
					}
				} else {
					success := 0
					for _, message := range messages {
						if message.Success {
							success++
						}
					}

					if success < leastCnt {
						consecutiveFailures++
						c.logger.Warn("[cluster] heartbeat success rate low: %d/%d, consecutive failures: %d", success, leastCnt, consecutiveFailures)

						if consecutiveFailures >= maxConsecutiveFailures {
							c.logger.Info("[cluster] heartbeat success rate too low, releaseLeader %s", c.GetMyName())
							c.releaseWithNodeName(c.GetMyName())
							consecutiveFailures = 0
						}
					} else {
						consecutiveFailures = 0
						c.logger.Trace("[cluster] heartbeat successful: %d/%d responses", success, len(messages))
					}
				}

				ticker.Reset(leaderHeartbeatInterval)
			} else if c.IsFollower() {
				if !c.IsReady() || c.GetLeaderNode() == nil {
					c.logger.Info("[cluster] follower %s is not ready. go fighting.", c.GetMyName())
					go c.fighting()
				}
				ticker.Reset(followerCheckInterval)
			}

			c.reconnect()
		}
	}
}

func (c *Cluster) sendMsgWithBackoffTimeout(flag uint8, msg *Message) []*Message {
	baseTimeout := 1000 * time.Millisecond

	unhealthyNodes := 0
	totalNodes := 0

	c.aliveNodes.Range(func(key, value interface{}) bool {
		node := value.(*Node)
		if node.name != c.GetMyName() {
			totalNodes++
			if node.getHealthScore() < 30 {
				unhealthyNodes++
			}
		}
		return true
	})

	// 如果有不健康的节点，增加超时时间
	if unhealthyNodes > 0 && totalNodes > 0 {
		timeoutMultiplier := 1.0 + (float64(unhealthyNodes)/float64(totalNodes))*2.0
		baseTimeout = time.Duration(float64(baseTimeout) * timeoutMultiplier)
		maxTime := c.config.Timeout
		if baseTimeout > maxTime {
			baseTimeout = maxTime
		}
	}

	messages := c.sendMsgWhitTimeout(baseTimeout, flag, msg)
	if len(messages) != c.GetAllNodeCount() {
		c.allNode.Range(func(key, value interface{}) bool {
			find := false
			for _, message := range messages {
				if message.NodeName == key {
					find = true
					break
				}
			}

			node := value.(*Node)
			if !find && node.name != c.GetMyName() {
				c.logger.Warn("[cluster] send message to %s failed, node %s maybe is not alive.", node.name, node.name)
				node.updateHeartbeatFailed()
			}

			return true
		})
	}

	return messages
}

func (c *Cluster) sendMsgWhitTimeout(timeout time.Duration, flag uint8, msg *Message) []*Message {
	// 先快照当前存活节点，避免发送过程中 aliveNodes 变化导致计数不一致
	type nodeEntry struct{ node *Node }
	var targets []nodeEntry
	c.aliveNodes.Range(func(key, value interface{}) bool {
		node := value.(*Node)
		if node.name != c.GetMyName() {
			targets = append(targets, nodeEntry{node})
		}
		return true
	})

	// 每次调用创建独立 channel，注册为当前活跃 channel 供 handler 写入
	ch := make(chan *Message, len(targets)+1)
	c.msgChan.Store(ch)

	for _, t := range targets {
		go func(n *Node) {
			if err := n.sendMessage(flag, msg); err != nil {
				c.logger.Error("[cluster] send message to %s error: %v", n.address, err)
			}
		}(t.node)
	}

	msgResponseList := make([]*Message, 0, len(targets))
	withTimeout, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	for {
		select {
		case <-withTimeout.Done():
			return msgResponseList
		case m := <-ch:
			msgResponseList = append(msgResponseList, m)
			if len(msgResponseList) == len(targets) {
				return msgResponseList
			}
		}
	}
}

func (c *Cluster) signLeader(node *Node, term int) bool {
	var eventsToSend []struct{ name, leader string }

	c.leaderLock.Lock()

	if node == nil {
		c.leaderLock.Unlock()
		return false
	}

	if term < c.GetMyTerm() {
		c.leaderLock.Unlock()
		return false
	}

	// collect events from releaseLeaderNoLock without sending them under the lock
	if c.leaderNode != nil {
		c.lostLeaderTime = time.Now()
		if c.leaderNode.name == c.GetMyName() {
			eventsToSend = append(eventsToSend, struct{ name, leader string }{eventNameUnsignMaster, ""})
		} else {
			eventsToSend = append(eventsToSend, struct{ name, leader string }{eventNameUnsignFollower, ""})
		}
		c.leaderNode = nil
	}

	if node.name == c.GetMyName() {
		atomic.StoreUint32(&c.sate, leader)
		eventsToSend = append(eventsToSend, struct{ name, leader string }{eventNameSignMaster, node.name})
	} else {
		atomic.StoreUint32(&c.sate, follower)
		c.curNode.updateHeartbeat()
		eventsToSend = append(eventsToSend, struct{ name, leader string }{eventNameSignFollower, node.name})
	}

	c.lostLeaderTime = time.Time{}
	c.leaderNode = node
	c.term = term
	c.leaderLock.Unlock()

	for _, ev := range eventsToSend {
		c.createEvent(ev.name, ev.leader)
	}
	return true
}

func (c *Cluster) releaseWithNodeName(name string) {
	var evName string

	c.leaderLock.Lock()

	if c.leaderNode == nil || c.leaderNode.name != name {
		c.leaderLock.Unlock()
		return
	}

	if !atomic.CompareAndSwapUint32(&c.sate, leader, follower) {
		c.leaderLock.Unlock()
		return
	}

	c.lostLeaderTime = time.Now()
	if c.leaderNode.name == c.GetMyName() {
		evName = eventNameUnsignMaster
	} else {
		evName = eventNameUnsignFollower
	}
	c.leaderNode = nil
	c.leaderLock.Unlock()

	c.createEvent(evName, "")
}

func (c *Cluster) releaseLeader() {
	c.leaderLock.Lock()
	atomic.StoreUint32(&c.sate, follower)

	var evName string
	if c.leaderNode != nil {
		c.lostLeaderTime = time.Now()
		if c.leaderNode.name == c.GetMyName() {
			evName = eventNameUnsignMaster
		} else {
			evName = eventNameUnsignFollower
		}
		c.leaderNode = nil
	}
	c.leaderLock.Unlock()

	if evName != "" {
		c.createEvent(evName, "")
	}
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
	if c.IsClosed() && name != eventNameClose {
		return
	}
	defer func() { recover() }()
	c.events <- &event{name, atomic.LoadUint32(&c.sate), c.GetMyName(), leaderName}
}

func (c *Cluster) consumeEvent() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("%s [ERROR] - Got a runtime error %s. %s\n%s", time.Now().Format("2006-01-02 15:04:05"), "consumeEvent", r, string(debug.Stack()))
			go c.consumeEvent()
		}
	}()

	for ev := range c.events {
		switch ev.name {
		case eventNameStartUp:
			c.logger.Debug("[cluster] %s start up event, cluster status: %s", ev.nodeName, getStatusName(ev.clusterStat))
		case eventNameSignFollower:
			c.logger.Debug("[cluster] %s sign follower event, cluster status: %s", ev.nodeName, getStatusName(ev.clusterStat))
			c.updateMetrics(false)
			c.jobTrackers.Range(func(key, value interface{}) bool {
				jobTracker := value.(JobTracker)
				jobTracker.OnStartedFollowing(ev.leaderName)
				return true
			})
		case eventNameSignMaster:
			c.logger.Debug("[cluster] %s sign master event, cluster status: %s", ev.nodeName, getStatusName(ev.clusterStat))
			c.updateMetrics(true)
			c.jobTrackers.Range(func(key, value interface{}) bool {
				jobTracker := value.(JobTracker)
				jobTracker.OnStartedLeading()
				return true
			})
		case eventNameUnsignMaster:
			c.logger.Debug("[cluster] %s unsign master event, cluster status: %s", ev.nodeName, getStatusName(ev.clusterStat))
			c.updateMetrics(false)
			c.jobTrackers.Range(func(key, value interface{}) bool {
				jobTracker := value.(JobTracker)
				jobTracker.OnStoppedLeading()
				return true
			})
		case eventNameElectionStart:
			c.logger.Debug("[cluster] %s election start event, cluster status: %s", ev.nodeName, getStatusName(ev.clusterStat))
		case eventNameElectionFinish:
			c.logger.Debug("[cluster] %s election finish event, cluster status: %s", ev.nodeName, getStatusName(ev.clusterStat))
		case eventNameUnsignFollower:
			c.logger.Debug("[cluster] %s unsign follower event, cluster status: %s", ev.nodeName, getStatusName(ev.clusterStat))
			c.updateMetrics(false)
			c.jobTrackers.Range(func(key, value interface{}) bool {
				tracker := value.(JobTracker)
				tracker.OnStoppedFollowing()
				return true
			})
		case eventNameClose:
			close(c.closeSuccess)
			c.logger.Debug("[cluster] %s close event, cluster status: %s", ev.nodeName, getStatusName(ev.clusterStat))
		default:
			c.logger.Warn("[cluster] unknown type %s event", ev.name)
		}
	}
}

func getStatusName(s uint32) string {
	switch s {
	case closed:
		return "closed"
	case _init:
		return "init"
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
		c.logger.Trace("[%s] call local func '%s'", f.uuid, f.funcName)
		safego.Go(func() {
			c.callLocalFunc(f)
		})
	} else { // 远程调用
		c.logger.Trace("[%s] call remote func '%s - %s'", f.uuid, f.nodeName, f.funcName)
		c.callRemoteFunc(f)
	}

	cache.LocalCache.Set(remoteCall+f.uuid, f, f.timeout+(5*time.Second)) //写缓存，TTL时间比超时时间富余一些。

	f.wait()
	return f.result, f.err
}

func (c *Cluster) callLocalFunc(f *FuncSpec) {
	golocalv1.PutTraceID(f.traceId)
	defer golocalv1.Clean()

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
	msg := &remoteCallMessage{
		TraceID: f.traceId, UUID: f.uuid, FuncName: f.funcName, Param: f.param, Sync: f.sync,
	}
	if val, ok := c.aliveNodes.Load(f.nodeName); ok {
		_node := val.(*Node)
		if err := _node.sendMessage(messageRemoteCallReq, msg); err != nil {
			f.setResult(nil, fmt.Errorf("remote call failed. %w", err))
			c.logger.Error("[cluster] [remote call] %s failed. %s Cause of %s.", f.uuid, f.funcName, err)
		}
	} else {
		f.setResult(nil, fmt.Errorf("the node %s does not exist or is dead", f.nodeName))
	}
}

func (c *Cluster) reloadAllNodes(_ interface{}) (interface{}, error) {
	c.logger.Info("[cluster] do reloadAllNodes")
	c.loadNodes()
	c.reconnect()
	return nil, nil
}

// getHeadlessServiceDomain 从 DomainPatten 中提取 headless service 域名。
// 例如：algo-invoker-{suf}.algo-invoker-headless.pixon.svc.cluster.local
// 提取后：algo-invoker-headless.pixon.svc.cluster.local
func (c *Cluster) getHeadlessServiceDomain() string {
	pattern := c.config.ReplicasDiscovery.DomainPatten
	const placeholder = "{suf}."
	idx := strings.Index(pattern, placeholder)
	if idx < 0 {
		return ""
	}
	return pattern[idx+len(placeholder):]
}

// discoverReplicasFromDNS 通过查询 headless service DNS 获取当前 ready 的副本数。
// K8s headless service 的 DNS 解析返回所有 ready pod 的 IP 列表，len 即为副本数。
func (c *Cluster) discoverReplicasFromDNS() int {
	domain := c.getHeadlessServiceDomain()
	c.logger.Info("[cluster] discoverReplicasFromDNS: domain=%s", domain)

	if domain == "" {
		return 0
	}
	addrs, err := net.LookupHost(domain)
	if err != nil {
		c.logger.Error("[cluster] DNS lookup for headless service %s failed: %v", domain, err)

		// If DNS lookup failed, return the current replicas number.
		if v := c.currentReplicas.Load(); v != nil {
			return v.(int)
		}

		return 0
	}

	discoverHosts := func() int {
		cnt := 0
		result, err1 := shell.Exec("cat", "/etc/hosts")

		if err1 == nil {
			hostLines := strings.Split(result.Stdout.String(), "\n")
			for _, hostLine := range hostLines {
				if strings.Contains(hostLine, "cluster.local") {
					cnt++
				}
			}
		}
		return cnt
	}
	hostsLen := discoverHosts()

	c.logger.Debug("[cluster] discoverReplicasFromDNS, hostsLen=%d, addrsLen=%d", hostsLen, len(addrs))

	return max(len(addrs), hostsLen)
}

// watchReplicas 后台轮询 headless service DNS，检测 HPA 引起的副本数变化并触发集群拓扑更新。
func (c *Cluster) watchReplicas() {
	interval := c.config.ReplicasDiscovery.PollInterval
	if interval <= 0 {
		interval = 30 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	c.currentReplicas.Store(c.discoverReplicasFromDNS())

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			replicas := c.discoverReplicasFromDNS()
			currentReplicas := c.currentReplicas.Load().(int)

			if replicas <= 0 || replicas == currentReplicas {
				c.logger.Info("[cluster] discoverReplicasFromDNS: current replicas=%d", replicas)
				continue
			}

			c.logger.Info("[cluster] replicas changed: %d -> %d, reloading nodes", currentReplicas, replicas)
			c.currentReplicas.Store(replicas)
			c.reloadAllNodes(nil)
		}
	}
}
