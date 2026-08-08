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
	"context"

	"github.com/caiflower/common-tools/web/app"
	"github.com/caiflower/common-tools/web/app/server"
	"github.com/caiflower/common-tools/web/app/server/config"
	"github.com/caiflower/common-tools/web/app/server/net"
	"github.com/caiflower/common-tools/web/app/server/netpoll"
	"github.com/caiflower/common-tools/web/router"
	"google.golang.org/grpc"
)

type Engine struct {
	server.Core
	opts        *config.Options
	routerGroup *router.RouterGroup
	handler     *router.Handler
}

func Default(opts ...config.Option) *Engine {
	options := config.NewOptions(opts...)
	engine := &Engine{
		opts: options,
	}

	// 根据模式创建对应的服务器
	switch options.Mode {
	case config.ServerModeStandard:
		engine.Core = engine.createStandardServer()
	case config.ServerModeNetpoll:
		fallthrough
	default:
		engine.Core = engine.createNetpollServer()
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
	s := net.NewHttpServer(standardConfig)
	e.handler = s.Handler
	e.routerGroup = router.NewRouterGroup(e.handler)
	return s
}

func (e *Engine) createNetpollServer() server.Core {
	s := netpoll.NewHttpServer(*e.opts)
	e.handler = s.Handler
	e.routerGroup = router.NewRouterGroup(e.handler)
	return s
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
		EnableMetrics:                 options.EnableMetrics,
		DisableOptimization:           true,
		EnableActionController:        options.EnableActionController,
		EnableSwagger:                 options.EnableSwagger,
		EnableCLI:                     options.EnableCLI,
		CLIRoutesPath:                 options.CLIRoutesPath,
		DisableHeaderNamesNormalizing: options.DisableHeaderNamesNormalizing,
	}
}

// Handler returns the underlying Handler for advanced usage.
func (e *Engine) Handler() *router.Handler {
	return e.handler
}

// Group creates a new router group with the given path prefix and optional middleware.
func (e *Engine) Group(relativePath string, handlers ...app.HandlerFunc) *router.RouterGroup {
	return e.routerGroup.Group(relativePath, handlers...)
}

// Use adds middleware to the root router group.
func (e *Engine) Use(middleware ...app.HandlerFunc) router.IRoutes {
	return e.routerGroup.Use(middleware...)
}

// GET registers a GET route with auto-detected handler type.
func (e *Engine) GET(relativePath string, handlers ...interface{}) router.IRoutes {
	return e.routerGroup.GET(relativePath, handlers...)
}

// POST registers a POST route with auto-detected handler type.
func (e *Engine) POST(relativePath string, handlers ...interface{}) router.IRoutes {
	return e.routerGroup.POST(relativePath, handlers...)
}

// PUT registers a PUT route with auto-detected handler type.
func (e *Engine) PUT(relativePath string, handlers ...interface{}) router.IRoutes {
	return e.routerGroup.PUT(relativePath, handlers...)
}

// DELETE registers a DELETE route with auto-detected handler type.
func (e *Engine) DELETE(relativePath string, handlers ...interface{}) router.IRoutes {
	return e.routerGroup.DELETE(relativePath, handlers...)
}

// PATCH registers a PATCH route with auto-detected handler type.
func (e *Engine) PATCH(relativePath string, handlers ...interface{}) router.IRoutes {
	return e.routerGroup.PATCH(relativePath, handlers...)
}

// OPTIONS registers an OPTIONS route with auto-detected handler type.
func (e *Engine) OPTIONS(relativePath string, handlers ...interface{}) router.IRoutes {
	return e.routerGroup.OPTIONS(relativePath, handlers...)
}

// HEAD registers a HEAD route with auto-detected handler type.
func (e *Engine) HEAD(relativePath string, handlers ...interface{}) router.IRoutes {
	return e.routerGroup.HEAD(relativePath, handlers...)
}

// StaticFile registers a single route to serve a single file from the local filesystem.
func (e *Engine) StaticFile(relativePath, filepath string) router.IRoutes {
	return e.routerGroup.StaticFile(relativePath, filepath)
}

// Static serves files from the given file system root.
func (e *Engine) Static(relativePath, root string) router.IRoutes {
	return e.routerGroup.Static(relativePath, root)
}

// StaticFS works just like Static() but a custom FS can be used instead.
func (e *Engine) StaticFS(relativePath string, fs *app.FS) router.IRoutes {
	return e.routerGroup.StaticFS(relativePath, fs)
}

// Any registers a route for all HTTP methods with auto-detected handler type.
func (e *Engine) Any(relativePath string, handlers ...interface{}) router.IRoutes {
	return e.routerGroup.Any(relativePath, handlers...)
}

// Handle registers a route with a custom HTTP method.
func (e *Engine) Handle(httpMethod, relativePath string, handlers ...interface{}) router.IRoutes {
	return e.routerGroup.Handle(httpMethod, relativePath, handlers...)
}

// GRPC registers a gRPC route with the given HTTP method, protoc-generated handler and service instance.
func (e *Engine) GRPC(httpMethod string, relativePath string, handler func(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error), srv interface{}) router.IRoutes {
	return e.routerGroup.GRPC(httpMethod, relativePath, handler, srv)
}

// RouterGroup returns the root RouterGroup.
func (e *Engine) RouterGroup() *router.RouterGroup {
	return e.routerGroup
}

// CLIRoute overrides the resource and verb used by the CLI for a registered route.
func (e *Engine) CLIRoute(method, path, resource, verb string) {
	e.handler.CLIRoute(method, path, resource, verb)
}
