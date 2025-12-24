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
	"time"

	"github.com/caiflower/common-tools/pkg/basic"
	"github.com/caiflower/common-tools/pkg/bean"
	golocalv1 "github.com/caiflower/common-tools/pkg/golocal/v1"
	"github.com/caiflower/common-tools/pkg/limiter"
	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/caiflower/common-tools/pkg/tools"
	"github.com/caiflower/common-tools/pkg/tools/bytesconv"
	"github.com/caiflower/common-tools/web/common/compress"
	"github.com/caiflower/common-tools/web/common/e"
	"github.com/caiflower/common-tools/web/common/interceptor"
	"github.com/caiflower/common-tools/web/common/metric"
	"github.com/caiflower/common-tools/web/common/resp"
	"github.com/caiflower/common-tools/web/common/webctx"
	"github.com/caiflower/common-tools/web/router/controller"
	"github.com/caiflower/common-tools/web/router/param"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	BeginTime = "web/handler/req_beginTime"
)

var (
	assignableApiErrorElem   = reflect.TypeOf(new(e.ApiError)).Elem()
	assignableErrorElem      = reflect.TypeOf(new(error)).Elem()
	assignableWebContextElem = reflect.TypeOf(new(webctx.Context)).Elem()
	assignableWebContext     = reflect.TypeOf(new(webctx.Context))
)

type HandlerCfg struct {
	Name                  string        `yaml:"name" default:"default"`
	RootPath              string        `yaml:"rootPath"` // 可以为空
	HeaderTraceID         string        `yaml:"headerTraceID" default:"X-Request-Id"`
	ControllerRootPkgName string        `yaml:"controllerRootPkgName" default:"controller"`
	EnablePprof           bool          `yaml:"enablePprof"`
	WebLimiter            LimiterConfig `yaml:"webLimiter"`
	EnableMetrics         bool          `yaml:"enableMetrics"`
	DisableOptimization   bool          `yaml:"disableOptimization"`
}

type LimiterConfig struct {
	Enable bool `yaml:"enable"`
	Qos    int  `yaml:"qos" default:"1000"`
}

// BeforeDispatchCallbackFunc 在进行分发前进行回调的函数, 返回true结束
type BeforeDispatchCallbackFunc func(w http.ResponseWriter, r *http.Request) bool

var metrics = metric.NewHttpMetric()

