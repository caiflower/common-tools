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

package app

import (
	"context"
	"io"
	"net"
	"net/http"
	"reflect"
	"sync"

	"github.com/caiflower/common-tools/pkg/tools"
	"github.com/caiflower/common-tools/pkg/tools/bytesconv"
	"github.com/caiflower/common-tools/web/app/server/render"
	"github.com/caiflower/common-tools/web/common/bytestr"
	"github.com/caiflower/common-tools/web/common/e"
	"github.com/caiflower/common-tools/web/network"
	"github.com/caiflower/common-tools/web/protocol"
	"github.com/caiflower/common-tools/web/protocol/consts"
	"github.com/caiflower/common-tools/web/router/param"
)

var zeroTCPAddr = &net.TCPAddr{
	IP: net.IPv4zero,
}

type RequestCtx struct {
	context.Context

	// net
	httpRequest *http.Request
	writer      http.ResponseWriter

	// netpoll
	Response protocol.Response
	Request  protocol.Request
	conn     network.Conn

	// keys is a key/value pair exclusively for the context of each request.
	keys map[string]interface{}
	// This mutex protect keys map.
	mu sync.RWMutex

	data interface{}
	err  e.ApiError

	method  []byte
	action  string
	restful bool

	path  []byte
	Paths param.Params

	special int8
	// enableTrace defines whether enable trace.
	enableTrace                   bool
	networkType                   string
	disableHeaderNamesNormalizing bool
}

func (ctx *RequestCtx) SetHeader(key, value string) {
	if ctx.IsNetpoll() {
		ctx.Response.Header.Set(key, value)
		return
	}

	if ctx.disableHeaderNamesNormalizing {
		ctx.writer.Header()[key] = []string{value}
	} else {
		ctx.writer.Header().Set(key, value)
	}
}

func (ctx *RequestCtx) Write(bytes []byte) (int, error) {
	if ctx.IsNetpoll() {
		return ctx.Response.BodyWriter().Write(bytes)

	}

	return ctx.writer.Write(bytes)
}

func (ctx *RequestCtx) SetData(v interface{}) {
	ctx.data = v
}

func (ctx *RequestCtx) GetData() interface{} {
	return ctx.data
}

func (ctx *RequestCtx) SetPath(path []byte) {
	ctx.path = path
}

func (ctx *RequestCtx) GetPath() string {
	return string(ctx.path)
}

func (ctx *RequestCtx) GetParams() map[string][]string {
	if ctx.IsNetpoll() {
		var params = make(map[string][]string)
		ctx.Request.URI().QueryArgs().VisitAll(func(key, value []byte) {
			params[bytesconv.B2s(key)] = append(params[bytesconv.B2s(key)], bytesconv.B2s(value))
		})

		return params
	}

	return ctx.httpRequest.URL.Query()
}

func (ctx *RequestCtx) ComputeAction() {
	if !ctx.IsNetpoll() {
		ctx.action = ctx.httpRequest.URL.Query().Get("Action")
	} else {
		ctx.action = bytesconv.B2s(ctx.Request.URI().QueryArgs().Peek("Action"))
	}

	ctx.restful = ctx.action == ""
}

func (ctx *RequestCtx) SetAction(action string) {
	ctx.action = action
}

func (ctx *RequestCtx) GetAction() string {
	return ctx.action
}

func (ctx *RequestCtx) SetMethod(method []byte) {
	ctx.method = method
}

func (ctx *RequestCtx) GetMethod() string {
	return string(ctx.method)
}

func (ctx *RequestCtx) Method() []byte {
	return ctx.method
}

func (ctx *RequestCtx) SetHttpWriterAndRequest(w http.ResponseWriter, r *http.Request) {
	ctx.httpRequest = r
	ctx.writer = w
}

func (ctx *RequestCtx) GetResponseWriterAndRequest() (http.ResponseWriter, *http.Request) {
	if ctx.writer != nil {
		return ctx.writer, ctx.httpRequest
	}

	//request, _ := adaptor.GetCompatRequest(&ctx.Request)
	//response := adaptor.GetCompatResponseWriter(&ctx.Response)
	//return response, request
	return nil, nil
}

func (ctx *RequestCtx) UpgradeWebsocket() {
	ctx.special = 1
}

func (ctx *RequestCtx) IsAbort() bool {
	return ctx.special != 0
}

func (ctx *RequestCtx) IsRestful() bool {
	return ctx.restful
}

