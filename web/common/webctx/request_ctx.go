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
	"time"

	"github.com/caiflower/common-tools/pkg/basic"
	"github.com/caiflower/common-tools/web/network"
	netpoll1 "github.com/caiflower/common-tools/web/network/netpoll"
	"github.com/caiflower/common-tools/web/protocol"
	"github.com/cloudwego/netpoll"
)

type RequestCtx struct {
	ctx context.Context

	// net
	Request  *http.Request
	Response interface{}
	//StatusCode int
	Writer http.ResponseWriter

	// netx
	XRequest  *protocol.Request
	XResponse *protocol.Response
	conn      network.Conn

	Method  string
	Params  map[string][]string
	Path    string
	Version string
	Paths   map[string]string
	Action  string
	Restful bool

	Args         []reflect.Value
	TargetMethod *basic.Method
	Special      string
}

func (c *RequestCtx) ConvertToWebCtx() *Context {
	return &Context{RequestContext: c, Attributes: make(map[string]interface{})}
}

func (c *RequestCtx) SetResponse(v interface{}) {
	c.Response = v
}

func (c *RequestCtx) IsFinish() bool {
	return c.Response != nil
}

func (c *RequestCtx) GetPath() string {
	return c.Path
}

func (c *RequestCtx) GetParams() map[string][]string {
	return c.Params
}

func (c *RequestCtx) GetMethod() string {
	return c.Method
}

func (c *RequestCtx) GetAction() string {
	return c.Action
}

func (c *RequestCtx) GetVersion() string {
	return c.Version
}

func (c *RequestCtx) GetResponseWriterAndRequest() (http.ResponseWriter, *http.Request) {
	return c.Writer, c.Request
}

func (c *RequestCtx) GetResponse() interface{} {
	return c.Response
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
	c.Args = nil
	c.Special = ""
	c.TargetMethod = nil
	c.Response = nil
	c.Paths = make(map[string]string)
	c.Version = ""
	c.Action = ""
	c.Restful = false
	c.Response = nil
	if c.XRequest != nil {
		c.XRequest.Reset()
		c.XResponse.Reset()
	}
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

func (c *RequestCtx) GetReader() network.Reader {
	return c.conn
}

func (c *RequestCtx) GetWriter() network.Writer {
	return c.conn
}

func (c *RequestCtx) GetHttpRequest() (req *http.Request, err error) {
	conn := c.conn.(*netpoll1.Conn)
	reader := netpoll.NewIOReader(conn.Conn.(netpoll.Connection).Reader())
	bufReader := bufio.NewReaderSize(reader, 256)

	for {
		req, err = http.ReadRequest(bufReader)
		if err != nil {
			time.Sleep(time.Microsecond * 100)
			continue
		} else {
			break
		}
	}

	return req, err
}
