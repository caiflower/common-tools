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

package web

import (
	"testing"

	"github.com/caiflower/common-tools/global"
	"github.com/caiflower/common-tools/web/app/server/config"
	"github.com/caiflower/common-tools/web/app/server/net"
	netpollserver "github.com/caiflower/common-tools/web/app/server/netpoll"
)

// Every HTTP server form, including the Engine that applications actually
// register with the resource manager, must expose the same close order.
var (
	_ global.DaemonResource    = (*Engine)(nil)
	_ global.ResourceWithOrder = (*Engine)(nil)
	_ global.ResourceWithOrder = (*net.HttpServer)(nil)
	_ global.ResourceWithOrder = (*netpollserver.HttpServer)(nil)
)

func TestEngineClosesFirstInBothModes(t *testing.T) {
	modes := []config.ServerMode{config.ServerModeStandard, config.ServerModeNetpoll}
	for _, mode := range modes {
		t.Run(string(mode), func(t *testing.T) {
			engine := Default(config.WithMode(mode))
			if got := engine.Order(); got != global.OrderHTTPServer {
				t.Fatalf("Engine.Order() = %d, want %d", got, global.OrderHTTPServer)
			}
		})
	}
}