func (ctx *RequestCtx) Reset() {
	ctx.special = 0
	ctx.Paths = ctx.Paths[:0]
	ctx.writer = nil
	ctx.err = nil
	ctx.data = nil
	ctx.httpRequest = nil
	ctx.Response.Reset()
	ctx.Request.Reset()
	ctx.enableTrace = false
	ctx.keys = nil
}

func (ctx *RequestCtx) GetConn() network.Conn {
	return ctx.conn
}

func (ctx *RequestCtx) SetConn(conn network.Conn) *RequestCtx {
	ctx.conn = conn
	return ctx
}

func (ctx *RequestCtx) SetContext(c context.Context) *RequestCtx {
	ctx.Context = c
	return ctx
}

func (ctx *RequestCtx) GetContext() context.Context {
	return ctx.Context
}

func (ctx *RequestCtx) GetReader() network.Reader {
	return ctx.conn
}

func (ctx *RequestCtx) GetWriter() network.Writer {
	return ctx.conn
}

func (ctx *RequestCtx) GetContentEncoding() string {
	if ctx.IsNetpoll() {
		return bytesconv.B2s(ctx.Request.Header.PeekContentEncoding())
	}

	return ctx.Request.Header.Get("Content-Encoding")
}

func (ctx *RequestCtx) GetAcceptEncoding() string {
	if ctx.IsNetpoll() {
		return ctx.Request.Header.Get("Accept-Encoding")
	}

	return ctx.httpRequest.Header.Get("Accept-Encoding")
}

func (ctx *RequestCtx) GetContentLength() int64 {
	if ctx.IsNetpoll() {
		return int64(ctx.Request.Header.ContentLength())
	}

	return ctx.httpRequest.ContentLength
}

func (ctx *RequestCtx) HeaderGet(key string) string {
	if ctx.IsNetpoll() {
		return ctx.Request.Header.Get(key)
	}

	return ctx.httpRequest.Header.Get(key)
}

func (ctx *RequestCtx) URI() *protocol.URI {
	return ctx.Request.URI()
}

// Host returns requested host.
//
// The host is valid until returning from RequestHandler.
func (ctx *RequestCtx) Host() []byte {
	if ctx.IsNetpoll() {
		return ctx.URI().Host()
	}
	return bytesconv.S2b(ctx.httpRequest.Host)
}

func (ctx *RequestCtx) IsEnableTrace() bool {
	return false
}

func (ctx *RequestCtx) AbortWithMsg(msg string, statusCode int) {
	ctx.Response.Reset()
	ctx.SetStatusCode(statusCode)
	ctx.Response.Header.SetContentTypeBytes(bytestr.DefaultContentType)
	ctx.Response.SetBodyString(msg)
	ctx.Abort()
}

func (ctx *RequestCtx) Abort() {
	ctx.special = -1
}

func (ctx *RequestCtx) SetEnableTrace(b bool) {
	ctx.enableTrace = b
}

func (ctx *RequestCtx) GetBody() (body []byte) {
	if ctx.IsNetpoll() {
		body = ctx.Request.Body()
		return
	}

	body, _ = io.ReadAll(ctx.httpRequest.Body)
	return
}

func (ctx *RequestCtx) SetNetWorkType(networkType string) {
	ctx.networkType = networkType
}

func (ctx *RequestCtx) SetDisableHeaderNamesNormalizing(disable bool) {
	ctx.disableHeaderNamesNormalizing = disable
}

func (ctx *RequestCtx) IsNetpoll() bool {
	return ctx.networkType == ""
}

func (ctx *RequestContext) SetError(err e.ApiError) {
	ctx.err = err
}

func (ctx *RequestCtx) GetError() e.ApiError {
	return ctx.err
}

// bodyAllowedForStatus is a copy of http.bodyAllowedForStatus non-exported function.
func bodyAllowedForStatus(status int) bool {
	switch {
	case status >= 100 && status <= 199:
		return false
	case status == consts.StatusNoContent:
		return false
	case status == consts.StatusNotModified:
		return false
	}
	return true
}

// SetStatusCode sets response status code.
func (ctx *RequestContext) SetStatusCode(statusCode int) {
	if ctx.IsNetpoll() {
		ctx.Response.SetStatusCode(statusCode)
		return
	}

	ctx.writer.WriteHeader(statusCode)
}

