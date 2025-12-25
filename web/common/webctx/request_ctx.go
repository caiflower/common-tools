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

package webctx

import (
	"context"
	"net/http"
	"reflect"

	"github.com/caiflower/common-tools/pkg/basic"
	"github.com/caiflower/common-tools/pkg/tools/bytesconv"
	"github.com/caiflower/common-tools/web/common/adaptor"
	"github.com/caiflower/common-tools/web/common/bytestr"
	"github.com/caiflower/common-tools/web/network"
	"github.com/caiflower/common-tools/web/protocol"
	"github.com/caiflower/common-tools/web/router/param"
)

type RequestCtx struct {
	ctx context.Context

	// net
	HttpRequest *http.Request
	Writer      http.ResponseWriter

	// netpoll
	Response protocol.Response
	Request  protocol.Request
	conn     network.Conn

	isFinish bool
	Action   string
	data     interface{}

	method  []byte
	Restful bool

	path  []byte
	Paths param.Params

	Args         []reflect.Value
	TargetMethod *basic.Method
	Special      int8
	// enableTrace defines whether enable trace.
	enableTrace bool
}

func (c *RequestCtx) ConvertToWebCtx() *Context {
	return &Context{RequestContext: c}
}

func (c *RequestCtx) SetHeader(key, value string) {
	if c.Writer != nil {
		c.Writer.Header().Set(key, value)
		return
	}

	c.Response.Header.Set(key, value)
}

func (c *RequestCtx) WriteHeader(statusCode int) {
	if c.Writer != nil {
		c.Writer.WriteHeader(statusCode)
		return
	}

	c.Response.Header.SetStatusCode(statusCode)
}

func (c *RequestCtx) Write(bytes []byte) (int, error) {
	if c.Writer != nil {
		return c.Writer.Write(bytes)
	}

	return c.Response.BodyWriter().Write(bytes)
}

func (c *RequestCtx) SetData(v interface{}) {
	c.isFinish = true
	c.data = v
}

func (c *RequestCtx) GetData() interface{} {
	return c.data
}

func (c *RequestCtx) IsFinish() bool {
	return c.isFinish
}

func (c *RequestCtx) SetPath(path []byte) {
	c.path = path
}

func (c *RequestCtx) GetPath() string {
	return bytesconv.B2s(c.path)
}

func (c *RequestCtx) GetParams() map[string][]string {
	if c.HttpRequest != nil {
		return c.HttpRequest.URL.Query()
	}

	var params = make(map[string][]string)
	c.Request.URI().QueryArgs().VisitAll(func(key, value []byte) {
		params[bytesconv.B2s(key)] = append(params[bytesconv.B2s(key)], bytesconv.B2s(value))
	})

	return params
}

func (c *RequestCtx) SetAction() string {
	if c.HttpRequest != nil {
		return c.HttpRequest.URL.Query().Get("Action")
	}
	return bytesconv.B2s(c.Request.URI().QueryArgs().Peek("Action"))
}

func (c *RequestCtx) GetAction() string {
	return c.Action
}

func (c *RequestCtx) SetMethod(method []byte) {
	c.method = method
}

func (c *RequestCtx) GetMethod() string {
	return bytesconv.B2s(c.method)
}

func (c *RequestCtx) Method() []byte {
	return c.method
}

func (c *RequestCtx) GetResponseWriterAndRequest() (http.ResponseWriter, *http.Request) {
	if c.Writer != nil {
		return c.Writer, c.HttpRequest
	}

	request, _ := adaptor.GetCompatRequest(&c.Request)
	response := adaptor.GetCompatResponseWriter(&c.Response)
	return response, request
}

func (c *RequestCtx) UpgradeWebsocket() {
	c.Special = 1
}

func (c *RequestCtx) IsAbort() bool {
	return c.Special != 0
}

func (c *RequestCtx) IsRestful() bool {
	return c.Restful
}

func (c *RequestCtx) Reset() {
	c.TargetMethod = nil
	c.Args = c.Args[:0]
	c.Special = 0
	c.Paths = c.Paths[:0]
	c.isFinish = false
	c.Writer = nil
	c.HttpRequest = nil
	c.Response.Reset()
	c.Request.Reset()
	c.enableTrace = false
}

func (c *RequestCtx) GetConn() network.Conn {
	return c.conn
}

func (c *RequestCtx) SetConn(conn network.Conn) *RequestCtx {
	c.conn = conn
	return c
}

func (c *RequestCtx) SetContext(ctx context.Context) *RequestCtx {
	c.ctx = ctx
	return c
}

func (c *RequestCtx) GetContext() context.Context {
	return c.ctx
}

func (c *RequestCtx) GetReader() network.Reader {
	return c.conn
}

func (c *RequestCtx) GetWriter() network.Writer {
	return c.conn
}

func (c *RequestCtx) GetContentEncoding() string {
	if c.HttpRequest != nil {
		return c.Request.Header.Get("Content-Encoding")
	}

	return bytesconv.B2s(c.Request.Header.PeekContentEncoding())
}

func (c *RequestCtx) GetAcceptEncoding() string {
	if c.HttpRequest != nil {
		return c.HttpRequest.Header.Get("Accept-Encoding")
	}

	return c.Request.Header.Get("Accept-Encoding")
}

func (c *RequestCtx) GetContentLength() int64 {
	if c.HttpRequest != nil {
		return c.HttpRequest.ContentLength
	}

	return int64(c.Request.Header.ContentLength())
}

func (c *RequestCtx) HeaderGet(key string) string {
	if c.HttpRequest != nil {
		return c.HttpRequest.Header.Get(key)
	}
	return c.Request.Header.Get(key)
}

func (c *RequestCtx) URI() *protocol.URI {
	return c.Request.URI()
}

// Host returns requested host.
//
// The host is valid until returning from RequestHandler.
func (c *RequestCtx) Host() []byte {
	if c.HttpRequest != nil {
		return bytesconv.S2b(c.HttpRequest.Host)
	}
	return c.URI().Host()
}

func (c *RequestCtx) IsEnableTrace() bool {
	return false
}

func (c *RequestCtx) AbortWithMsg(msg string, statusCode int) {
	c.Response.Reset()
	c.WriteHeader(statusCode)
	c.Response.Header.SetContentTypeBytes(bytestr.DefaultContentType)
	c.Response.SetBodyString(msg)
	c.Abort()
}

func (c *RequestCtx) Abort() {
	c.Special = -1
}

func (c *RequestCtx) SetEnableTrace(b bool) {
	c.enableTrace = b
}
