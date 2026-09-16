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

package server

import "github.com/caiflower/common-tools/global"

// Daemon is embedded by every Core implementation to declare that an HTTP
// server must be closed first during graceful shutdown, before downstream
// resources (kafka/redis/db).
//
// Embedding it in a server implementation is enough to participate in graceful
// shutdown ordering; the rule lives here instead of being re-implemented by
// each launcher (standard / netpoll / Engine).
type Daemon struct{}

// Order implements global.ResourceWithOrder. Lower order values are closed
// earlier, so an HTTP server is always closed before the resources it depends on.
func (Daemon) Order() int {
	return global.OrderHTTPServer
}