// Render writes the response headers and calls render.Render to render data.
func (ctx *RequestContext) Render(code int, r render.Render) {
	ctx.SetStatusCode(code)

	if !bodyAllowedForStatus(code) {
		r.WriteContentType(&ctx.Response)
		return
	}

	if err := r.Render(&ctx.Response); err != nil {
		panic(err)
	}
}

// JSON serializes the given struct as JSON into the response body.
//
// It also sets the Content-Type as "application/json".
func (ctx *RequestContext) JSON(code int, obj interface{}) {
	if ctx.IsNetpoll() {
		ctx.Render(code, render.JSONRender{Data: obj})
		return
	}

	ctx.SetHeader(consts.HeaderContentType, consts.MIMEApplicationJSONUTF8)
	ctx.SetStatusCode(code)
	toByte, _ := tools.ToByte(obj)
	ctx.Write(toByte)
}

// SetBodyString sets response body to the given value.
func (ctx *RequestContext) SetBodyString(body string) {
	ctx.Response.SetBodyString(body)
}

// RemoteAddr returns client address for the given request.
//
// If address is nil, it will return zeroTCPAddr.
func (ctx *RequestContext) RemoteAddr() net.Addr {
	if ctx.conn == nil {
		return zeroTCPAddr
	}
	addr := ctx.conn.RemoteAddr()
	if addr == nil {
		return zeroTCPAddr
	}
	return addr
}

func (ctx *RequestContext) GetStatusCode() int {
	if ctx.IsNetpoll() {
		return ctx.Response.Header.StatusCode()
	}

	return 200
}

// Set is used to store a new key/value pair exclusively for this context.
// It also lazy initializes  c.Keys if it was not used previously.
func (ctx *RequestContext) Set(key string, value interface{}) {
	ctx.mu.Lock()
	if ctx.keys == nil {
		ctx.keys = make(map[string]interface{})
	}

	ctx.keys[key] = value
	ctx.mu.Unlock()
}

// Get returns the value for the given key, ie: (value, true).
// If the value does not exist it returns (nil, false)
func (ctx *RequestContext) Get(key string) (value interface{}, exists bool) {
	ctx.mu.RLock()
	value, exists = ctx.keys[key]
	ctx.mu.RUnlock()
	return
}

type HandlerFunc func(c context.Context, ctx *RequestContext)

// HandlersChain defines a HandlerFunc array.
type HandlersChain []HandlerFunc

type HandlerNameOperator interface {
	SetHandlerName(handler HandlerFunc, name string)
	GetHandlerName(handler HandlerFunc) string
}

func SetHandlerNameOperator(o HandlerNameOperator) {
	inbuiltHandlerNameOperator = o
}

type inbuiltHandlerNameOperatorStruct struct {
	handlerNames map[uintptr]string
}

func (o *inbuiltHandlerNameOperatorStruct) SetHandlerName(handler HandlerFunc, name string) {
	o.handlerNames[getFuncAddr(handler)] = name
}

func (o *inbuiltHandlerNameOperatorStruct) GetHandlerName(handler HandlerFunc) string {
	return o.handlerNames[getFuncAddr(handler)]
}

type concurrentHandlerNameOperatorStruct struct {
	handlerNames map[uintptr]string
	lock         sync.RWMutex
}

func (o *concurrentHandlerNameOperatorStruct) SetHandlerName(handler HandlerFunc, name string) {
	o.lock.Lock()
	defer o.lock.Unlock()
	o.handlerNames[getFuncAddr(handler)] = name
}

func (o *concurrentHandlerNameOperatorStruct) GetHandlerName(handler HandlerFunc) string {
	o.lock.RLock()
	defer o.lock.RUnlock()
	return o.handlerNames[getFuncAddr(handler)]
}

func SetConcurrentHandlerNameOperator() {
	SetHandlerNameOperator(&concurrentHandlerNameOperatorStruct{handlerNames: map[uintptr]string{}})
}

func init() {
	inbuiltHandlerNameOperator = &inbuiltHandlerNameOperatorStruct{handlerNames: map[uintptr]string{}}
}

var inbuiltHandlerNameOperator HandlerNameOperator

func SetHandlerName(handler HandlerFunc, name string) {
	inbuiltHandlerNameOperator.SetHandlerName(handler, name)
}

func GetHandlerName(handler HandlerFunc) string {
	return inbuiltHandlerNameOperator.GetHandlerName(handler)
}

func getFuncAddr(v interface{}) uintptr {
	return reflect.ValueOf(reflect.ValueOf(v)).Field(1).Pointer()
}
