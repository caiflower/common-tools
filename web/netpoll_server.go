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

package web

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/caiflower/common-tools/pkg/bean"
	"github.com/caiflower/common-tools/pkg/limiter"
	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/caiflower/common-tools/pkg/tools"
	"github.com/cloudwego/netpoll"
)

var DefaultNetpollHttpServer *NetpollHttpServer

type NetpollConfig struct {
	Config
	// 性能优化配置
	MaxConnections    int           `default:"10000"`
	BufferSize        int           `default:"8192"`
	ConnectionTimeout time.Duration `default:"30s"`
	MaxWorkers        int           `default:"1000"`
	WorkerQueueSize   int           `default:"10000"`
}

// ConnectionPool 连接池
type ConnectionPool struct {
	mu          sync.RWMutex
	connections map[netpoll.Connection]*PooledConnection
	maxSize     int
	currentSize int
}

// PooledConnection 池化连接
type PooledConnection struct {
	conn     netpoll.Connection
	lastUsed time.Time
	inUse    bool
	buffer   []byte
}

// WorkerJob 工作任务
type WorkerJob struct {
	conn   netpoll.Connection
	server *NetpollHttpServer
}

// Worker 工作协程
type Worker struct {
	workerPool chan chan *WorkerJob
	jobChannel chan *WorkerJob
	quit       chan bool
}

type NetpollHttpServer struct {
	config        *NetpollConfig
	logger        logger.ILog
	listener      netpoll.Listener
	handler       *handler
	limiterBucket limiter.Limiter
	ctx           context.Context
	cancel        context.CancelFunc

	bufferPool   *sync.Pool
	responsePool *sync.Pool
}

func InitDefaultNetpollHttpServer(config NetpollConfig) *NetpollHttpServer {
	if DefaultNetpollHttpServer != nil {
		return DefaultNetpollHttpServer
	}
	DefaultNetpollHttpServer = NewNetpollHttpServer(config)
	return DefaultNetpollHttpServer
}

func NewNetpollHttpServer(config NetpollConfig) *NetpollHttpServer {
	_ = tools.DoTagFunc(&config, nil, []func(reflect.StructField, reflect.Value, interface{}) error{tools.SetDefaultValueIfNil})

	ctx, cancel := context.WithCancel(context.Background())

	server := &NetpollHttpServer{
		config: &config,
		logger: logger.DefaultLogger(),
		ctx:    ctx,
		cancel: cancel,
	}

	// 初始化缓冲池
	server.bufferPool = &sync.Pool{
		New: func() interface{} {
			return make([]byte, config.BufferSize)
		},
	}

	// 初始化响应池
	server.responsePool = &sync.Pool{
		New: func() interface{} {
			return &NetpollResponseWriter{
				buffer: make([]byte, config.BufferSize),
			}
		},
	}

	// 初始化公共处理器
	initHandler(&config.Config, server.logger)
	server.handler = commonHandler

	return server
}

// NetpollConnection 连接包装器
type NetpollConnection struct {
	conn     netpoll.Connection
	lastUsed time.Time
}

func (s *NetpollHttpServer) AddController(v interface{}) {
	c, err := newController(v, s.config.ControllerRootPkgName, s.config.RootPath)
	if err != nil {
		logger.Warn("[AddController] add error: %s", err.Error())
		return
	}

	for _, path := range c.paths {
		logger.Info("Register action path %s?Action=xxx", path)
		s.handler.controllers[path] = c
	}

	if !bean.HasBean(bean.GetBeanNameFromValue(v)) {
		bean.AddBean(v)
	}
}

