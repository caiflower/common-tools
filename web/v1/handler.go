package v1

import (
	"fmt"
	"io"
	"net/http"
	"reflect"

	"github.com/caiflower/common-tools/pkg/basic"
	crash "github.com/caiflower/common-tools/pkg/e"
	golocalv1 "github.com/caiflower/common-tools/pkg/golocal/v1"
	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/caiflower/common-tools/pkg/tools"
	"github.com/caiflower/common-tools/web/e"
)

var commonHandler *handler

func initHandler(config *Config, logger logger.ILog) {
	commonHandler = &handler{
		config:      config,
		controllers: make(map[string]*controller),
		logger:      logger,
	}
}

type handler struct {
	config      *Config
	controllers map[string]*controller
	logger      logger.ILog
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
		method:  r.Method,
		params:  r.URL.Query(),
		path:    r.URL.Path,
		success: 500,
	}

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

	}

	return ctx.targetMethod != nil
}

func (h *handler) setArgs(r *http.Request, ctx *RequestCtx) e.ApiError {
	method := ctx.targetMethod

	// 非restful风格
	if !ctx.restful {
		arg := method.GetArgs()[0]
		switch arg.Kind() {
		case reflect.Ptr:
			ctx.args = append(ctx.args, reflect.New(arg.Elem()))
		case reflect.Struct:
			ctx.args = append(ctx.args, reflect.New(arg))
		default:
			return e.NewApiError(e.NotAcceptable, fmt.Sprintf("parse param failed. not support kind %s", arg.Kind()), nil)
		}

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
				return e.NewApiError(e.NotAcceptable, fmt.Sprintf("parse param tag failed. detail: %s", err.Error()), err)
			}
		}

		if arg.Kind() == reflect.Struct {
			ctx.args[0] = ctx.args[0].Elem()
		}
	} else {

	}

	return nil
}

func (h *handler) validArgs(ctx *RequestCtx) e.ApiError {
	return nil
}

func (h *handler) doTargetMethod(ctx *RequestCtx) e.ApiError {
	defer crash.OnError("doTargetMethod failed!")

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
		h.writeError(w, r, ctx, e.NewApiError(e.Internal, "unknown error", nil))
	}
}