func NewHandler(config HandlerCfg, logger logger.ILog) *Handler {
	commonHandler := &Handler{
		config:       &config,
		controllers:  make(map[string]*controller.Controller),
		restfulPaths: make(map[string]struct{}),
		logger:       logger,
		metric:       metrics,
	}

	commonHandler.ctxPool.New = func() interface{} {
		return &webctx.RequestCtx{
			Paths: make(param.Params, 0, 10),
		}
	}

	if config.WebLimiter.Enable {
		getLimiterCallBack := func(qos int) limiter.Limiter {
			limiterBucket := limiter.NewXTokenBucket(qos, qos)
			return limiterBucket
		}

		limiterBucket := getLimiterCallBack(config.WebLimiter.Qos)
		commonHandler.beforeDispatchCallbackFunc = func(w http.ResponseWriter, r *http.Request) bool {
			if limiterBucket.TakeTokenNonBlocking() {
				return false
			}

			res := resp.Result{
				RequestId: tools.UUID(),
				Error:     e.NewApiError(e.TooManyRequests, "TooManyRequests", nil),
			}

			w.Header().Set("Content-Type", "application/json; charset=UTF-8")
			w.WriteHeader(res.Error.GetCode())
			_, _ = w.Write([]byte(tools.ToJson(res)))
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

	beforeDispatchCallbackFunc BeforeDispatchCallbackFunc
	interceptors               interceptor.ItemSort

	// RequestContext pool
	ctxPool sync.Pool
	running bool
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer golocalv1.Clean()

	ctx := h.getRequestContext()
	ctx = InitCtx(ctx, w, r)
	defer h.putRequestContext(ctx)

	h.serverCommon(ctx)

	if h.specialRequest(w, r) {
		return
	}

	// dispatch
	h.Dispatch(ctx)
}

func (h *Handler) Serve(ctx *webctx.RequestCtx) {
	defer golocalv1.Clean()

	ctx.SetMethod(ctx.Request.Method())
	ctx.SetPath(ctx.Request.Path())
	h.serverCommon(ctx)

	// dispatch
	h.Dispatch(ctx)
}

func (h *Handler) serverCommon(ctx *webctx.RequestCtx) {
	var traceID string
	traceID = ctx.HeaderGet(h.config.HeaderTraceID)
	if traceID == "" {
		traceID = tools.UUID()
	}

	golocalv1.PutTraceID(traceID)
	if h.config.EnableMetrics {
		golocalv1.Put(BeginTime, time.Now())
	}
	golocalv1.PutContext(ctx.GetContext())

	// TODO writer request is nil
	if h.beforeDispatchCallbackFunc != nil {
		if h.beforeDispatchCallbackFunc(ctx.Writer, ctx.HttpRequest) {
			return
		}
	}

	return
}

func (h *Handler) SetBeforeDispatchCallBack(callbackFunc BeforeDispatchCallbackFunc) {
	h.beforeDispatchCallbackFunc = callbackFunc
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
		return nil
	}

	paths := c.GetPaths()
	for _, path := range paths {
		logger.Info("Register action path %s?Action=MethodName", path)
		h.controllers[path] = c
	}

	if !bean.HasBean(bean.GetBeanNameFromValue(v)) {
		bean.AddBean(v)
	}

	return c
}

func (h *Handler) Register(controller *controller.RestfulController) {
	method := controller.GetMethod()
	version := controller.GetVersion()
	action := controller.GetAction()
	controllerName := controller.GetControllerName()
	originPath := controller.GetOriginPath()

	path := fmt.Sprintf("/%s%s%s", version, controller.GetGroup(), originPath)
	if _, ok := h.restfulPaths[path]; ok {
		panic(fmt.Sprintf("Register restfulApi failed. RestfulPath method[%s] version[%s] path[%s] already exist. ", method, version, originPath))
	}

	targetMethod := controller.GetTargetMethod()
	if targetMethod == nil && controllerName != "" {
		c := h.controllers[controllerName]
		if c != nil {
			cls := c.GetCls()
			targetMethod = cls.GetMethod(cls.GetPkgName() + "." + action)
		}
	}

	if targetMethod == nil {
		panic(fmt.Sprintf("Register restfulApi failed. path[%s] Not found controller[%s] action[%s]. ", path, controllerName, action))
	}

	methodRouter := h.trees.get(method)
	if methodRouter == nil {
		methodRouter = &router{method: method, root: &node{}}
		h.trees = append(h.trees, methodRouter)
	}

	methodRouter.addRoute(path, []*basic.Method{targetMethod})

	logger.Info("Register path %v, Method: %v", path, method)
}

func (h *Handler) getRequestContext() *webctx.RequestCtx {
	ctx := h.ctxPool.Get().(*webctx.RequestCtx)

	return ctx
}

func InitCtx(ctx *webctx.RequestCtx, w http.ResponseWriter, r *http.Request) *webctx.RequestCtx {
	ctx.HttpRequest = r
	ctx.SetMethod(bytesconv.S2b(r.Method))
	ctx.SetPath(bytesconv.S2b(r.URL.Path))
	ctx.Writer = w
	return ctx
}

func (h *Handler) putRequestContext(ctx *webctx.RequestCtx) {
	ctx.Reset()
	h.ctxPool.Put(ctx)
}

func (h *Handler) Dispatch(ctx *webctx.RequestCtx) {
	defer h.onCrash("dispatch", ctx, e.NewApiError(e.Internal, "InternalError", nil))

	// method
	if !h.setTargetMethod(ctx) {
		h.writeError(ctx, e.NewApiError(e.NotFound, "no such api.", nil))
		return
	}

	// set args
	webContext := ctx.ConvertToWebCtx()
	if !h.config.DisableOptimization {
		if err := setArgsOptimized(ctx, webContext); err != nil {
			if err.IsInternalError() {
				h.logger.Warn("setArgsOptimized failed. Error: %v", err)
			}
			h.writeError(ctx, err)
			return
		}
	} else {
		if err := setArgs(ctx, webContext); err != nil {
			if err.IsInternalError() {
				h.logger.Warn("setArgs failed. Error: %v", err)
			}
			h.writeError(ctx, err)
			return
		}
	}

	// valid args
	if err := validArgs(ctx); err != nil {
		h.writeError(ctx, err)
		return
	}

	defer h.onDoTargetMethodCrash("doTargetMethod", ctx, webContext, e.NewApiError(e.Internal, "InternalError", nil))

	// doTargetMethod
	targetMethod := func() e.ApiError {
		return doTargetMethod(ctx)
	}

	// aop
	if err := h.interceptors.DoInterceptor(webContext, targetMethod); err != nil {
		h.writeError(ctx, err)
		return
	}

	// set response
	h.writeResponse(ctx)
}

func (h *Handler) setTargetMethod(ctx *webctx.RequestCtx) bool {
	ctx.Action = ctx.SetAction()
	if ctx.Action != "" {
		ctx.Restful = false
	} else {
		ctx.Restful = true
	}

	path := ctx.GetPath()
	if !ctx.IsRestful() {
		// action风格
		c := h.controllers[path]
		if c != nil {
			cls := c.GetCls()
			ctx.TargetMethod = cls.GetMethod(cls.GetPkgName() + "." + ctx.Action)
		}
	} else {
		// restful
		tree := h.trees.get(ctx.GetMethod())
		if tree != nil {
			res := tree.find(path, &ctx.Paths, false)
			if res.handlers != nil {
				ctx.TargetMethod = res.handlers[0]
				ctx.Action = ctx.TargetMethod.GetName()
			}
		}
	}

	return ctx.TargetMethod != nil
}

func doTargetMethod(ctx *webctx.RequestCtx) (err e.ApiError) {
	if ctx.TargetMethod.HasArgs() {
		if ctx.TargetMethod.GetArgs()[0].Kind() == reflect.Struct {
			ctx.Args[0] = ctx.Args[0].Elem()
		}
	}

	results := ctx.TargetMethod.Invoke(ctx.Args)
	rets := ctx.TargetMethod.GetRets()
	for i, ret := range rets {
		if ret.AssignableTo(assignableApiErrorElem) {
			_err := results[i].Interface()
			if _err != nil {
				return _err.(e.ApiError)
			}
		} else if ret.AssignableTo(assignableErrorElem) {
			_err := results[i].Interface().(error)
			if _err != nil {
				return e.NewApiError(e.Unknown, _err.Error(), _err)
			}
		} else {
			ctx.SetData(results[i].Interface())
		}
	}

	return nil
}

func (h *Handler) writeError(ctx *webctx.RequestCtx, err e.ApiError) {
	if ctx.IsAbort() {
		return
	}

	// metric
	if h.config.EnableMetrics {
		sub := time.Now().Sub(golocalv1.Get(BeginTime).(time.Time))
		// fix: 关闭协程提升性能
		h.metric.SaveMetric(h.config.Name, strconv.Itoa(err.GetCode()), ctx.GetMethod(), ctx.GetPath(), sub.Milliseconds())
	}

	ctx.SetHeader("Content-Type", "application/json; charset=UTF-8")
	ctx.SetHeader("Accept-Encoding", "gzip, br")

	res := resp.Result{
		RequestId: golocalv1.GetTraceID(),
		Error:     &e.Error{Code: err.GetCode(), Message: err.GetMessage(), Type: err.GetType(), Cause: err.GetCause()},
	}

	restful := ctx.IsRestful()
	if restful && res.Error != nil {
		ctx.WriteHeader(res.Error.GetCode())
	}

	bytes, _ := tools.Marshal(res)
	str := ctx.GetAcceptEncoding()
	if !restful {
		if strings.Contains(str, "gzip") {
			bytes = compress.AppendGzipBytesLevel(nil, bytes, 5)
			ctx.SetHeader("Content-Encoding", "gzip")
		} else if strings.Contains(str, "br") {
			tmpBytes, err := tools.Brotil(bytes)
			if err == nil {
				bytes = tmpBytes
				ctx.SetHeader("Content-Encoding", "br")
			}
		}
	}

	if _, err := ctx.Write(bytes); err != nil {
		h.logger.Error("writeResponse Error: %s", err.Error())
	}
}

func (h *Handler) writeResponse(ctx *webctx.RequestCtx) {
	if ctx.IsAbort() {
		return
	}

	// metric
	if h.config.EnableMetrics {
		sub := time.Now().Sub(golocalv1.Get(BeginTime).(time.Time))
		// fix: 关闭协程提升性能
		h.metric.SaveMetric(h.config.Name, "200", ctx.GetMethod(), ctx.GetPath(), sub.Milliseconds())
	}

	ctx.SetHeader("Content-Type", "application/json; charset=UTF-8")
	ctx.SetHeader("Accept-Encoding", "gzip, br")

	res := resp.Result{
		RequestId: golocalv1.GetTraceID(),
		Data:      ctx.GetData(),
	}

	bytes, _ := tools.Marshal(res)
	str := ctx.GetAcceptEncoding()
	if strings.Contains(str, "gzip") {
		bytes = compress.AppendGzipBytesLevel(nil, bytes, 5)
		ctx.SetHeader("Content-Encoding", "gzip")
	} else if strings.Contains(str, "br") {
		tmpBytes, err := tools.Brotil(bytes)
		if err == nil {
			bytes = tmpBytes
			ctx.SetHeader("Content-Encoding", "br")
		}
	}

	if _, err := ctx.Write(bytes); err != nil {
		h.logger.Error("writeResponse Error: %s", err.Error())
	}
}

func (h *Handler) onCrash(txt string, ctx *webctx.RequestCtx, e e.ApiError) {
	if err := recover(); err != nil {
		h.logger.Fatal("Got a runtime error %s, %v. \n%s", txt, err, string(debug.Stack()))
		h.writeError(ctx, e)
	}
}

func (h *Handler) onDoTargetMethodCrash(txt string, ctx *webctx.RequestCtx, interceptorCtx *webctx.Context, defaultErr e.ApiError) {
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

		h.writeError(ctx, defaultErr)
	}
}

