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

package webv1

import (
	"reflect"

	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/caiflower/common-tools/pkg/tools"
	"github.com/caiflower/common-tools/web/interceptor"
)

// ServerMode 服务器模式
type ServerMode string

const (
	ServerModeStandard ServerMode = "standard" // 标准 HTTP 服务器
	ServerModeNetpoll  ServerMode = "netpoll"  // Netpoll 高性能服务器
)

// UnifiedConfig 统一配置
type UnifiedConfig struct {
	// 基础配置
	Name                  string     `yaml:"name" default:"default"`
	Port                  uint       `yaml:"port" default:"8080"`
	ReadTimeout           uint       `yaml:"readTimeout" default:"20"`
	WriteTimeout          uint       `yaml:"writeTimeout" default:"35"`
	HandleTimeout         *uint      `yaml:"handleTimeout" default:"60"`
	RootPath              string     `yaml:"rootPath"`
	HeaderTraceID         string     `yaml:"headerTraceID" default:"X-Request-Id"`
	ControllerRootPkgName string     `yaml:"controllerRootPkgName" default:"controller"`
	WebLimiter            WebLimiter `yaml:"webLimiter"`
	EnablePprof           bool       `yaml:"enablePprof"`

	// 服务器模式配置
	Mode ServerMode `yaml:"mode" default:"standard"`

	// Netpoll 特有配置
	NetpollConfig NetpollSpecificConfig `yaml:"netpoll"`
}

// NetpollSpecificConfig Netpoll 特有配置
type NetpollSpecificConfig struct {
	MaxConnections     int  `yaml:"maxConnections" default:"10000"`
	IdleTimeout        uint `yaml:"idleTimeout" default:"300"`
	MaxRequestBodySize int  `yaml:"maxRequestBodySize" default:"4194304"`
	BufferSize         int  `yaml:"bufferSize" default:"8192"`
	WorkerPoolSize     int  `yaml:"workerPoolSize" default:"0"` // 0 表示使用 CPU 核数 * 2
	TaskQueueSize      int  `yaml:"taskQueueSize" default:"1000"`
	EnableMetrics      bool `yaml:"enableMetrics" default:"true"`
	MetricsInterval    uint `yaml:"metricsInterval" default:"30"` // 秒
}

// WebServer 统一服务器接口
type WebServer interface {
	// 基础方法
	Name() string
	Start() error
	Close()

	// 控制器管理
	AddController(v interface{})
	Register(controller *RestfulController)

	// 拦截器管理
	AddInterceptor(i interceptor.Interceptor, order int)
	SetBeforeDispatchCallBack(callbackFunc BeforeDispatchCallbackFunc)
}

// UnifiedWebServer 统一 Web 服务器
type UnifiedWebServer struct {
	config UnifiedConfig
	server WebServer
	logger logger.ILog
}

// NewUnifiedWebServer 创建统一 Web 服务器
func NewUnifiedWebServer(config UnifiedConfig) *UnifiedWebServer {
	_ = tools.DoTagFunc(&config, nil, []func(reflect.StructField, reflect.Value, interface{}) error{tools.SetDefaultValueIfNil})

	uws := &UnifiedWebServer{
		config: config,
		logger: logger.DefaultLogger(),
	}

	// 根据模式创建对应的服务器
	switch config.Mode {
	case ServerModeNetpoll:
		uws.server = uws.createNetpollServer()
	case ServerModeStandard:
		fallthrough
	default:
		uws.server = uws.createStandardServer()
	}

	return uws
}

// createStandardServer 创建标准服务器
func (uws *UnifiedWebServer) createStandardServer() WebServer {
	standardConfig := Config{
		Name:                  uws.config.Name,
		Port:                  uws.config.Port,
		ReadTimeout:           uws.config.ReadTimeout,
		WriteTimeout:          uws.config.WriteTimeout,
		HandleTimeout:         uws.config.HandleTimeout,
		RootPath:              uws.config.RootPath,
		HeaderTraceID:         uws.config.HeaderTraceID,
		ControllerRootPkgName: uws.config.ControllerRootPkgName,
		WebLimiter:            uws.config.WebLimiter,
		EnablePprof:           uws.config.EnablePprof,
	}

	return NewHttpServer(standardConfig)
}

// createNetpollServer 创建 Netpoll 服务器
func (uws *UnifiedWebServer) createNetpollServer() WebServer {
	netpollConfig := NetpollConfig{
		Config: Config{
			Name:                  uws.config.Name,
			Port:                  uws.config.Port,
			ReadTimeout:           uws.config.ReadTimeout,
			WriteTimeout:          uws.config.WriteTimeout,
			HandleTimeout:         uws.config.HandleTimeout,
			RootPath:              uws.config.RootPath,
			HeaderTraceID:         uws.config.HeaderTraceID,
			ControllerRootPkgName: uws.config.ControllerRootPkgName,
			WebLimiter:            uws.config.WebLimiter,
			EnablePprof:           uws.config.EnablePprof,
		},
	}

	return NewNetpollHttpServer(netpollConfig)
}

// Name 获取服务器名称
func (uws *UnifiedWebServer) Name() string {
	return uws.server.Name()
}

// Start 启动服务器
func (uws *UnifiedWebServer) Start() error {
	uws.logger.Info("Starting %s server in %s mode", uws.config.Name, uws.config.Mode)
	return uws.server.Start()
}

// Close 关闭服务器
func (uws *UnifiedWebServer) Close() {
	uws.server.Close()
}

// AddController 添加控制器
func (uws *UnifiedWebServer) AddController(v interface{}) {
	uws.server.AddController(v)
}

