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
	"github.com/caiflower/common-tools/web/common/cfg"
	"github.com/caiflower/common-tools/web/router"
	"github.com/caiflower/common-tools/web/server"
	"github.com/caiflower/common-tools/web/server/net"
)

// WebServer 统一服务器接口
type Engine struct {
	server.Core
	opts *cfg.Options
}

func Default(opts ...cfg.Option) *Engine {
	options := cfg.NewOptions(opts)
	engine := &Engine{
		opts: options,
	}

	// 根据模式创建对应的服务器
	switch options.Mode {
	case cfg.ServerModeNetpoll:
		//engine.Core = uws.createNetpollServer()
	case cfg.ServerModeStandard:
		fallthrough
	default:
		engine.Core = engine.createStandardServer()
	}

	return engine
}

// createStandardServer 创建标准服务器
func (e *Engine) createStandardServer() server.Core {
	options := e.opts

	standardConfig := net.NormalConfig{
		Port:          options.Port,
		ReadTimeout:   options.ReadTimeout,
		WriteTimeout:  options.WriteTimeout,
		HandleTimeout: options.HandleTimeout,
		HandlerCfg:    e.getHandlerCfg(),
	}
	return net.NewHttpServer(standardConfig)
}

func (e *Engine) getHandlerCfg() router.HandlerCfg {
	options := e.opts
	return router.HandlerCfg{
		Name:                  options.Name,
		RootPath:              options.RootPath,
		HeaderTraceID:         options.HeaderTraceID,
		ControllerRootPkgName: options.ControllerRootPkgName,
		EnablePprof:           options.EnablePprof,
		WebLimiter: router.LimiterConfig{
			Enable: options.LimiterEnabled,
			Qos:    options.Qps,
		},
	}
}
