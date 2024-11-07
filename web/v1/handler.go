package v1

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"

	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/caiflower/common-tools/pkg/tools"
	"github.com/caiflower/common-tools/web"
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

	// 执行具体的业务
	h.dispatch(w, r)
}

type RequestCtx struct {
	method   string
	params   map[string][]string
	path     string
	action   string
	response interface{}
}

type commonResponse struct {
	Code  int           `json:"code"`
	Data  interface{}   `json:"data,omitempty"`
	Error *web.ApiError `json:"error,omitempty"`
}

func (h *handler) dispatch(w http.ResponseWriter, r *http.Request) {
	ctx := &RequestCtx{
		method: r.Method,
		params: r.URL.Query(),
		path:   r.URL.Path,
	}

	// 处理controller的逻辑
	if h.doController(w, r, ctx) {
		return
	}

	// 处理restful的逻辑
	if h.doRestful(w, r, ctx) {
		return
	}

	h.writeError(w, r, ctx, web.NewApiError(web.NotFound, "no such api.", nil))
}

func (h *handler) doController(w http.ResponseWriter, r *http.Request, ctx *RequestCtx) bool {
	if len(ctx.params["action"]) > 0 {
		ctx.action = ctx.params["action"][0]
	} else if len(ctx.params["Action"]) > 0 {
		ctx.action = ctx.params["Action"][0]
	}

	c := h.controllers[ctx.path]
	if c == nil {
		return false
	}

	method := c.cls.GetMethod(c.cls.GetPkgName() + "." + ctx.action)
	if method == nil {
		return false
	}

	args := method.GetArgs()
	invokeArgs := make([]reflect.Value, len(args))
	for i, arg := range args {
		kind := reflect.TypeOf(arg).Kind()
		switch kind {
		case reflect.Pointer:
			invokeArgs[i] = reflect.New(arg).Elem()
		case reflect.Struct:
			invokeArgs[i] = reflect.New(arg)
		}
	}

	if len(invokeArgs) > 0 {
		bytes, _ := io.ReadAll(r.Body)
		err := json.Unmarshal(bytes, invokeArgs[0].Interface())
		if err != nil {
			fmt.Println(err.Error())
		}
	}

	invoke := method.Invoke(invokeArgs)
	ctx.response = invoke[0].Interface()

	h.writeResponse(w, r, ctx)
	return true
}

func (h *handler) doRestful(w http.ResponseWriter, r *http.Request, ctx *RequestCtx) bool {
	return false
}

func (h *handler) writeError(w http.ResponseWriter, r *http.Request, ctx *RequestCtx, e *web.ApiError) {
	w.Header().Set("Content-Type", "application/json")
	res := commonResponse{
		Code:  e.Code,
		Error: e,
	}

	bytes, _ := tools.Marshal(res)
	w.WriteHeader(res.Code)
	if _, err := w.Write(bytes); err != nil {
		h.logger.Error("writeResponse Error: %s", err.Error())
	}
}

func (h *handler) writeResponse(w http.ResponseWriter, r *http.Request, ctx *RequestCtx) {
	w.Header().Set("Content-Type", "application/json")
	res := commonResponse{
		Code: 200,
		Data: ctx.response,
	}

	bytes, _ := tools.Marshal(res)
	if _, err := w.Write(bytes); err != nil {
		h.logger.Error("writeResponse Error: %s", err.Error())
	}
}