var promHttpHandler = promhttp.Handler()

func (h *Handler) specialRequest(w http.ResponseWriter, r *http.Request) bool {
	switch r.URL.Path {
	case "/metrics":
		promHttpHandler.ServeHTTP(w, r)
		return true
	case "/debugxxx":
		w.WriteHeader(200)
		_, _ = w.Write([]byte("ok"))
		return true
	}
	if h.config.EnablePprof {
		if strings.HasPrefix(r.URL.Path, "/debug/pprof/") {
			handleName := strings.Replace(r.URL.Path, "/debug/pprof/", "", 1)
			switch handleName {
			case "":
				pprof.Index(w, r)
				return true
			case "profile":
				pprof.Profile(w, r)
				return true
			case "cmdline":
				pprof.Cmdline(w, r)
				return true
			case "trace":
				pprof.Trace(w, r)
				return true
			case "symbol":
				pprof.Symbol(w, r)
				return true
			}

			if runtimepprof.Lookup(handleName) != nil {
				pprof.Handler(handleName).ServeHTTP(w, r)
				return true
			}
		}
	}
	return false
}

func (h *Handler) IsRunning() bool {
	return h.running
}

func (h *Handler) SetRunning(running bool) {
	h.running = running
}

func (h *Handler) GetCtxPool() *sync.Pool {
	return &h.ctxPool
}