// Register 注册 RESTful 控制器
func (uws *UnifiedWebServer) Register(controller *RestfulController) {
	uws.server.Register(controller)
}

// AddInterceptor 添加拦截器
func (uws *UnifiedWebServer) AddInterceptor(i interceptor.Interceptor, order int) {
	uws.server.AddInterceptor(i, order)
}

// SetBeforeDispatchCallBack 设置分发前回调
func (uws *UnifiedWebServer) SetBeforeDispatchCallBack(callbackFunc BeforeDispatchCallbackFunc) {
	uws.server.SetBeforeDispatchCallBack(callbackFunc)
}

// GetMode 获取服务器模式
func (uws *UnifiedWebServer) GetMode() ServerMode {
	return uws.config.Mode
}

// IsNetpollMode 判断是否为 Netpoll 模式
func (uws *UnifiedWebServer) IsNetpollMode() bool {
	return uws.config.Mode == ServerModeNetpoll
}

// GetConfig 获取配置
func (uws *UnifiedWebServer) GetConfig() UnifiedConfig {
	return uws.config
}

// 全局默认服务器实例
var DefaultUnifiedServer *UnifiedWebServer

// InitDefaultUnifiedServer 初始化默认统一服务器
func InitDefaultUnifiedServer(config UnifiedConfig) *UnifiedWebServer {
	if DefaultUnifiedServer != nil {
		return DefaultUnifiedServer
	}
	DefaultUnifiedServer = NewUnifiedWebServer(config)
	return DefaultUnifiedServer
}

// 便捷函数，兼容现有 API
func AddControllerUnified(v interface{}) {
	if DefaultUnifiedServer != nil {
		DefaultUnifiedServer.AddController(v)
	} else if DefaultHttpServer != nil {
		DefaultHttpServer.AddController(v)
	} else {
		logger.Warn("No default server initialized")
	}
}

func RegisterUnified(controller *RestfulController) {
	if DefaultUnifiedServer != nil {
		DefaultUnifiedServer.Register(controller)
	} else if DefaultHttpServer != nil {
		DefaultHttpServer.Register(controller)
	} else {
		logger.Warn("No default server initialized")
	}
}

func AddInterceptorUnified(interceptor interceptor.Interceptor, order int) {
	if DefaultUnifiedServer != nil {
		DefaultUnifiedServer.AddInterceptor(interceptor, order)
	} else if DefaultHttpServer != nil {
		DefaultHttpServer.AddInterceptor(interceptor, order)
	} else {
		logger.Warn("No default server initialized")
	}
}

// ConfigBuilder 配置构建器
type ConfigBuilder struct {
	config UnifiedConfig
}

// NewConfigBuilder 创建配置构建器
func NewConfigBuilder() *ConfigBuilder {
	return &ConfigBuilder{
		config: UnifiedConfig{
			Mode: ServerModeStandard,
		},
	}
}

// WithName 设置服务器名称
func (cb *ConfigBuilder) WithName(name string) *ConfigBuilder {
	cb.config.Name = name
	return cb
}

// WithPort 设置端口
func (cb *ConfigBuilder) WithPort(port uint) *ConfigBuilder {
	cb.config.Port = port
	return cb
}

// WithMode 设置服务器模式
func (cb *ConfigBuilder) WithMode(mode ServerMode) *ConfigBuilder {
	cb.config.Mode = mode
	return cb
}

// WithNetpollMode 启用 Netpoll 模式
func (cb *ConfigBuilder) WithNetpollMode() *ConfigBuilder {
	cb.config.Mode = ServerModeNetpoll
	return cb
}

// WithStandardMode 启用标准模式
func (cb *ConfigBuilder) WithStandardMode() *ConfigBuilder {
	cb.config.Mode = ServerModeStandard
	return cb
}

// WithMaxConnections 设置最大连接数（Netpoll 模式）
func (cb *ConfigBuilder) WithMaxConnections(max int) *ConfigBuilder {
	cb.config.NetpollConfig.MaxConnections = max
	return cb
}

// WithBufferSize 设置缓冲区大小（Netpoll 模式）
func (cb *ConfigBuilder) WithBufferSize(size int) *ConfigBuilder {
	cb.config.NetpollConfig.BufferSize = size
	return cb
}

// WithWorkerPool 设置工作池配置（Netpoll 模式）
func (cb *ConfigBuilder) WithWorkerPool(workers, queueSize int) *ConfigBuilder {
	cb.config.NetpollConfig.WorkerPoolSize = workers
	cb.config.NetpollConfig.TaskQueueSize = queueSize
	return cb
}

// WithMetrics 启用性能监控（Netpoll 模式）
func (cb *ConfigBuilder) WithMetrics(enable bool, interval uint) *ConfigBuilder {
	cb.config.NetpollConfig.EnableMetrics = enable
	cb.config.NetpollConfig.MetricsInterval = interval
	return cb
}

// WithLimiter 设置限流配置
func (cb *ConfigBuilder) WithLimiter(enable bool, qos int) *ConfigBuilder {
	cb.config.WebLimiter.Enable = enable
	cb.config.WebLimiter.Qos = qos
	return cb
}

// WithPprof 启用性能分析
func (cb *ConfigBuilder) WithPprof(enable bool) *ConfigBuilder {
	cb.config.EnablePprof = enable
	return cb
}

// Build 构建配置
func (cb *ConfigBuilder) Build() UnifiedConfig {
	return cb.config
}

// BuildServer 构建服务器
func (cb *ConfigBuilder) BuildServer() *UnifiedWebServer {
	return NewUnifiedWebServer(cb.config)
}
