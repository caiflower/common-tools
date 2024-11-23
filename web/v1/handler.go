package v1

import (
	"fmt"
	"io"
	"net/http"
	"reflect"
	"runtime/debug"
	"strings"

	"github.com/caiflower/common-tools/pkg/basic"
	golocalv1 "github.com/caiflower/common-tools/pkg/golocal/v1"
	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/caiflower/common-tools/pkg/tools"
	"github.com/caiflower/common-tools/web/e"
	"github.com/caiflower/common-tools/web/interceptor"
)

// BeforeDispatchCallbackFunc 在进行分发前进行回调的函数, 返回true结束
type BeforeDispatchCallbackFunc func(w http.ResponseWriter, r *http.Request) bool

var commonHandler *handler

func initHandler(config *Config, logger logger.ILog) {
	commonHandler = &handler{
		config:       config,
		controllers:  make(map[string]*controller),
		restfulPaths: make(map[string]struct{}),
		logger:       logger,
	}
}

type handler struct {
	config             *Config
	controllers        map[string]*controller
	restfulControllers []*RestfulController
	restfulPaths       map[string]struct{}
	logger             logger.ILog

	beforeDispatchCallbackFunc BeforeDispatchCallbackFunc
	paramsValidFuncList        []func(reflect.StructField, reflect.Value, interface{}) error
	interceptors               interceptor.ItemSort
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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
	defer golocalv1.Clean()

	if h.beforeDispatchCallbackFunc != nil {
		if h.beforeDispatchCallbackFunc(w, r) {
			return
		}
	}

	// 执行具体的业务
	h.dispatch(w, r)
}

type RequestCtx struct {
	method       string
	params       map[string][]string
	path         string
	version      string
	paths        map[string]string
	action       string
	restful      bool
	args         []reflect.Value
	targetMethod *basic.Method
	response     interface{}
	success      int
}

func (c *RequestCtx) convert2InterceptorCtx() *interceptor.Context {
	return &interceptor.Context{Ctx: c, Attributes: make(map[string]interface{})}
}

func (c *RequestCtx) SetResponse(v interface{}) {
	c.response = v
	c.success = 200
}

func (c *RequestCtx) IsFinish() bool {
	return c.success == 200
}

func (c *RequestCtx) GetPath() string {
	return c.path
}

func (c *RequestCtx) GetPathParam() map[string]string {
	return c.paths
}

func (c *RequestCtx) GetParams() map[string][]string {
	return c.params
}

type CommonResponse struct {
	RequestId string
	Code      *int        `json:",omitempty"`
	Data      interface{} `json:",omitempty"`
	Error     e.ApiError  `json:",omitempty"`
}

func (h *handler) dispatch(w http.ResponseWriter, r *http.Request) {
	ctx := &RequestCtx{
		method: r.Method,
		params: r.URL.Query(),
		paths:  make(map[string]string),
		path:   r.URL.Path,
	}

	defer h.onCrash("dispatch", w, r, ctx, e.NewApiError(e.Internal, "InternalError", nil))

	// 设置action
	h.setAction(ctx)

	// 设置目标method
	if !h.setTargetMethod(r, ctx) {
		h.writeError(w, r, ctx, e.NewApiError(e.NotFound, "no such api.", nil))
		return
	}

	// 设置args
	if err := h.setArgs(r, ctx); err != nil {
		h.writeError(w, r, ctx, err)
		return
	}

	// 校验args
	if err := h.validArgs(ctx); err != nil {
		h.writeError(w, r, ctx, err)
		return
	}

	interceptorCtx := ctx.convert2InterceptorCtx()
	defer h.onDoTargetMethodCrash("doTargetMethod", w, r, ctx, interceptorCtx, e.NewApiError(e.Internal, "InternalError", nil))

	// 执行目标方法
	doTargetMethod := func() e.ApiError {
		return h.doTargetMethod(w, r, ctx)
	}

	// aop切入
	if err := h.interceptors.DoInterceptor(interceptorCtx, doTargetMethod); err != nil {
		h.writeError(w, r, ctx, err)
		return
	}

	// 返回数据
	h.writeResponse(w, r, ctx)
}

func (h *handler) setAction(ctx *RequestCtx) {
	if len(ctx.params["action"]) > 0 {
		ctx.action = ctx.params["action"][0]
	} else if len(ctx.params["Action"]) > 0 {
		ctx.action = ctx.params["Action"][0]
	}
}

