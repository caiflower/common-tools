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

package router

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/pprof"
	"reflect"
	"runtime/debug"
	runtimepprof "runtime/pprof"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/caiflower/common-tools/web/common/adaptor"
	"github.com/caiflower/common-tools/web/common/e"
	"github.com/caiflower/common-tools/web/common/goai"
	"github.com/caiflower/common-tools/web/protocol/consts"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/caiflower/common-tools/pkg/bean"
	golocalv1 "github.com/caiflower/common-tools/pkg/golocal/v1"
	"github.com/caiflower/common-tools/pkg/limiter"
	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/caiflower/common-tools/pkg/tools"
	"github.com/caiflower/common-tools/pkg/tools/bytesconv"
	"github.com/caiflower/common-tools/web/app"
	"github.com/caiflower/common-tools/web/common/compress"
	"github.com/caiflower/common-tools/web/common/interceptor"
	"github.com/caiflower/common-tools/web/common/metric"
	"github.com/caiflower/common-tools/web/common/resp"
	"github.com/caiflower/common-tools/web/router/controller"
	"github.com/caiflower/common-tools/web/router/method"
	"github.com/caiflower/common-tools/web/router/param"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

const (
	dispatchBeginTime = "web/handler/dispatch_begin_time"
)

const (
	statusInitialized uint32 = iota
	statusRunning
	statusClosed
)

var (
	assignableApiErrorElem = reflect.TypeOf(new(e.ApiError)).Elem()
	assignableErrorElem    = reflect.TypeOf(new(error)).Elem()
)

// CallbackFunc 在进行分发前进行回调的函数, 返回true结束
type CallbackFunc func(ctx *app.RequestContext) bool

type HandlerCfg struct {
	Name                          string        `yaml:"name" default:"default"`
	RootPath                      string        `yaml:"rootPath"` // 可以为空
	HeaderTraceID                 string        `yaml:"headerTraceID" default:"X-Request-Id"`
	ControllerRootPkgName         string        `yaml:"controllerRootPkgName" default:"controller"`
	EnablePprof                   bool          `yaml:"enablePprof"`
	WebLimiter                    LimiterConfig `yaml:"webLimiter"`
	EnableMetrics                 bool          `yaml:"enableMetrics"`
	DisableOptimization           bool          `yaml:"disableOptimization"`
	EnableActionController        bool          `yaml:"enableActionController"`
	EnableSwagger                 bool          `yaml:"enableSwagger"`
	DisableHeaderNamesNormalizing bool          `yaml:"disableHeaderNamesNormalizing"`
}

type LimiterConfig struct {
	Enable bool `yaml:"enable"`
	Qos    int  `yaml:"qos" default:"1000"`
}

func NewHandler(config HandlerCfg, logger logger.ILog) *Handler {
	commonHandler := &Handler{
		config:                    &config,
		controllers:               make(map[string]*controller.Controller),
		restfulPaths:              make(map[string]struct{}),
		logger:                    logger,
		oai:                       goai.Default(),
		afterDispatchCallbackFunc: resp.DefaultResultCallback,
	}
	setGoAIInstance(commonHandler.oai)

	commonHandler.ctxPool.New = func() interface{} {
		ctx := &app.RequestCtx{
			Paths: make(param.Params, 0, 10),
		}
		ctx.SetNetWorkType("standard")
		ctx.SetDisableHeaderNamesNormalizing(config.DisableHeaderNamesNormalizing)
		return ctx
	}

	if config.WebLimiter.Enable {
		getLimiterCallBack := func(qos int) limiter.Limiter {
			limiterBucket := limiter.NewXTokenBucket(qos, qos)
			return limiterBucket
		}

		limiterBucket := getLimiterCallBack(config.WebLimiter.Qos)
		commonHandler.qosCallback = func(ctx *app.RequestContext) bool {
			if limiterBucket.TakeTokenNonBlocking() {
				return false
			}

			ctx.SetError(e.NewApiError(e.TooManyRequests, "Too Many Requests", nil))
			return true
		}
	}

	return commonHandler
}

