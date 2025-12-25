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

import (
	"github.com/caiflower/common-tools/web/common/interceptor"
	"github.com/caiflower/common-tools/web/protocol"
	"github.com/caiflower/common-tools/web/router"
	controller2 "github.com/caiflower/common-tools/web/router/controller"
)

type Core interface {
	Name() string
	Start() error
	Close()

	AddController(v interface{}) *controller2.Controller
	Register(controller *controller2.RestfulController)

	AddInterceptor(i interceptor.Interceptor, order int)
	SetBeforeDispatchCallBack(callbackFunc router.BeforeDispatchCallbackFunc)

	AddProtocol(protocol string, core protocol.Server)
}