func (h *handler) setTargetMethod(r *http.Request, ctx *RequestCtx) bool {
	if ctx.action != "" {
		// action风格
		ctx.restful = false
		c := h.controllers[ctx.path]
		if c != nil {
			method := c.cls.GetMethod(c.cls.GetPkgName() + "." + ctx.action)
			ctx.targetMethod = method
		}
	} else {
		// 根据restful风格查询method
		ctx.restful = true
		for _, c := range h.restfulControllers {
			if c.method == ctx.method && tools.MatchReg(ctx.path, "/"+c.version+c.path) {
				ctx.version = c.version
				ctx.targetMethod = c.targetMethod

				path := strings.TrimPrefix(ctx.path, "/"+c.version)
				k := 0
				for i, cur := range c.otherPaths {
					next := ""
					if i+1 < len(c.otherPaths) {
						next = c.otherPaths[i+1]
					}
					path = strings.TrimPrefix(path, "/"+cur+"/")
					pathValue := ""
					for j := 0; j < len(path); j++ {
						if next != "" && strings.HasPrefix(path[j:], "/"+next+"/") {
							pathValue = path[:j]
							path = path[j:]
							break
						} else if next == "" {
							pathValue = path[j:]
							break
						}
					}

					if pathValue != "" {
						ctx.paths[c.pathParams[k]] = pathValue
						k++
					}
				}

				break
			}
		}
	}

	return ctx.targetMethod != nil
}

func (h *handler) setArgs(r *http.Request, ctx *RequestCtx) e.ApiError {
	method := ctx.targetMethod

	if !method.HasArgs() {
		return nil
	}

	arg := method.GetArgs()[0]
	switch arg.Kind() {
	case reflect.Ptr:
		ctx.args = append(ctx.args, reflect.New(arg.Elem()))
	case reflect.Struct:
		ctx.args = append(ctx.args, reflect.New(arg))
	default:
		return e.NewApiError(e.InvalidArgument, fmt.Sprintf("parse param failed. not support kind %s", arg.Kind()), nil)
	}

	// 非restful风格
	if !ctx.restful {
		// 先解析body，解析param
		var bytes []byte
		if r.ContentLength != 0 {
			bytes, _ = io.ReadAll(r.Body)
			if isGzip(r.Header) {
				tmpBytes, err := tools.Gunzip(bytes)
				if err != nil {
					return e.NewApiError(e.InvalidArgument, fmt.Sprintf("parse param failed. ungzip failed. %s", err.Error()), nil)
				} else {
					bytes = tmpBytes
				}
			} else if isBr(r.Header) {
				tmpBytes, err := tools.UnBrotil(bytes)
				if err != nil {
					return e.NewApiError(e.InvalidArgument, fmt.Sprintf("parse param failed. unbr failed. %s", err.Error()), nil)
				} else {
					bytes = tmpBytes
				}
			}
		} else {
			bytes = []byte("{}")
		}
		if err := tools.Unmarshal(bytes, ctx.args[0].Interface()); err != nil {
			h.logger.Warn("unmarshal failed. error: %s", err.Error())
		}

		if ctx.params != nil && len(ctx.params) > 0 {
			if err := tools.DoTagFunc(ctx.args[0].Interface(), ctx.params, []func(reflect.StructField, reflect.Value, interface{}) error{setParam}); err != nil {
				return e.NewApiError(e.InvalidArgument, fmt.Sprintf("parse params failed. detail: %s", err.Error()), err)
			}
		}

	} else {
		switch ctx.method {
		case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
			// 先解析body，解析param
			var bytes []byte
			if r.ContentLength != 0 {
				bytes, _ = io.ReadAll(r.Body)
				if isGzip(r.Header) {
					tmpBytes, err := tools.Gunzip(bytes)
					if err != nil {
						return e.NewApiError(e.InvalidArgument, fmt.Sprintf("parse param failed. ungzip failed. %s", err.Error()), nil)
					} else {
						bytes = tmpBytes
					}
				} else if isBr(r.Header) {
					tmpBytes, err := tools.UnBrotil(bytes)
					if err != nil {
						return e.NewApiError(e.InvalidArgument, fmt.Sprintf("parse param failed. unbr failed. %s", err.Error()), nil)
					} else {
						bytes = tmpBytes
					}
				}
			} else {
				bytes = []byte("{}")
			}
			if err := tools.Unmarshal(bytes, ctx.args[0].Interface()); err != nil {
				h.logger.Warn("unmarshal failed. error: %s", err.Error())
				return e.NewApiError(e.InvalidArgument, fmt.Sprintf("parse json failed."), err)
			}
		case http.MethodGet:
			if ctx.params != nil && len(ctx.params) > 0 {
				if err := tools.DoTagFunc(ctx.args[0].Interface(), ctx.params, []func(reflect.StructField, reflect.Value, interface{}) error{setParam}); err != nil {
					return e.NewApiError(e.InvalidArgument, fmt.Sprintf("parse params failed. detail: %s", err.Error()), err)
				}
			}
		}

		// 设置paths
		if ctx.paths != nil && len(ctx.paths) > 0 {
			if err := tools.DoTagFunc(ctx.args[0].Interface(), ctx.paths, []func(reflect.StructField, reflect.Value, interface{}) error{setPath}); err != nil {
				return e.NewApiError(e.InvalidArgument, fmt.Sprintf("parse paths failed. detail: %s", err.Error()), err)
			}
		}
	}

	// 设置header
	if err := tools.DoTagFunc(ctx.args[0].Interface(), r.Header, []func(reflect.StructField, reflect.Value, interface{}) error{setHeader}); err != nil {
		return e.NewApiError(e.Internal, fmt.Sprintf("set header failed. detail: %s", err.Error()), err)
	}

	// 设置默认值
	if err := tools.DoTagFunc(ctx.args[0].Interface(), ctx.paths, []func(reflect.StructField, reflect.Value, interface{}) error{tools.SetDefaultValueIfNil}); err != nil {
		return e.NewApiError(e.Internal, fmt.Sprintf("set default value failed. detail: %s", err.Error()), err)
	}

	return nil
}