type Handler struct {
	config *HandlerCfg

	controllers  map[string]*controller.Controller
	trees        MethodTrees
	restfulPaths map[string]struct{}

	logger logger.ILog
	metric *metric.HttpMetric

	// qos call back
	qosCallback                CallbackFunc
	beforeDispatchCallbackFunc CallbackFunc
	interceptors               interceptor.ItemSort
	afterDispatchCallbackFunc  CallbackFunc

	// RequestContext pool
	ctxPool sync.Pool
	status  uint32
	// goai
	oai *goai.OpenApiV3
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer golocalv1.Clean()

	ctx := h.getRequestContext()
	ctx = initCtx(ctx, w, r)
	ctx.SetContext(context.TODO())
	defer h.putRequestContext(ctx)

	if h.specialRequest(ctx) {
		return
	}

	if h.serverCommon(ctx) {
		return
	}

	// dispatch
	h.Dispatch(ctx)

	if h.afterDispatchCallbackFunc != nil {
		h.afterDispatchCallbackFunc(ctx)
	} else {
		if err := ctx.GetError(); err != nil {
			h.writeError(ctx, err)
		} else {
			h.writeResponse(ctx)
		}
	}

	// record metric
	h.recordMetric(ctx)
}

func (h *Handler) Serve(ctx *app.RequestCtx) {
	defer golocalv1.Clean()

	ctx.SetMethod(ctx.Request.Method())
	ctx.SetPath(ctx.Request.Path())

	if h.specialRequest(ctx) {
		return
	}

	if h.serverCommon(ctx) {
		return
	}

	// dispatch
	h.Dispatch(ctx)

	h.afterDispatchCallbackFunc(ctx)

	// record metric
	h.recordMetric(ctx)
}

func (h *Handler) serverCommon(ctx *app.RequestCtx) bool {
	var traceID string
	traceID = ctx.HeaderGet(h.config.HeaderTraceID)
	if traceID == "" {
		traceID = tools.UUID()
	}

	golocalv1.PutTraceID(traceID)
	if h.config.EnableMetrics {
		golocalv1.Put(dispatchBeginTime, time.Now())
	}
	golocalv1.PutContext(ctx.GetContext())

	if h.qosCallback != nil {
		return h.qosCallback(ctx)
	}

	if h.beforeDispatchCallbackFunc != nil {
		return h.beforeDispatchCallbackFunc(ctx)
	}

	return false
}

func (h *Handler) SetBeforeDispatchCallBack(callbackFunc CallbackFunc) {
	h.beforeDispatchCallbackFunc = callbackFunc
}

func (h *Handler) SetAfterDispatchCallBack(callbackFunc CallbackFunc) {
	h.afterDispatchCallbackFunc = callbackFunc
}

func (h *Handler) AddInterceptor(i interceptor.Interceptor, order int) {
	h.interceptors = append(h.interceptors, interceptor.Item{
		Interceptor: i,
		Order:       order,
	})
}

func (h *Handler) SortInterceptors() {
	sort.Sort(h.interceptors)
}

func (h *Handler) AddController(v interface{}) *controller.Controller {
	c, err := controller.NewController(v, h.config.ControllerRootPkgName, h.config.RootPath)
	if err != nil {
		logger.Warn("[AddController] add error: %s", err.Error())
		panic(fmt.Sprintf("AddController failed. Error: %v", err))
	}

	if h.config.EnableActionController {
		paths := c.GetPaths()
		for _, path := range paths {
			logger.Info("Register action path %s?Action=MethodName", path)
			h.controllers[path] = c
		}
	}

	if !bean.HasBean(bean.GetBeanNameFromValue(v)) {
		bean.AddBean(v)
	}

	// register method argument schemas to goai
	for _, m := range c.GetAllMethod() {
		if m.HasArgs() {
			arg := m.GetArgs()[0]
			switch arg.Kind() {
			case reflect.Ptr:
				_ = h.oai.Add(goai.AddInput{Object: reflect.New(arg.Elem()).Elem().Interface()})
			case reflect.Struct:
				_ = h.oai.Add(goai.AddInput{Object: reflect.New(arg).Elem().Interface()})
			default:
			}
		}
	}

	return c
}