func (s *NetpollHttpServer) Register(controller *RestfulController) {
	path := "method:" + controller.method + "version:" + controller.version + "path:" + controller.path
	if _, e := s.handler.restfulPaths[path]; e {
		panic(fmt.Sprintf("Register restfulApi failed. RestfulPath method[%s] version[%s] path[%s] already exist. ", controller.method, controller.version, controller.path))
	}
	c := s.handler.controllers[controller.controllerName]
	if c != nil {
		controller.targetMethod = c.cls.GetMethod(c.cls.GetPkgName() + "." + controller.action)
		s.handler.restfulPaths[path] = struct{}{}
		s.handler.restfulControllers = append(s.handler.restfulControllers, controller)
	}
	if controller.targetMethod == nil {
		panic(fmt.Sprintf("Register restfulApi failed. Not found controller[%s] action[%s]. ", controller.controllerName, controller.action))
	}
	logger.Info("Register path %v, Method: %v", "/"+controller.version+controller.originPath, controller.method)
}

func (s *NetpollHttpServer) SetBeforeDispatchCallBack(callbackFunc BeforeDispatchCallbackFunc) {
	s.handler.beforeDispatchCallbackFunc = callbackFunc
}

func (s *NetpollHttpServer) AddInterceptor(i Interceptor, order int) {
	s.handler.interceptors = append(s.handler.interceptors, Item{
		Interceptor: i,
		Order:       order,
	})
}

func (s *NetpollHttpServer) Name() string {
	return fmt.Sprintf("NETPOLL_HTTP_SERVER:%s", s.config.Name)
}

func (s *NetpollHttpServer) Start() error {
	// 创建监听器
	addr := fmt.Sprintf(":%d", s.config.Port)
	listener, err := netpoll.CreateListener("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to create netpoll listener: %w", err)
	}
	s.listener = listener

	// 配置限流器
	if s.config.WebLimiter.Enable {
		getLimiterCallBack := func(qos int) limiter.Limiter {
			limiterBucket := limiter.NewXTokenBucket(qos, qos)
			return limiterBucket
		}
		s.limiterBucket = getLimiterCallBack(s.config.WebLimiter.Qos)
	}

	// 初始化性能监控
	metric := NewHttpMetric()
	s.handler.metric = metric

	s.logger.Info(
		"\n***************************** netpoll http server startup ***************************************\n"+
			"************* web service [name:%s] [rootPath:%s] listening on %d *********\n"+
			"*************************************************************************************************", s.config.Name, s.config.RootPath, s.config.Port)

	// 启动连接处理
	go s.serve()

	return nil
}

func (s *NetpollHttpServer) serve() {
	// 创建符合 OnRequest 接口的处理函数
	onRequest := func(ctx context.Context, connection netpoll.Connection) error {
		// 处理连接
		s.handleConnection(connection)
		return nil
	}

	// 使用 netpoll 的事件循环处理连接
	eventLoop, err := netpoll.NewEventLoop(
		onRequest,
		netpoll.WithReadTimeout(time.Duration(s.config.ReadTimeout)*time.Second),
		netpoll.WithWriteTimeout(time.Duration(s.config.WriteTimeout)*time.Second),
	)
	if err != nil {
		s.logger.Error("Failed to create event loop: %v", err)
		return
	}

	// 启动事件循环
	err = eventLoop.Serve(s.listener)
	if err != nil {
		s.logger.Error("Event loop serve error: %v", err)
	}
}

func (s *NetpollHttpServer) handleConnection(conn netpoll.Connection) {
	defer func() {
		if r := recover(); r != nil {
			s.logger.Error("Connection handler panic: %v", r)
		}
		_ = conn.Close()
	}()

	// 创建 HTTP 处理器
	httpHandler := &NetpollHttpHandler{
		server: s,
		conn:   conn,
	}

	// 处理 HTTP 请求
	httpHandler.ServeHTTP()
}

func (s *NetpollHttpServer) Close() {
	s.logger.Info("      **** netpoll http server shutdown ****")

	if s.cancel != nil {
		s.cancel()
	}

	if s.listener != nil {
		if err := s.listener.Close(); err != nil {
			s.logger.Warn(" **** netpoll http server shutdown error **** \n"+
				"**** error:%s ****", err.Error())
		}
		s.logger.Info(" **** netpoll http server gracefully shutdown ****")
	}

	s.limiterBucket = nil
}
