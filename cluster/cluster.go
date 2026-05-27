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

	"github.com/caiflower/common-tools/cluster/proto"
	"github.com/caiflower/common-tools/global"
	"github.com/caiflower/common-tools/global/env"
	"github.com/caiflower/common-tools/pkg/bean"
	"github.com/caiflower/common-tools/pkg/safego"
	"github.com/caiflower/common-tools/pkg/shell"
	redisv1 "github.com/caiflower/common-tools/redis/v1"
	gocache "github.com/patrickmn/go-cache"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"

	golocalv1 "github.com/caiflower/common-tools/pkg/golocal/v1"

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
	// Deprecated: Use cluster.CallFuncAs[T] instead to avoid deserialization failures with remote calls.
	CallFunc(fc *FuncSpec) (interface{}, error) // callFunc
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
	TLS     TLSConfig     `yaml:"tls" json:"tls"`
	Nodes   []*struct {
		Name  string
		Ip    string
		Port  int
		Local bool
	} `yaml:"nodes" json:"nodes"`
	RedisDiscovery    RedisDiscovery    `yaml:"redisDiscovery" json:"redisDiscovery"`
	ReplicasDiscovery ReplicasDiscovery `yaml:"replicasDiscovery" json:"replicasDiscovery"`
}

type TLSConfig struct {
	Enabled  bool   `yaml:"enabled" json:"enabled"`
	CertFile string `yaml:"certFile" json:"certFile"`
	KeyFile  string `yaml:"keyFile" json:"keyFile"`
	CAFile   string `yaml:"caFile" json:"caFile"`
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
	grpcServer         *grpc.Server                                           // gRPC 服务端
	logger             logger.ILog                                            // 日志框架
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
	callCache          *gocache.Cache
	heartbeatStreams   *heartbeatStreamManager
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
		callCache:   gocache.New(gocache.NoExpiration, 1*time.Minute),
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
	// init metrics
	initMetrics()

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
				if _, err := CallFuncAs[any](c, NewFuncSpec(v, remoteFuncNameOfReloadAllNodes, nil, 2*time.Second).IgnoreNotReady()); err != nil {
					c.logger.Error("[cluster] call %s reload nodes failed: %s", v, err.Error())
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

	c.logger.Info("[cluster] startup success")

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

	if c.heartbeatStreams != nil {
		c.heartbeatStreams.stopAll()
	}

	c.aliveNodes.Range(func(key, value interface{}) bool {
		node := value.(*Node)
		if node.name != c.GetMyName() {
			node.close()
		}
		return true
	})

	if c.grpcServer != nil {
		gracefulCh := make(chan struct{})
		go func() {
			c.grpcServer.GracefulStop()
			close(gracefulCh)
		}()
		select {
		case <-gracefulCh:
		case <-time.After(3 * time.Second):
			c.grpcServer.Stop()
		}
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
	c.logger.Info("[cluster] closed")
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

		c.logger.Info("[cluster] replicas discovery enabled, current replicas: %d", replicas)
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
		c.logger.Info("[cluster] loadNodes success, nodes=%+v, addresses=%+v", c.GetAllNodeNames(), addresses)
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

				c.logger.Info("[cluster] set default localhost, address=%s", address)
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

func (c *Cluster) markNodeUnavailable(nodeName string) {
	if v, ok := c.aliveNodes.LoadAndDelete(nodeName); ok {
		node := v.(*Node)
		if nodeName != c.GetMyName() {
			node.close()
		}
		c.logger.Warn("[cluster] node %s marked as unavailable", nodeName)
	}
}

// connect 集群建立连接
func (c *Cluster) reconnect() {
	c.connectLock.Lock()
	defer c.connectLock.Unlock()

	type connectResult struct {
		nodeName string
		node     *Node
		client   *grpcNodeClient
		err      error
	}

	var pending []connectResult
	c.allNode.Range(func(key, value interface{}) bool {
		nodeName := key.(string)
		if c.curNode.name == nodeName {
			return true
		}
		node := value.(*Node)
		if _, ex := c.aliveNodes.Load(nodeName); ex && node.getGrpcClient() != nil {
			return true
		}
		if node.getGrpcClient() != nil {
			node.close()
		}
		pending = append(pending, connectResult{nodeName: nodeName, node: node})
		return true
	})

	if len(pending) == 0 {
		return
	}

	resultCh := make(chan connectResult, len(pending))
	for i := range pending {
		go func(r connectResult) {
			client, err := newGrpcNodeClient(c.ctx, r.node.address, &c.config.TLS)
			resultCh <- connectResult{nodeName: r.nodeName, node: r.node, client: client, err: err}
		}(pending[i])
	}

	for range len(pending) {
		r := <-resultCh
		if r.err != nil {
			c.logger.Trace("[cluster] connect to %s failed: %v", r.node.address, r.err)
			c.aliveNodes.Delete(r.nodeName)
			continue
		}
		r.node.setGrpcClient(r.client)
		c.aliveNodes.Store(r.nodeName, r.node)
		c.logger.Trace("[cluster] %s connected, now alive", r.nodeName)
	}

	c.updateMetrics(c.IsLeader())
}

func (c *Cluster) listen() {
	lis, err := net.Listen("tcp", c.curNode.address)
	if err != nil {
		c.logger.Error("[cluster] listen on %s failed: %v", c.curNode.address, err)
		return
	}

	var serverOpts []grpc.ServerOption
	serverOpts = append(serverOpts,
		grpc.KeepaliveParams(keepalive.ServerParameters{
			Time:    30 * time.Second,
			Timeout: 5 * time.Second,
		}),
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             15 * time.Second,
			PermitWithoutStream: true,
		}),
	)
	if c.config.TLS.Enabled {
		creds, err := loadTLSServerCredentials(&c.config.TLS)
		if err != nil {
			c.logger.Error("[cluster] load TLS server credentials failed: %v", err)
			return
		}
		serverOpts = append(serverOpts, grpc.Creds(creds))
	}

	c.grpcServer = grpc.NewServer(serverOpts...)
	proto.RegisterClusterServiceServer(c.grpcServer, newClusterServiceServer(c))

	go func() {
		if err := c.grpcServer.Serve(lis); err != nil {
			c.logger.Error("[cluster] grpc server serve failed: %v", err)
		}
	}()
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
		c.logger.Warn("[cluster] high election retry count: %d for node %s", retryCount, c.GetMyName())
	}

	defer func() {
		if c.GetLeaderNode() != nil {
			c.logger.Info("[cluster] node %s term %d election finished, leader=%s", c.curNode.name, c.term, c.GetLeaderName())
		} else {
			atomic.CompareAndSwapUint32(&c.sate, candidate, follower)

			if !c.IsClose() {
				// 指数退避，上限 200ms，避免 100 个节点同时重试造成风暴
				backoff := time.Duration((retryCount+1)*(retryCount+1)) * 20 * time.Millisecond
				if backoff > 200*time.Millisecond {
					backoff = 200 * time.Millisecond
				}
				c.logger.Debug("[cluster] election failed, retry %d after %v", retryCount+1, backoff)
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
		c.logger.Info("[cluster] node %s, alive=%d, quorum=%d", c.curNode.name, count, quorum)

		if count >= quorum {
			messages := c.askLeaderFromNodes(500*time.Millisecond, c.curNode.name, c.term)
			if len(messages) < quorum {
				break
			}

			leaderNode := ""
			maxTerm := int32(0)
			for _, message := range messages {
				if message.Term >= maxTerm {
					maxTerm = message.Term
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

				if c.signLeader(node, int(maxTerm)) {
					return
				}
			}

			if int(maxTerm) >= c.GetMyTerm() {
				c.logger.Info("[cluster] update term to %d from other node", maxTerm)
				c.term = int(maxTerm)
				break
			}
		} else {
			if sleepTimes%10 == 0 {
				c.logger.Warn("[cluster] alive=%d, not enough nodes for election", count)
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
	c.logger.Info("[cluster] node %s term %d begin requesting votes", c.curNode.name, nextTerm)
	c.createEvent(eventNameElectionStart, "")
	defer func() {
		c.logger.Info("[cluster] vote finished, alive=%d, nodes=%v, total=%d", c.GetAliveNodeCount(), c.GetAliveNodeNames(), c.GetAllNodeCount())
		c.createEvent(eventNameElectionFinish, c.GetLeaderName())
	}()

	// 开始获取选票
	for {
		if c.IsReady() || c.IsClose() {
			c.logger.Debug("[cluster] cluster is ready or closed, skip voting")
			return
		}

		c.reconnect()

		count := c.GetAliveNodeCount()
		quorum := c.getQuorum()
		c.logger.Info("[cluster] node %s, alive=%d, quorum=%d", c.curNode.name, count, quorum)

		if count >= quorum {
			myNodeName := c.GetMyName()
			if !atomic.CompareAndSwapUint32(&c.sate, follower, candidate) {
				return
			}

			if !c.voteNode(nextTerm, myNodeName) {
				c.logger.Info("[cluster] self-vote failed, term %d already voted for other", nextTerm)
				return
			}

			// get vote from other node
			votesCount := 1
			messages := c.askVoteFromNodes(500*time.Millisecond, myNodeName, nextTerm)
			for _, message := range messages {
				if message.Success && message.VoteNodeName == myNodeName {
					votesCount++
				}
			}
			c.logger.Info("[cluster] term %d self-vote finished, received %d votes", nextTerm, votesCount)

			if votesCount >= quorum {
				messages1 := c.broadcastLeaderToNodes(1000*time.Millisecond, myNodeName, nextTerm, c.curNode.name)
				successCount := 1
				for _, message := range messages1 {
					if message.Success {
						successCount++
					}
				}

				if successCount >= quorum {
					if c.signLeader(c.curNode, nextTerm) {
						c.logger.Info("[cluster] node %s became leader (broadcast ack %d/%d)", c.GetMyName(), successCount, c.GetAliveNodeCount())
						return
					}
				}

				c.logger.Info("[cluster] broadcast leader failed (ack %d/%d, quorum %d), advancing to next term", successCount, len(messages1)+1, quorum)
			}

			// Failed to secure leadership (insufficient votes or broadcast acks), return to trigger next election attempt with incremented term
			return
		}

		if sleepTimes%10 == 0 {
			c.logger.Warn("[cluster] alive=%d, not enough nodes for election", count)
		}

		sleepTimes++
		time.Sleep(time.Duration(rand.Intn(100)) * time.Millisecond)
	}
}

func (c *Cluster) heartbeat() {
	defer e.OnError("cluster heartbeat")

	leaderHeartbeatInterval := c.config.Timeout / 3
	const followerCheckInterval = 500 * time.Millisecond

	c.heartbeatStreams = newHeartbeatStreamManager(c)

	ticker := time.NewTicker(followerCheckInterval)
	defer ticker.Stop()

	consecutiveFailures := 0
	maxConsecutiveFailures := 3

	for {
		select {
		case <-c.ctx.Done():
			c.heartbeatStreams.stopAll()
			return
		case <-ticker.C:
			if c.IsLeader() {
				c.logger.Trace("[cluster] leader %s sending heartbeat", c.GetMyName())

				c.aliveNodes.Range(func(key, value interface{}) bool {
					nodeName := key.(string)
					node := value.(*Node)
					if nodeName != c.GetMyName() {
						c.heartbeatStreams.startStream(node)
					}
					return true
				})

				quorum := c.getQuorum()
				leastCnt := quorum - 1

				success := 0
				total := 0
				c.aliveNodes.Range(func(key, value interface{}) bool {
					nodeName := key.(string)
					if nodeName != c.GetMyName() {
						total++
						if c.heartbeatStreams.sendHeartbeat(nodeName, int32(c.term)) {
							success++
						}
					}
					return true
				})

				if total < leastCnt || success < leastCnt {
					consecutiveFailures++
					c.logger.Warn("[cluster] heartbeat insufficient: %d/%d, consecutive failures: %d", success, leastCnt, consecutiveFailures)

					if consecutiveFailures >= maxConsecutiveFailures {
						c.logger.Info("[cluster] too many heartbeat failures, stepping down as leader: %s", c.GetMyName())
						c.releaseWithNodeName(c.GetMyName())
						consecutiveFailures = 0
					}
				} else {
					consecutiveFailures = 0
					c.logger.Trace("[cluster] heartbeat ok: %d/%d responses", success, total)
				}

				ticker.Reset(leaderHeartbeatInterval)
			} else if c.IsFollower() {
				c.heartbeatStreams.stopAll()
				if !c.IsReady() || c.GetLeaderNode() == nil {
					c.logger.Info("[cluster] follower %s not ready, triggering election", c.GetMyName())
					go c.fighting()
				}
				ticker.Reset(followerCheckInterval)
			}

			c.reconnect()
		}
	}
}

func broadcastToNodes[T any](c *Cluster, timeout time.Duration, sendFn func(ctx context.Context, n *Node) (*T, bool)) []*T {
	type nodeEntry struct{ node *Node }
	var targets []nodeEntry
	c.aliveNodes.Range(func(key, value interface{}) bool {
		node := value.(*Node)
		if node.name != c.GetMyName() {
			targets = append(targets, nodeEntry{node})
		}
		return true
	})

	ch := make(chan *T, len(targets))

	for _, t := range targets {
		go func(n *Node) {
			ctx, cancel := context.WithTimeout(c.ctx, timeout)
			defer cancel()

			respMsg, ok := sendFn(ctx, n)
			if !ok {
				return
			}

			if respMsg != nil {
				ch <- respMsg
			}
		}(t.node)
	}

	msgResponseList := make([]*T, 0, len(targets))
	withTimeout, cancel := context.WithTimeout(c.ctx, timeout)
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

func (c *Cluster) askLeaderFromNodes(timeout time.Duration, nodeName string, term int) []*proto.AskLeaderResponse {
	return broadcastToNodes(c, timeout, func(ctx context.Context, n *Node) (*proto.AskLeaderResponse, bool) {
		resp, err := n.askLeader(ctx, &proto.AskLeaderRequest{
			NodeName: nodeName,
			Term:     int32(term),
		})
		if err != nil {
			c.logger.Error("[cluster] AskLeader to %s failed: %v", n.address, err)
			c.markNodeUnavailable(n.name)
			return nil, false
		}
		return resp, true
	})
}

func (c *Cluster) askVoteFromNodes(timeout time.Duration, nodeName string, term int) []*proto.AskVoteResponse {
	return broadcastToNodes(c, timeout, func(ctx context.Context, n *Node) (*proto.AskVoteResponse, bool) {
		resp, err := n.askVote(ctx, &proto.AskVoteRequest{
			NodeName: nodeName,
			Term:     int32(term),
		})
		if err != nil {
			c.logger.Error("[cluster] AskVote to %s failed: %v", n.address, err)
			c.markNodeUnavailable(n.name)
			return nil, false
		}
		return resp, true
	})
}

func (c *Cluster) broadcastLeaderToNodes(timeout time.Duration, nodeName string, term int, leaderNodeName string) []*proto.BroadcastLeaderResponse {
	return broadcastToNodes(c, timeout, func(ctx context.Context, n *Node) (*proto.BroadcastLeaderResponse, bool) {
		resp, err := n.broadcastLeader(ctx, &proto.BroadcastLeaderRequest{
			NodeName:       nodeName,
			Term:           int32(term),
			LeaderNodeName: leaderNodeName,
		})
		if err != nil {
			c.logger.Error("[cluster] BroadcastLeader to %s failed: %v", n.address, err)
			c.markNodeUnavailable(n.name)
			return nil, false
		}
		return resp, true
	})
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
			c.logger.Debug("[cluster] %s started, status=%s", ev.nodeName, getStatusName(ev.clusterStat))
		case eventNameSignFollower:
			c.logger.Debug("[cluster] %s became follower, status=%s", ev.nodeName, getStatusName(ev.clusterStat))
			c.updateMetrics(false)
			c.jobTrackers.Range(func(key, value interface{}) bool {
				jobTracker := value.(JobTracker)
				jobTracker.OnStartedFollowing(ev.leaderName)
				return true
			})
		case eventNameSignMaster:
			c.logger.Debug("[cluster] %s became leader, status=%s", ev.nodeName, getStatusName(ev.clusterStat))
			c.updateMetrics(true)
			c.jobTrackers.Range(func(key, value interface{}) bool {
				jobTracker := value.(JobTracker)
				jobTracker.OnStartedLeading()
				return true
			})
		case eventNameUnsignMaster:
			c.logger.Debug("[cluster] %s stepped down as leader, status=%s", ev.nodeName, getStatusName(ev.clusterStat))
			c.updateMetrics(false)
			c.jobTrackers.Range(func(key, value interface{}) bool {
				jobTracker := value.(JobTracker)
				jobTracker.OnStoppedLeading()
				return true
			})
		case eventNameElectionStart:
			c.logger.Debug("[cluster] %s election started, status=%s", ev.nodeName, getStatusName(ev.clusterStat))
		case eventNameElectionFinish:
			c.logger.Debug("[cluster] %s election finished, status=%s", ev.nodeName, getStatusName(ev.clusterStat))
		case eventNameUnsignFollower:
			c.logger.Debug("[cluster] %s lost leader, status=%s", ev.nodeName, getStatusName(ev.clusterStat))
			c.updateMetrics(false)
			c.jobTrackers.Range(func(key, value interface{}) bool {
				tracker := value.(JobTracker)
				tracker.OnStoppedFollowing()
				return true
			})
		case eventNameClose:
			close(c.closeSuccess)
			c.logger.Debug("[cluster] %s closed, status=%s", ev.nodeName, getStatusName(ev.clusterStat))
		default:
			c.logger.Warn("[cluster] unknown event type: %s", ev.name)
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

// Deprecated: Use cluster.CallFuncAs[T] instead to avoid deserialization failures with remote calls.
func (c *Cluster) CallFunc(f *FuncSpec) (interface{}, error) {
	if !c.IsReady() && !f.ignoreClusterNotReady {
		return nil, errors.New("cluster is not ready")
	}

	f.startTimer()

	// 本地调用
	if c.GetMyNode().name == f.nodeName {
		c.logger.Trace("[%s] call local func '%s'", f.uuid, f.funcName)
		safego.Go(func() {
			c.callLocalFunc(f)
		})
	} else { // 远程调用
		c.logger.Trace("[%s] call remote func '%s' on node '%s'", f.uuid, f.funcName, f.nodeName)
		c.callRemoteFunc(f)
	}

	c.callCache.Set(remoteCall+f.uuid, f, f.timeout+cacheTTLExtension)
	f.onFinish = func() {
		c.callCache.Delete(remoteCall + f.uuid)
	}

	f.wait()
	return f.result, f.err
}

func (c *Cluster) callLocalFunc(f *FuncSpec) {
	golocalv1.PutTraceID(f.traceId)
	defer golocalv1.Clean()

	fc := c.localFuncs[f.funcName]
	if fc == nil {
		err := fmt.Errorf("not such function '%s' in the cluster", f.funcName)
		c.logger.Error("[cluster] [remote call] %s failed: function '%s' not found", f.uuid, f.funcName)
		f.setResult(nil, err)
		return
	}
	defer func() {
		if r := recover(); r != nil {
			c.logger.Error("[cluster] [remote call] runtime panic: %v\n%s", r, string(debug.Stack()))
			f.setResult(nil, fmt.Errorf("%s", r))
		}
	}()
	f.setResult(fc(f.param))
}

func (c *Cluster) callRemoteFunc(f *FuncSpec) {
	if val, ok := c.aliveNodes.Load(f.nodeName); ok {
		_node := val.(*Node)
		req, err := newRemoteCallRequest(f)
		if err != nil {
			f.setResult(nil, fmt.Errorf("remote call failed. %w", err))
			c.logger.Error("[cluster] [remote call] %s failed: %s, cause: %v", f.uuid, f.funcName, err)
			return
		}

		ctx, cancel := context.WithTimeout(c.ctx, f.timeout)
		defer cancel()

		resp, err := _node.remoteCall(ctx, req)
		if err != nil {
			f.setResult(nil, fmt.Errorf("remote call failed. %w", err))
			c.logger.Error("[cluster] [remote call] %s failed: %s, cause: %v", f.uuid, f.funcName, err)
			c.markNodeUnavailable(f.nodeName)
			return
		}

		result, _ := newRemoteCallResult(resp)
		var respErr error
		if resp.Err != "" {
			respErr = errors.New(resp.Err)
		}
		f.setResult(result, respErr)
	} else {
		f.setResult(nil, fmt.Errorf("node %s does not exist or is unreachable", f.nodeName))
	}
}

func (c *Cluster) reloadAllNodes(_ interface{}) (interface{}, error) {
	c.logger.Info("[cluster] reloading all nodes")
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
	c.logger.Info("[cluster] discovering replicas from DNS, domain=%s", domain)

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

	c.logger.Debug("[cluster] DNS discovery result, hosts=%d, addrs=%d", hostsLen, len(addrs))

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
				c.logger.Debug("[cluster] current replicas=%d, no change", replicas)
				continue
			}

			c.logger.Info("[cluster] replicas changed: %d -> %d, reloading nodes", currentReplicas, replicas)
			c.currentReplicas.Store(replicas)
			c.reloadAllNodes(nil)
		}
	}
}
