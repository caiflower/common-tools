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
	"bufio"
	"context"
	"net/http"
	"reflect"

	"github.com/caiflower/common-tools/pkg/basic"
	"github.com/caiflower/common-tools/web/common/bytesconv"
	"github.com/caiflower/common-tools/web/network"
	netpoll1 "github.com/caiflower/common-tools/web/network/netpoll"
	"github.com/caiflower/common-tools/web/protocol"
	"github.com/caiflower/common-tools/web/router/param"
	"github.com/cloudwego/netpoll"
)

type RequestCtx struct {
	ctx context.Context

	// net
	Request *http.Request
	Writer  http.ResponseWriter

	// netpoll
	HttpResponse protocol.Response
	HttpRequest  protocol.Request
	conn         network.Conn

	isFinish bool
	Action   string
	data     interface{}

	method  []byte
	Restful bool

	path  []byte
	Paths param.Params

	Args         []reflect.Value
	TargetMethod *basic.Method
	Special      string
}

func (c *RequestCtx) ConvertToWebCtx() *Context {
	return &Context{RequestContext: c}
}

func (c *RequestCtx) SetHeader(key, value string) {
	if c.Writer != nil {
		c.Writer.Header().Set(key, value)
		return
	}

	c.HttpResponse.Header.Set(key, value)
}

func (c *RequestCtx) WriteHeader(statusCode int) {
	if c.Writer != nil {
		c.Writer.WriteHeader(statusCode)
		return
	}

	c.HttpResponse.WriteHeader(statusCode)
}

func (c *RequestCtx) Write(bytes []byte) (int, error) {
	if c.Writer != nil {
		return c.Writer.Write(bytes)
	}

	return c.HttpResponse.Write(bytes)
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
	if c.Request != nil {
		return c.Request.URL.Query()
	}

	var params = make(map[string][]string)
	c.HttpRequest.URI().QueryArgs().VisitAll(func(key, value []byte) {
		params[bytesconv.B2s(key)] = append(params[bytesconv.B2s(key)], bytesconv.B2s(value))
	})

	return params
}

func (c *RequestCtx) SetAction() string {
	if c.Request != nil {
		return c.Request.URL.Query().Get("Action")
	}
	return bytesconv.B2s(c.HttpRequest.URI().QueryArgs().Peek("Action"))
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

func (c *RequestCtx) GetResponseWriterAndRequest() (http.ResponseWriter, *http.Request) {
	return c.Writer, c.Request
}

func (c *RequestCtx) UpgradeWebsocket() {
	c.Special = "websocket"
}

func (c *RequestCtx) IsSpecial() bool {
	return c.Special != ""
}

func (c *RequestCtx) IsRestful() bool {
	return c.Restful
}

func (c *RequestCtx) Reset() {
	c.TargetMethod = nil
	c.Args = c.Args[:0]
	c.Special = ""
	c.Paths = c.Paths[:0]
	c.isFinish = false
	c.Writer = nil
	c.Request = nil
	c.HttpResponse.Reset()
	c.HttpRequest.Reset()
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

func (c *RequestCtx) GetHttpRequest() (req *http.Request, err error) {
	conn := c.conn.(*netpoll1.Conn)
	reader := netpoll.NewIOReader(conn.Conn.(netpoll.Connection).Reader())
	bufReader := bufio.NewReaderSize(reader, 128)

	req, err = http.ReadRequest(bufReader)
	if err != nil {
		return
	}

	req = req.WithContext(c.ctx)
	return
}

func (c *RequestCtx) GetContentEncoding() string {
	if c.Request != nil {
		return c.Request.Header.Get("Content-Encoding")
	}

	return bytesconv.B2s(c.HttpRequest.Header.ContentType())
}

func (c *RequestCtx) GetAcceptEncoding() string {
	if c.Request != nil {
		return c.Request.Header.Get("Accept-Encoding")
	}

	return c.HttpRequest.Header.Get("Accept-Encoding")
}

func (c *RequestCtx) GetContentLength() int64 {
	if c.Request != nil {
		return c.Request.ContentLength
	}

	return int64(c.HttpRequest.Header.ContentLength())
}

func (c *RequestCtx) HeaderGet(key string) string {
	if c.Request != nil {
		return c.Request.Header.Get(key)
	}
	return c.HttpRequest.Header.Get(key)
}
