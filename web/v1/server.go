package webv1

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"sort"
	"time"

	"github.com/caiflower/common-tools/global"
	"github.com/caiflower/common-tools/pkg/bean"
	"github.com/caiflower/common-tools/pkg/limiter"
	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/caiflower/common-tools/pkg/tools"
	"github.com/caiflower/common-tools/web/e"
	"github.com/caiflower/common-tools/web/interceptor"
)

// 基于云生http.server实现的http服务器

type Config struct {
	Name                  string     `yaml:"name" default:"default"`
	Port                  int        `yaml:"port" default:"8080"`
	ReadTimeout           int        `yaml:"readTimeout" default:"20"`
	WriteTimeout          int        `yaml:"writeTimeout" default:"35"`
	RootPath              string     `yaml:"rootPath"` // 可以为空
	HeaderTraceID         string     `yaml:"headerTraceID" default:"X-Request-Id"`
	ControllerRootPkgName string     `yaml:"controllerRootPkgName" default:"controller"`
	WebLimiter            WebLimiter `yaml:"webLimiter"`
	EnablePprof           bool       `yaml:"enablePprof"`
}

type WebLimiter struct {
	Enable bool `yaml:"enable"`
	Qos    int  `yaml:"qos" default:"1000"`
}

var DefaultHttpServer *HttpServer

type HttpServer struct {
	config        *Config
	logger        logger.ILog
	server        *http.Server
	handler       *handler
	limiterBucket limiter.Limiter
}

func InitDefaultHttpServer(config Config) *HttpServer {
	if DefaultHttpServer != nil {
		return DefaultHttpServer
	}
	DefaultHttpServer = NewHttpServer(config)
	return DefaultHttpServer
}

func NewHttpServer(config Config) *HttpServer {
	_ = tools.DoTagFunc(&config, nil, []func(reflect.StructField, reflect.Value, interface{}) error{tools.SetDefaultValueIfNil})

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
	DefaultHttpServer.AddController(v)
}

// AddController 只能增加func、point和interface，其他类型直接忽略
func (s *HttpServer) AddController(v interface{}) {
	c, err := newController(v, s.config.ControllerRootPkgName, s.config.RootPath)
	if err != nil {
		logger.Warn("[AddController] add error: %s", err.Error())
		return
	}

	for _, path := range c.paths {
		logger.Info("Register action path %s?Action=xxx", path)
		s.handler.controllers[path] = c
	}

	bean.AddBean(v)
}

func Register(controller *RestfulController) {
	DefaultHttpServer.Register(controller)
}

// Register 注册restfulController
func (s *HttpServer) Register(controller *RestfulController) {
	path := "method:" + controller.method + "version:" + controller.version + "path:" + controller.path
	if _, e := s.handler.restfulPaths[path]; e {
		panic(fmt.Sprintf("Register restfulApi failed. RestfulPath method[%s] version[%s] path[%s] already exsit. ", controller.method, controller.version, controller.path))
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

//func AddParamValidFunc(fn ValidFunc) {
//	DefaultHttpServer.AddParamValidFunc(fn)
//}
//
//func (s *HttpServer) AddParamValidFunc(fn ValidFunc) {
//	s.handler.paramsValidFuncList = append(s.handler.paramsValidFuncList, fn)
//}

func AddInterceptor(interceptor interceptor.Interceptor, order int) {
	DefaultHttpServer.AddInterceptor(interceptor, order)
}

func (s *HttpServer) SetBeforeDispatchCallBack(callbackFunc BeforeDispatchCallbackFunc) {
	s.handler.beforeDispatchCallbackFunc = callbackFunc
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

	if s.config.WebLimiter.Enable {
		getLimiterCallBack := func(qos int) limiter.Limiter {
			limiterBucket := limiter.NewXTokenBucket(qos, qos)
			return limiterBucket
		}

		s.limiterBucket = getLimiterCallBack(s.config.WebLimiter.Qos)

		var limiterCallBack = func(w http.ResponseWriter, r *http.Request) bool {
			if s.limiterBucket.TakeTokenNonBlocking() {
				return false
			} else {
				res := CommonResponse{
					RequestId: tools.UUID(),
					Error:     e.NewApiError(e.TooManyRequests, "TooManyRequests", nil),
				}

				w.Header().Set("Content-Type", "application/json; charset=UTF-8")
				w.WriteHeader(res.Error.GetCode())
				w.Write([]byte(tools.ToJson(res)))
				return true
			}
		}
		s.SetBeforeDispatchCallBack(limiterCallBack)
	}

	sort.Sort(s.handler.interceptors)
	metric := NewHttpMetric()
	s.handler.metric = metric

	global.DefaultResourceManger.Add(s)

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
	s.server = nil
	s.limiterBucket = nil
}
