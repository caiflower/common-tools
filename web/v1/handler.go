package v1

import (
	"fmt"
	"io"
	"net/http"
	"reflect"
	"runtime/debug"
	"strings"
	"time"

	"github.com/caiflower/common-tools/pkg/basic"
	golocalv1 "github.com/caiflower/common-tools/pkg/golocal/v1"
	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/caiflower/common-tools/pkg/tools"
	"github.com/caiflower/common-tools/web/e"
)

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
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var traceID string
	if h.config.HeaderTraceID != "" {
		traceID = r.Header.Get(h.config.HeaderTraceID)
	}
	if traceID == "" {
		traceID = tools.UUID()
	}
	golocalv1.PutTraceID(traceID)
	defer golocalv1.Clean()

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

func (c *RequestCtx) setResponse(v interface{}) {
	c.response = v
	c.success = 200
}

type commonResponse struct {
	RequestID string      `json:"requestId"`
	Code      *int        `json:"code,omitempty"`
	Data      interface{} `json:"data,omitempty"`
	Error     e.ApiError  `json:"error,omitempty"`
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

	// 执行目标方法
	if err := h.doTargetMethod(ctx); err != nil {
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
		return e.NewApiError(e.NotAcceptable, fmt.Sprintf("parse param failed. not support kind %s", arg.Kind()), nil)
	}

	// 非restful风格
	if !ctx.restful {
		// 先解析body，解析param
		var bytes []byte
		if r.ContentLength != 0 {
			bytes, _ = io.ReadAll(r.Body)
		} else {
			bytes = []byte("{}")
		}
		if err := tools.Unmarshal(bytes, ctx.args[0].Interface()); err != nil {
			h.logger.Warn("unmarshal failed. error: %s", err.Error())
		}

		if ctx.params != nil && len(ctx.params) > 0 {
			if err := tools.DoTagFunc(ctx.args[0].Interface(), ctx.params, []func(reflect.StructField, reflect.Value, interface{}) error{tools.SetParam}); err != nil {
				return e.NewApiError(e.NotAcceptable, fmt.Sprintf("parse params failed. detail: %s", err.Error()), err)
			}
		}

	} else {
		switch ctx.method {
		case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
			// 先解析body，解析param
			var bytes []byte
			if r.ContentLength != 0 {
				bytes, _ = io.ReadAll(r.Body)
			} else {
				bytes = []byte("{}")
			}
			if err := tools.Unmarshal(bytes, ctx.args[0].Interface()); err != nil {
				h.logger.Warn("unmarshal failed. error: %s", err.Error())
				return e.NewApiError(e.NotAcceptable, fmt.Sprintf("parse json failed."), err)
			}
		case http.MethodGet:
			if ctx.params != nil && len(ctx.params) > 0 {
				if err := tools.DoTagFunc(ctx.args[0].Interface(), ctx.params, []func(reflect.StructField, reflect.Value, interface{}) error{tools.SetParam}); err != nil {
					return e.NewApiError(e.NotAcceptable, fmt.Sprintf("parse params failed. detail: %s", err.Error()), err)
				}
			}
		}

		// 设置paths
		if ctx.paths != nil && len(ctx.paths) > 0 {
			if err := tools.DoTagFunc(ctx.args[0].Interface(), ctx.paths, []func(reflect.StructField, reflect.Value, interface{}) error{tools.SetPath}); err != nil {
				return e.NewApiError(e.NotAcceptable, fmt.Sprintf("parse paths failed. detail: %s", err.Error()), err)
			}
		}
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

	if err := tools.DoTagFunc(ctx.args[0].Interface(), ctx.paths, []func(reflect.StructField, reflect.Value, interface{}) error{tools.CheckNil, tools.CheckInList, tools.CheckRegxp, tools.Between}); err != nil {
		return e.NewApiError(e.InvalidArgument, err.Error(), nil)
	}

	return nil
}

func (h *handler) doTargetMethod(ctx *RequestCtx) e.ApiError {
	if ctx.targetMethod.GetArgs()[0].Kind() == reflect.Struct {
		ctx.args[0] = ctx.args[0].Elem()
	}

	results := ctx.targetMethod.Invoke(ctx.args)
	rets := ctx.targetMethod.GetRets()
	for i, ret := range rets {
		if ret.AssignableTo(reflect.TypeOf(new(e.ApiError)).Elem()) {
			return results[i].Interface().(e.ApiError)
		} else if ret.AssignableTo(reflect.TypeOf(new(error)).Elem()) {
			err := results[i].Interface().(error)
			return e.NewApiError(e.Unknown, err.Error(), err)
		} else {
			ctx.setResponse(results[i].Interface())
		}
	}
	return nil
}

func (h *handler) doRestful(w http.ResponseWriter, r *http.Request, ctx *RequestCtx) bool {
	return false
}

func (h *handler) writeError(w http.ResponseWriter, r *http.Request, ctx *RequestCtx, e e.ApiError) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	code := e.GetCode()
	res := commonResponse{
		RequestID: golocalv1.GetTraceID(),
		Code:      &code,
		Error:     e,
	}

	bytes, _ := tools.Marshal(res)

	if ctx.restful && res.Code != nil {
		// 如果是restful风格设置http响应码等于code
		w.WriteHeader(*res.Code)
		res.Code = nil
	} else {
		if *res.Code != 0 {
			//w.WriteHeader(http.StatusInternalServerError)
		}
	}

	if _, err := w.Write(bytes); err != nil {
		h.logger.Error("writeResponse Error: %s", err.Error())
	}
}

func (h *handler) writeResponse(w http.ResponseWriter, r *http.Request, ctx *RequestCtx) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")

	if ctx.success == 200 {
		res := commonResponse{
			RequestID: golocalv1.GetTraceID(),
			Code:      &ctx.success,
			Data:      ctx.response,
		}
		bytes, _ := tools.Marshal(res)
		if _, err := w.Write(bytes); err != nil {
			h.logger.Error("writeResponse Error: %s", err.Error())
		}
	} else {

	}
}

func (h *handler) onCrash(txt string, w http.ResponseWriter, r *http.Request, ctx *RequestCtx, e e.ApiError) {
	if err := recover(); err != nil {
		fmt.Printf("%s [ERROR] - Got a runtime error %s. %s\n%s", time.Now().Format("2006-01-02 15:04:05"), txt, r, string(debug.Stack()))
		h.writeError(w, r, ctx, e)
	}
}