func (h *handler) validArgs(ctx *RequestCtx) e.ApiError {
	method := ctx.targetMethod
	if !method.HasArgs() {
		return nil
	}

	funcList := []func(reflect.StructField, reflect.Value, interface{}) error{tools.CheckNil, tools.CheckInList, tools.CheckRegxp, tools.CheckBetween, tools.CheckLen}
	funcList = append(funcList, h.paramsValidFuncList...)
	if err := tools.DoTagFunc(ctx.args[0].Interface(), ctx.paths, funcList); err != nil {
		return e.NewApiError(e.InvalidArgument, err.Error(), nil)
	}

	return nil
}

func (h *handler) doTargetMethod(w http.ResponseWriter, r *http.Request, ctx *RequestCtx) (err e.ApiError) {
	if ctx.targetMethod.HasArgs() {
		if ctx.targetMethod.GetArgs()[0].Kind() == reflect.Struct {
			ctx.args[0] = ctx.args[0].Elem()
		}
	}

	results := ctx.targetMethod.Invoke(ctx.args)
	rets := ctx.targetMethod.GetRets()
	for i, ret := range rets {
		if ret.AssignableTo(reflect.TypeOf(new(e.ApiError)).Elem()) {
			_err := results[i].Interface()
			if _err != nil {
				return _err.(e.ApiError)
			}
		} else if ret.AssignableTo(reflect.TypeOf(new(error)).Elem()) {
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

func (h *handler) writeError(w http.ResponseWriter, r *http.Request, ctx *RequestCtx, e e.ApiError) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	code := e.GetCode()
	res := CommonResponse{
		RequestId: golocalv1.GetTraceID(),
		Code:      &code,
		Error:     e,
	}

	if ctx.restful && res.Code != nil {
		// 如果是restful风格设置http响应码等于code
		w.WriteHeader(*res.Code)
		res.Code = nil
	} else {
		if *res.Code != 0 {
			//w.WriteHeader(http.StatusInternalServerError)
		}
	}
	bytes, _ := tools.Marshal(res)
	if !ctx.restful && acceptGzip(r.Header) {
		tmpBytes, err := tools.Gzip(bytes)
		if err == nil {
			bytes = tmpBytes
			w.Header().Set("Content-Encoding", "gzip")
		}
	} else if !ctx.restful && acceptBr(r.Header) {
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

func (h *handler) writeResponse(w http.ResponseWriter, r *http.Request, ctx *RequestCtx) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")

	res := CommonResponse{
		RequestId: golocalv1.GetTraceID(),
		Code:      &ctx.success,
		Data:      ctx.response,
	}

	if ctx.restful {
		res.Code = nil
	} else {
		if *res.Code == 200 {
			*res.Code = 0
		}
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

func (h *handler) onCrash(txt string, w http.ResponseWriter, r *http.Request, ctx *RequestCtx, e e.ApiError) {
	if err := recover(); err != nil {
		fmt.Printf("Got a runtime error %s, %s. %v\n%s", txt, err, r, string(debug.Stack()))
		h.writeError(w, r, ctx, e)
	}
}

func (h *handler) onDoTargetMethodCrash(txt string, w http.ResponseWriter, r *http.Request, ctx *RequestCtx, interceptorCtx *interceptor.Context, defaultErr e.ApiError) {
	if err := recover(); err != nil {
		h.logger.Error("Got a runtime error %s, %s. %v\n%s", txt, err, r, string(debug.Stack()))

		// onPanic
		for _, v := range h.interceptors {
			apiError := v.Interceptor.OnPanic(interceptorCtx)
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
