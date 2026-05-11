/*
 * Copyright 2025 caiflower Authors
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
 *
 * Copyright 2025 CloudWeGo Authors
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
 *
 * This file may have been modified by caiflower authors. All caiflower
 * Modifications are Copyright 2025 caiflower Authors.
 */

package sse

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/caiflower/common-tools/web"
	"github.com/caiflower/common-tools/web/app"
	"github.com/caiflower/common-tools/web/app/client"
	"github.com/caiflower/common-tools/web/app/server/config"
	"github.com/caiflower/common-tools/web/protocol"
	"github.com/caiflower/common-tools/web/router/controller"
)

type Request struct {
}

type sseController struct {
}

func (c *sseController) Dail(ctx *app.RequestContext) {

	ctx.Abort()
	println("Server Got LastEventID", GetLastEventID(&ctx.Request))
	w := NewWriter(ctx)
	for i := 0; i < 5; i++ {
		_ = w.WriteEvent(fmt.Sprintf("id-%d", i), "message", []byte("hello\n\nworld"))
		time.Sleep(10 * time.Millisecond)
	}
	// [optional] it writes 0\r\n\r\n to indicate the end of chunked response
	// hertz will do it after handler returns
	_ = w.Close()
}

// Example demonstrates a simple SSE server and client interaction.
func Example() {
	port := 8080
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	// --- SSE Server ---
	engine := web.Default(config.WithAddr(addr))
	sse := engine.AddController(&sseController{})
	engine.Register(controller.NewRestFul().Method(http.MethodGet).Path("/").RegisterMethod(sse.GetMethod("Dail")))
	go engine.Start()
	defer engine.Close()
	time.Sleep(20 * time.Millisecond) // wait for server to start

	// --- SSE Client ---
	c, _ := client.NewClient()
	req, resp := protocol.AcquireRequest(), protocol.AcquireResponse()
	req.SetRequestURI("http://" + addr + "/")
	req.SetMethod("GET")
	req.SetHeader(LastEventIDHeader, "id-0")

	// adds `text/event-stream` to client `Accept` header
	// may required for some Model Context Protocol(MCP) servers
	AddAcceptMIME(req)

	if err := c.Do(context.Background(), req, resp); err != nil {
		panic(err)
	}
	r, err := NewReader(resp)
	if err != nil {
		panic(err)
	}
	defer r.Close()

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(200 * time.Millisecond)
		// cancel can be used to force ForEach returns by closing the remote connection
		_ = cancel
	}()
	err = r.ForEach(ctx, func(e *Event) error {
		println("Event:", e.String())
		return nil
	})
	if err != nil {
		panic(err)
	}
	println("Client LastEventID", r.LastEventID())
	// Output:
	//
}
