package v1

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"sort"
	"time"

	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/caiflower/common-tools/web/interceptor"
)

// 基于云生http.server实现的http服务器

type Config struct {
	Name                  string `yaml:"name" default:"default"`
	Port                  int    `yaml:"port" default:"8080"`
	ReadTimeout           int    `yaml:"read_timeout" default:"10"`
	WriteTimeout          int    `yaml:"write_timeout" default:"10"`
	RootPath              string `yaml:"root_path"` // 可以为空
	HeaderTraceID         string `yaml:"header_trace_id" default:"X-Request-ID"`
	ControllerRootPkgName string `yaml:"controller_root_pkg_name" default:"controller"`
}

var defaultHttpServer *HttpServer

type HttpServer struct {
	config  *Config
	logger  logger.ILog
	server  *http.Server
	handler *handler
}

func InitDefaultHttpServer(config Config) *HttpServer {
	if defaultHttpServer != nil {
		return defaultHttpServer
	}
	defaultHttpServer = NewHttpServer(config)
	return defaultHttpServer
}

func NewHttpServer(config Config) *HttpServer {
	httpServer := &HttpServer{
		config: &config,
		logger: logger.DefaultLogger(),
	}

	// 初始化公共处理器
	initHandler(&config, httpServer.logger)

	httpServer.handler = commonHandler
	return httpServer
}

func AddController(v interface{}) {
	defaultHttpServer.AddController(v)
}

// AddController 只能增加func、point和interface，其他类型直接忽略
func (s *HttpServer) AddController(v interface{}) {
	c, err := newController(v, s.config.ControllerRootPkgName)
	if err != nil {
		logger.Warn("[AddController] add error: %s", err.Error())
		return
	}

	for _, path := range c.paths {
		s.handler.controllers[path] = c
	}
}

func Register(controller *RestfulController) {
	defaultHttpServer.AddController(controller)
}

// Register 注册restfulController
func (s *HttpServer) Register(controller *RestfulController) {
	path := "method:" + controller.method + "version:" + controller.version + "path:" + controller.path
	if _, e := s.handler.restfulPaths[path]; e {
		panic(fmt.Sprintf("Register restfulApi failed. RestfulPath method[%s] version[%s] path[%s] already exsit. ", controller.method, controller.version, controller.originPath))
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
}

func AddParamValidFunc(fn func(reflect.StructField, reflect.Value, interface{}) error) {
	defaultHttpServer.AddParamValidFunc(fn)
}

func (s *HttpServer) AddParamValidFunc(fn func(reflect.StructField, reflect.Value, interface{}) error) {
	s.handler.paramsValidFuncList = append(s.handler.paramsValidFuncList, fn)
}

func AddInterceptor(interceptor interceptor.Interceptor, order int) {
	defaultHttpServer.AddInterceptor(interceptor, order)
}

func (s *HttpServer) AddInterceptor(i interceptor.Interceptor, order int) {
	s.handler.interceptors = append(s.handler.interceptors, interceptor.Item{
		Interceptor: i,
		Order:       order,
	})
}

func (s *HttpServer) StartUp() {
	s.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", s.config.Port),
		ReadTimeout:  time.Duration(s.config.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(s.config.WriteTimeout) * time.Second,
		Handler:      commonHandler,
	}

	sort.Sort(s.handler.interceptors)

	s.logger.Info(
		"\n***************************** http server startup ***********************************************\n"+
			"************* web service [name:%s] [rootPath:%s] listening on %d *********\n"+
			"*************************************************************************************************", s.config.Name, s.config.RootPath, s.config.Port)
	go func() {
		if err := s.server.ListenAndServe(); err != nil {
			if err.Error() != "http: Server closed" {
				panic(err)
			}
		}
	}()
}

func (s *HttpServer) Close() {
	s.logger.Info("      **** http server shutdown ****")
	if s.server != nil {
		// 30秒超时
		ctx, _ := context.WithTimeout(context.Background(), time.Second*30)
		if err := s.server.Shutdown(ctx); err != nil {
			s.logger.Warn(" **** http server shutdown error **** \n"+
				"**** error:%s ****", err.Error())
		}
		s.logger.Info(" **** http server gracefully shutdown ****")
	}
}