func (h *Handler) Register(ctl *controller.RestfulController) {
	var (
		m                           = ctl.GetMethod()
		originPath                  = ctl.GetOriginPath()
		isGrpc, grpcMethodDesc, srv = ctl.GetGrpcMethodDesc()
		methodDesc                  *method.Method
	)

	if m == "" {
		panic("Register restfulApi failed. Method cannot be empty.")
	}

	rawPath := fmt.Sprintf("%s%s", ctl.GetGroup(), originPath)
	path := normalizeRestfulPath(rawPath)
	if path == "" {
		panic("Register restfulApi failed. Path cannot be empty.")
	}

	if _, ok := h.restfulPaths[path]; ok {
		panic(fmt.Sprintf("Register restfulApi failed. RestfulPath method[%s] path[%s] already exist. ", m, originPath))
	}
	h.restfulPaths[path] = struct{}{}

	targetMethod := ctl.GetTargetMethod()
	if targetMethod == nil {
		panic(fmt.Sprintf("Register restfulApi failed. method[%s] path[%s] not found targetMethod. ", m, originPath))
	}

	if !isGrpc {
		methodDesc = method.NewDefaultTypeMethod(targetMethod)
	} else {
		methodDesc = method.NewGrpcTypeMethod(grpcMethodDesc, srv, targetMethod)
	}

	methodRouter := h.trees.get(m)
	if methodRouter == nil {
		methodRouter = &router{method: m, root: &node{}}
		h.trees = append(h.trees, methodRouter)
	}

	methodRouter.addRoute(path, []method.Method{*methodDesc})

	swaggerPath := toSwaggerPath(rawPath)
	_ = h.oai.Add(goai.AddInput{
		Path:        swaggerPath,
		Method:      m,
		Object:      targetMethod.GetFunc(),
		OperationID: strings.Replace(targetMethod.GetName(), targetMethod.GetPkgName()+".", "", -1),
	})

	logger.Info("Register path %v, Method: %v", path, m)
}

func (h *Handler) getRequestContext() *app.RequestCtx {
	return h.ctxPool.Get().(*app.RequestCtx)
}

func initCtx(ctx *app.RequestCtx, w http.ResponseWriter, r *http.Request) *app.RequestCtx {
	ctx.SetMethod(bytesconv.S2b(r.Method))
	ctx.SetPath(bytesconv.S2b(r.URL.Path))
	ctx.SetHttpWriterAndRequest(w, r)
	return ctx
}

func (h *Handler) putRequestContext(ctx *app.RequestCtx) {
	ctx.Reset()
	h.ctxPool.Put(ctx)
}

func (h *Handler) Dispatch(ctx *app.RequestCtx) {
	defer h.onCrash("dispatch", ctx, e.NewApiError(e.Internal, "InternalError", nil))

	var (
		m          *method.Method
		find       bool
		inputValue []reflect.Value
		inputArg   interface{}
	)

	// method
	if m, find = h.getTargetMethod(ctx); !find {
		ctx.SetError(e.NewApiError(e.NotFound, "no such api.", nil))
		return
	}

	webContext := ctx.ConvertToWebCtx()
	if m.GetType() == method.DefaultTypeOfMethod && m.HasArgs() {
		var (
			targetM = m.GetTargetMethod()
			argLen  = len(targetM.GetArgs())
			arg     reflect.Type
		)

		onlyCtx := false

		inputValue = make([]reflect.Value, argLen)
		if argLen == 2 {
			inputValue[0] = reflect.ValueOf(ctx)
			arg = targetM.GetArgs()[1]
		} else {
			arg = targetM.GetArgs()[0]
			if arg.ConvertibleTo(reflect.TypeOf(ctx)) {
				inputValue[0] = reflect.ValueOf(ctx)
				onlyCtx = true
			}
		}

		if !onlyCtx {
			switch arg.Kind() {
			case reflect.Ptr:
				v := reflect.New(arg.Elem())
				inputValue[len(inputValue)-1] = v
				inputArg = v.Interface()
			case reflect.Struct:
				v := reflect.New(arg)
				inputArg = v.Interface()
				inputValue[len(inputValue)-1] = v.Elem()
			default:
				ctx.SetError(e.NewInternalError(fmt.Errorf("parse param failed. not support kind %s", arg.Kind())))
				return
			}

			// set args
			if err := setArgsOptimized(ctx, inputArg, targetM.GetArgInfo(0)); err != nil {
				if err.IsInternalError() {
					h.logger.Warn("setArgsOptimized failed. Error: %v", err)
				}
				ctx.SetError(err)
				return
			}

			// valid args
			if err := validArgs(inputArg); err != nil {
				ctx.SetError(err)
				return
			}
		}
	}

	defer h.onDoTargetMethodCrash("doTargetMethod", ctx, webContext, e.NewApiError(e.Internal, "InternalError", nil))

	// doTargetMethod
	targetMethod := func() e.ApiError {
		return h.doTargetMethod(ctx, m, inputValue)
	}

	// aop
	if err := h.interceptors.DoInterceptor(webContext, targetMethod); err != nil {
		ctx.SetError(err)
		return
	}
}

