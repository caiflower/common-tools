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
	"github.com/caiflower/common-tools/web/common/config"
	"github.com/caiflower/common-tools/web/router"
	"github.com/caiflower/common-tools/web/server"
	"github.com/caiflower/common-tools/web/server/net"
	"github.com/caiflower/common-tools/web/server/netx"
)

type Engine struct {
	server.Core
	opts *config.Options
}

func Default(opts ...config.Option) *Engine {
	options := config.NewOptions(opts)
	engine := &Engine{
		opts: options,
	}

	// 根据模式创建对应的服务器
	switch options.Mode {
	case config.ServerModeNetpoll:
		engine.Core = engine.createNetxServer()
	case config.ServerModeStandard:
		fallthrough
	default:
		engine.Core = engine.createStandardServer()
	}

	return engine
}

func (e *Engine) createStandardServer() server.Core {
	options := e.opts

	standardConfig := net.NormalConfig{
		Addr:          options.Addr,
		ReadTimeout:   options.ReadTimeout,
		WriteTimeout:  options.WriteTimeout,
		HandleTimeout: options.HandleTimeout,
		HandlerCfg:    e.getHandlerCfg(),
	}
	return net.NewHttpServer(standardConfig)
}

func (e *Engine) createNetxServer() server.Core {
	return netx.NewHttpServer(*e.opts)
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
