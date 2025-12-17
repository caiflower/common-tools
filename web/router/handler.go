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
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	"github.com/caiflower/common-tools/web/common/e"
	"github.com/caiflower/common-tools/web/common/interceptor"
	"github.com/caiflower/common-tools/web/common/metric"
	"github.com/caiflower/common-tools/web/common/reflectx"
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
	Debug                 bool          `yaml:"debug"`
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
			Paths: make(param.Params, 0, 1),
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
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer golocalv1.Clean()
	if h.serverCommon(w, r) {
		return
	}

	ctx := h.getRequestContextWithRequest(w, r)
	defer h.putRequestContext(ctx)

	// dispatch
	h.Dispatch(ctx, w, r)
}

func (h *Handler) ServeHTTPWithRequestContext(ctx *webctx.RequestCtx, w http.ResponseWriter, r *http.Request) {
	defer golocalv1.Clean()
	if h.serverCommon(w, r) {
		return
	}

	// dispatch
	h.Dispatch(ctx, w, r)
}

func (h *Handler) serverCommon(w http.ResponseWriter, r *http.Request) (done bool) {
	if h.specialRequest(w, r) {
		done = true
		return
	}

	var traceID string
	if h.config.HeaderTraceID != "" {
		traceID = r.Header.Get(h.config.HeaderTraceID)
	}
	if traceID == "" {
		traceID = tools.UUID()
		if h.config.HeaderTraceID != "" {
			r.Header.Set(h.config.HeaderTraceID, traceID)
		}
	}
	golocalv1.PutTraceID(traceID)
	golocalv1.Put(BeginTime, time.Now())
	golocalv1.PutContext(r.Context())

	if h.beforeDispatchCallbackFunc != nil {
		if h.beforeDispatchCallbackFunc(w, r) {
			done = true
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

func (h *Handler) getRequestContextWithRequest(w http.ResponseWriter, r *http.Request) *webctx.RequestCtx {
	ctx := h.getRequestContext()

	ctx.Method = r.Method
	ctx.Params = r.URL.Query()
	ctx.Path = r.URL.Path
	ctx.Request = r
	ctx.Writer = w
	return ctx
}

func (h *Handler) putRequestContext(ctx *webctx.RequestCtx) {
	ctx.Reset()
	h.ctxPool.Put(ctx)
}

func (h *Handler) Dispatch(ctx *webctx.RequestCtx, w http.ResponseWriter, r *http.Request) {
	defer h.onCrash("dispatch", w, r, ctx, e.NewApiError(e.Internal, "InternalError", nil))

	// action
	h.setAction(ctx)

	// method
	if !h.setTargetMethod(r, ctx) {
		h.writeError(w, r, ctx, e.NewApiError(e.NotFound, "no such api.", nil))
		return
	}

	// set args
	webContext := ctx.ConvertToWebCtx()
	if err := h.setArgs(r, ctx, webContext); err != nil {
		h.writeError(w, r, ctx, err)
		return
	}

	// valid args
	if err := h.validArgs(ctx); err != nil {
		h.writeError(w, r, ctx, err)
		return
	}

	defer h.onDoTargetMethodCrash("doTargetMethod", w, r, ctx, webContext, e.NewApiError(e.Internal, "InternalError", nil))

	// doTargetMethod
	doTargetMethod := func() e.ApiError {
		return h.doTargetMethod(w, r, ctx)
	}

	// aop
	if err := h.interceptors.DoInterceptor(webContext, doTargetMethod); err != nil {
		h.writeError(w, r, ctx, err)
		return
	}

	// set response
	h.writeResponse(w, r, ctx)
}

func (h *Handler) setAction(ctx *webctx.RequestCtx) {
	if len(ctx.Params["action"]) > 0 {
		ctx.Action = ctx.Params["action"][0]
	} else if len(ctx.Params["Action"]) > 0 {
		ctx.Action = ctx.Params["Action"][0]
	} else {
		ctx.Restful = true
	}
}

func (h *Handler) setTargetMethod(r *http.Request, ctx *webctx.RequestCtx) bool {
	if !ctx.IsRestful() {
		// action风格
		c := h.controllers[ctx.Path]
		if c != nil {
			cls := c.GetCls()
			method := cls.GetMethod(cls.GetPkgName() + "." + ctx.Action)
			ctx.TargetMethod = method
		}
	} else {
		// restful
		tree := h.trees.get(ctx.Method)
		if tree != nil {
			res := tree.find(ctx.Path, &ctx.Paths, false)
			if res.handlers != nil {
				ctx.TargetMethod = res.handlers[0]
			}
		}
	}

	return ctx.TargetMethod != nil
}

func (h *Handler) setArgs(r *http.Request, ctx *webctx.RequestCtx, webContext *webctx.Context) e.ApiError {
	method := ctx.TargetMethod

	if !method.HasArgs() {
		return nil
	}

	arg := method.GetArgs()[0]
	switch arg.Kind() {
	case reflect.Ptr:
		ctx.Args = append(ctx.Args, reflect.New(arg.Elem()))
	case reflect.Struct:
		ctx.Args = append(ctx.Args, reflect.New(arg))
	default:
		return e.NewApiError(e.InvalidArgument, fmt.Sprintf("parse param failed. not support kind %s", arg.Kind()), nil)
	}

	var bytes []byte
	if r.ContentLength != 0 {
		bytes, _ = io.ReadAll(r.Body)
		if isGzip(r.Header) {
			tmpBytes, err := tools.Gunzip(bytes)
			if err != nil {
				return e.NewApiError(e.InvalidArgument, fmt.Sprintf("parse param failed. ungzip failed. %s", err.Error()), nil)
			}

			bytes = tmpBytes
		} else if isBr(r.Header) {
			tmpBytes, err := tools.UnBrotil(bytes)
			if err != nil {
				return e.NewApiError(e.InvalidArgument, fmt.Sprintf("parse param failed. unbr failed. %s", err.Error()), nil)
			}

			bytes = tmpBytes
		}
	} else {
		bytes = []byte("{}")
	}

	// set context
	indirect := reflect.Indirect(reflect.ValueOf(ctx.Args[0].Interface()))
	for i := 0; i < indirect.NumField(); i++ {
		field := indirect.Field(i)
		if field.Type().AssignableTo(assignableWebContextElem) {
			field.Set(reflect.ValueOf(*webContext))
			break
		} else if field.Type().AssignableTo(assignableWebContext) {
			field.Set(reflect.ValueOf(webContext))
			break
		}
	}

	fnObjs := make([]tools.FnObj, 0, 10)
	// 非restful风格
	if !ctx.IsRestful() {
		// 先解析body，解析param

		if err := tools.Unmarshal(bytes, ctx.Args[0].Interface()); err != nil {
			err = json.Unmarshal(bytes, ctx.Args[0].Interface())
			var typeError *json.UnmarshalTypeError
			if errors.As(err, &typeError) {
				return e.NewApiError(e.InvalidArgument, fmt.Sprintf("Malformed %s type '%s'", reflect.TypeOf(ctx.Args[0].Interface()).Elem().Name()+"."+typeError.Field, typeError.Value), err)
			}

			if len(ctx.Params) > 0 {
				fnObjs = append(fnObjs, tools.FnObj{
					Fn:   reflectx.SetParam,
					Data: ctx.Params,
				})
			}

			return e.NewApiError(e.InvalidArgument, fmt.Sprintf("%s", err.Error()), err)
		}
	} else {
		switch ctx.Method {
		case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
			// 先解析body，解析param
			if err := tools.Unmarshal(bytes, ctx.Args[0].Interface()); err != nil {
				var typeError *json.UnmarshalTypeError
				if errors.As(err, &typeError) {
					return e.NewApiError(e.InvalidArgument, fmt.Sprintf("Malformed %s type '%s'", reflect.TypeOf(ctx.Args[0].Interface()).Elem().Name()+"."+typeError.Field, typeError.Value), err)
				}

				return e.NewApiError(e.InvalidArgument, fmt.Sprintf("%s", err.Error()), err)
			}
		case http.MethodGet:
			if len(ctx.Params) > 0 {
				fnObjs = append(fnObjs, tools.FnObj{
					Fn:   reflectx.SetParam,
					Data: ctx.Params,
				})
			}
		}

		// set paths
		if len(ctx.Paths) > 0 {
			fnObjs = append(fnObjs, tools.FnObj{
				Fn:   reflectx.SetPath,
				Data: ctx.Paths,
			})
		}
	}

	// set header
	fnObjs = append(fnObjs, tools.FnObj{
		Fn:   reflectx.SetHeader,
		Data: r.Header,
	})

	// set default
	fnObjs = append(fnObjs, tools.FnObj{
		Fn:   tools.SetDefaultValueIfNil,
		Data: r.Header,
	})

	if err := tools.DoTagFunc(ctx.Args[0].Interface(), fnObjs); err != nil {
		logger.Error("do tag function failed.", err)
		return e.NewInternalError(err)
	}

	return nil
}

func (h *Handler) validArgs(ctx *webctx.RequestCtx) e.ApiError {
	method := ctx.TargetMethod
	if !method.HasArgs() {
		return nil
	}

	elem := reflect.TypeOf(ctx.Args[0].Interface()).Elem()
	pkgPath := elem.PkgPath()
	if err := tools.DoTagFunc(ctx.Args[0].Interface(), []tools.FnObj{
		{
			Fn: reflectx.CheckParam,
			Data: reflectx.ValidObject{
				PkgPath:   pkgPath,
				FiledName: elem.Name(),
			}},
	}); err != nil {
		return e.NewApiError(e.InvalidArgument, err.Error(), nil)
	}

	return nil
}

func (h *Handler) doTargetMethod(w http.ResponseWriter, r *http.Request, ctx *webctx.RequestCtx) (err e.ApiError) {
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
			ctx.SetResponse(results[i].Interface())
		}
	}

	return nil
}

func (h *Handler) writeError(w http.ResponseWriter, r *http.Request, ctx *webctx.RequestCtx, err e.ApiError) {
	if ctx.IsSpecial() {
		return
	}

	// 记录metric
	sub := time.Now().Sub(golocalv1.Get(BeginTime).(time.Time))
	go h.metric.SaveMetric(h.config.Name, strconv.Itoa(err.GetCode()), ctx.Method, ctx.Path, sub.Milliseconds())

	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	res := resp.Result{
		RequestId: golocalv1.GetTraceID(),
		Error:     &e.Error{Code: err.GetCode(), Message: err.GetMessage(), Type: err.GetType(), Cause: err.GetCause()},
	}

	restful := ctx.IsRestful()
	if restful && res.Error != nil {
		w.WriteHeader(res.Error.GetCode())
	}

	bytes, _ := tools.Marshal(res)
	if !restful {
		if acceptGzip(r.Header) {
			tmpBytes, err := tools.Gzip(bytes)
			if err == nil {
				bytes = tmpBytes
				w.Header().Set("Content-Encoding", "gzip")
			}
		} else if acceptBr(r.Header) {
			tmpBytes, err := tools.Brotil(bytes)
			if err == nil {
				bytes = tmpBytes
				w.Header().Set("Content-Encoding", "br")
			}
		}
	}

	w.Header().Set("Accept-Encoding", "gzip, br")
	if _, err := w.Write(bytes); err != nil {
		h.logger.Error("writeResponse Error: %s", err.Error())
	}
}

func (h *Handler) writeResponse(w http.ResponseWriter, r *http.Request, ctx *webctx.RequestCtx) {
	if ctx.IsSpecial() {
		return
	}

	// 记录metric
	sub := time.Now().Sub(golocalv1.Get(BeginTime).(time.Time))
	go h.metric.SaveMetric(h.config.Name, "200", ctx.Method, ctx.Path, sub.Milliseconds())

	w.Header().Set("Content-Type", "application/json; charset=UTF-8")

	res := resp.Result{
		RequestId: golocalv1.GetTraceID(),
		Data:      ctx.GetResponse(),
	}

	bytes, _ := tools.Marshal(res)
	if acceptGzip(r.Header) {
		tmpBytes, err := tools.Gzip(bytes)
		if err == nil {
			bytes = tmpBytes
			w.Header().Set("Content-Encoding", "gzip")
		}
	} else if acceptBr(r.Header) {
		tmpBytes, err := tools.Brotil(bytes)
		if err == nil {
			bytes = tmpBytes
			w.Header().Set("Content-Encoding", "br")
		}
	}

	w.Header().Set("Accept-Encoding", "gzip, br")
	if _, err := w.Write(bytes); err != nil {
		h.logger.Error("writeResponse Error: %s", err.Error())
	}
}

func (h *Handler) onCrash(txt string, w http.ResponseWriter, r *http.Request, ctx *webctx.RequestCtx, e e.ApiError) {
	if err := recover(); err != nil {
		fmt.Printf("Got a runtime error %s, %s. %v\n%s", txt, err, r, string(debug.Stack()))
		h.writeError(w, r, ctx, e)
	}
}

func (h *Handler) onDoTargetMethodCrash(txt string, w http.ResponseWriter, r *http.Request, ctx *webctx.RequestCtx, interceptorCtx *webctx.Context, defaultErr e.ApiError) {
	if err := recover(); err != nil {
		h.logger.Error("Got a runtime error %s, %s. %v\n%s", txt, err, r, string(debug.Stack()))

		// onPanic
		for _, v := range h.interceptors {
			apiError := v.Interceptor.OnPanic(interceptorCtx, err)
			if apiError != nil {
				defaultErr = apiError
				break
			}
		}

		h.writeError(w, r, ctx, defaultErr)
	}
}

func acceptGzip(header http.Header) bool {
	if header == nil {
		return false
	}
	return strings.Contains(header.Get("Accept-Encoding"), "gzip")
}

func acceptBr(header http.Header) bool {
	if header == nil {
		return false
	}
	return strings.Contains(header.Get("Accept-Encoding"), "br")
}

func isGzip(header http.Header) bool {
	if header == nil {
		return false
	}
	return strings.Contains(header.Get("Content-Encoding"), "gzip")
}

func isBr(header http.Header) bool {
	if header == nil {
		return false
	}
	return strings.Contains(header.Get("Content-Encoding"), "br")
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