func (h *Handler) getTargetMethod(ctx *app.RequestCtx) (*method.Method, bool) {
	ctx.ComputeAction()

	var m *method.Method

	path := ctx.GetPath()
	if !ctx.IsRestful() && h.config.EnableActionController {
		// action 风格
		c := h.controllers[path]
		if c != nil {
			m = c.GetMethodDesc(ctx.GetAction())
		}
	} else {
		// restful
		tree := h.trees.get(ctx.GetMethod())
		if tree != nil {
			res := tree.find(path, &ctx.Paths, false)
			if res.handlers != nil {
				m = &res.handlers[0]
				ctx.SetAction(m.GetAction())
			}
		}
	}

	return m, m != nil
}

func (h *Handler) doTargetMethod(ctx *app.RequestCtx, targetMethodDesc *method.Method, inputValues []reflect.Value) e.ApiError {
	t, targetMethod, grpcMethodDesc, grpcSrv := targetMethodDesc.GetInfo()

	switch t {
	case method.GrpcTypeOfMethod:
		bindAndValid := func(arg interface{}) (err error) {
			err = setArgsOptimized(ctx, arg, targetMethod.GetArgInfo(1))
			if err != nil {
				return err
			}

			err = validArgs(arg)
			return
		}

		data, err := grpcMethodDesc.Handler(grpcSrv, ctx, bindAndValid, nil)
		if err != nil {
			var apiError e.ApiError
			switch {
			case errors.As(err, &apiError):
				return err.(e.ApiError)
			default:
				st, ok := status.FromError(err)
				if ok {
					apiErr := e.ConvertGrpcCodeToErrorCode(st)
					if apiErr != nil {
						return apiErr
					}
				} else {
					return e.NewInternalError(err)
				}
			}
		}
		ctx.SetData(data)
	default:
		results := targetMethod.Invoke(inputValues)
		rets := targetMethod.GetRets()
		for i, ret := range rets {
			if ret.AssignableTo(assignableApiErrorElem) {
				_err := results[i].Interface()
				if _err != nil {
					return _err.(e.ApiError)
				}
			} else if ret.AssignableTo(assignableErrorElem) {
				_err := results[i].Interface()
				if _err != nil {
					_err1 := _err.(error)
					return e.NewApiError(e.Unknown, _err1.Error(), _err1)
				}
			} else {
				ctx.SetData(results[i].Interface())
			}
		}
	}

	return nil
}

func (h *Handler) writeError(ctx *app.RequestCtx, err e.ApiError) {
	if ctx.IsAbort() || err == nil {
		return
	}

	if err.IsInternalError() {
		h.logger.Error("handle request failed. Error: %s", err.Error())
	}

	ctx.SetHeader(consts.HeaderContentType, consts.MIMEApplicationJSONUTF8)
	ctx.SetHeader(consts.HeaderAcceptEncoding, "gzip, br")

	res := resp.Result{
		RequestID: golocalv1.GetTraceID(),
		Error:     &e.Error{Code: err.GetCode(), Message: err.GetMessage(), Type: err.GetType(), Cause: err.GetCause()},
	}

	bytes, _ := tools.Marshal(res)
	str := ctx.GetAcceptEncoding()
	if ctx.IsRestful() {
		ctx.SetStatusCode(err.GetCode())
	} else {
		if strings.Contains(str, "gzip") {
			bytes = compress.AppendGzipBytesLevel(nil, bytes, 5)
			ctx.SetHeader(consts.HeaderContentEncoding, "gzip")
		} else if strings.Contains(str, "br") {
			tmpBytes, err := tools.Brotil(bytes)
			if err == nil {
				bytes = tmpBytes
				ctx.SetHeader(consts.HeaderContentEncoding, "br")
			}
		}
	}

	if _, err := ctx.Write(bytes); err != nil {
		h.logger.Error("writeResponse Error: %s", err.Error())
	}
}

func (h *Handler) writeResponse(ctx *app.RequestCtx) {
	if ctx.IsAbort() {
		return
	}

	ctx.SetHeader(consts.HeaderContentType, consts.MIMEApplicationJSONUTF8)
	ctx.SetHeader(consts.HeaderAcceptEncoding, "gzip, br")

	res := resp.Result{
		RequestID: golocalv1.GetTraceID(),
		Data:      ctx.GetData(),
	}

	bytes, _ := tools.Marshal(res)
	str := ctx.GetAcceptEncoding()
	if strings.Contains(str, "gzip") {
		bytes = compress.AppendGzipBytesLevel(nil, bytes, 5)
		ctx.SetHeader(consts.HeaderContentEncoding, "gzip")
	} else if strings.Contains(str, "br") {
		tmpBytes, err := tools.Brotil(bytes)
		if err == nil {
			bytes = tmpBytes
			ctx.SetHeader(consts.HeaderContentEncoding, "br")
		}
	}

	if _, err := ctx.Write(bytes); err != nil {
		h.logger.Error("writeResponse Error: %s", err.Error())
	}
}

