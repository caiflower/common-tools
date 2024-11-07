package v1

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/caiflower/common-tools/pkg/logger"
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

type HttpServer struct {
	config  *Config
	logger  logger.ILog
	server  *http.Server
	handler *handler
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

// AddController 只能增加func、point和interface，其他类型直接忽略
func (s *HttpServer) AddController(v interface{}) {
	c, err := newController(v, s.config.ControllerRootPkgName)
	if err != nil {
		logger.Warn("[AddController] add error: %s", err.Error())
		return
	}
	s.handler.controllers[c.path] = c
}

func (s *HttpServer) AddInterceptor() {
	return
}

func (s *HttpServer) StartUp() {
	s.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", s.config.Port),
		ReadTimeout:  time.Duration(s.config.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(s.config.WriteTimeout) * time.Second,
		Handler:      commonHandler,
	}

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