func (h *Handler) onCrash(txt string, ctx *app.RequestCtx, e e.ApiError) {
	if err := recover(); err != nil {
		h.logger.Fatal("Got a runtime error %s, %v. \n%s", txt, err, string(debug.Stack()))
		ctx.SetError(e)
	}
}

func (h *Handler) onDoTargetMethodCrash(txt string, ctx *app.RequestCtx, interceptorCtx *app.Context, defaultErr e.ApiError) {
	if err := recover(); err != nil {
		h.logger.Fatal("Got a runtime error %s, %v. \n%s", txt, err, string(debug.Stack()))

		// onPanic
		for _, v := range h.interceptors {
			apiError := v.Interceptor.OnPanic(interceptorCtx, err)
			if apiError != nil {
				defaultErr = apiError
				break
			}
		}

		ctx.SetError(defaultErr)
	}
}

var promHttpHandler = promhttp.Handler()

func (h *Handler) specialRequest(ctx *app.RequestCtx) bool {
	path := ctx.GetPath()

	if ctx.IsNetpoll() && h.config.EnableMetrics && path == "/metrics" {
		handler := adaptor.HertzHandler(promhttp.HandlerFor(prometheus.DefaultGatherer, promhttp.HandlerOpts{ErrorHandling: promhttp.ContinueOnError}))
		handler(ctx.GetContext(), ctx)
		return true
	}

	if h.config.EnableMetrics && path == "/metrics" {
		w, r := ctx.GetResponseWriterAndRequest()
		promHttpHandler.ServeHTTP(w, r)
		return true
	} else if h.config.EnableSwagger && path == "/swagger/json" {
		ctx.Write([]byte(h.oai.String()))
		return true
	} else if h.config.EnablePprof {
		handleName := strings.Replace(path, "/debug/pprof/", "", 1)
		switch handleName {
		case "":
			handler := adaptor.HertzHandler(http.HandlerFunc(pprof.Index))
			handler(ctx.GetContext(), ctx)
			return true
		case "profile":
			handler := adaptor.HertzHandler(http.HandlerFunc(pprof.Profile))
			handler(ctx.GetContext(), ctx)
			return true
		case "cmdline":
			handler := adaptor.HertzHandler(http.HandlerFunc(pprof.Cmdline))
			handler(ctx.GetContext(), ctx)
			return true
		case "trace":
			handler := adaptor.HertzHandler(http.HandlerFunc(pprof.Trace))
			handler(ctx.GetContext(), ctx)
			return true
		case "symbol":
			handler := adaptor.HertzHandler(http.HandlerFunc(pprof.Symbol))
			handler(ctx.GetContext(), ctx)
			return true
		}

		if runtimepprof.Lookup(handleName) != nil {
			handler := adaptor.HertzHandler(pprof.Handler(handleName))
			handler(ctx.GetContext(), ctx)
			return true
		}
	}

	return false
}

func (h *Handler) IsRunning() bool {
	if atomic.LoadUint32(&h.status) != statusRunning {
		return false
	}

	return true
}

// SetRunning 设置运行状态
func (h *Handler) SetRunning(val bool) bool {
	if val {
		return atomic.CompareAndSwapUint32(&h.status, statusInitialized, statusRunning)
	}

	return atomic.CompareAndSwapUint32(&h.status, statusRunning, statusClosed)
}

func (h *Handler) GetCtxPool() *sync.Pool {
	return &h.ctxPool
}

func (h *Handler) RegisterGRPCService(serviceDesc *grpc.ServiceDesc, srv interface{}) *controller.Controller {
	ctl := h.AddController(srv)
	ctl.SetGrpcService(serviceDesc, srv)
	return ctl
}

func (h *Handler) recordMetric(ctx *app.RequestContext) {
	if h.config.EnableMetrics {
		sub := time.Now().Sub(golocalv1.Get(dispatchBeginTime).(time.Time))
		// fix: 关闭协程提升性能
		metric.SaveMetric(h.config.Name, strconv.Itoa(ctx.GetStatusCode()), ctx.GetMethod(), ctx.GetPath(), 0, sub.Seconds())
	}
}
