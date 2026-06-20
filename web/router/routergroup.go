/*
 * Copyright 2022 CloudWeGo Authors
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
 * The MIT License (MIT)
 *
 * Copyright (c) 2014 Manuel Martínez-Almeida
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy
 * of this software and associated documentation files (the "Software"), to deal
 * in the Software without restriction, including without limitation the rights
 * to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 * copies of the Software, and to permit persons to whom the Software is
 * furnished to do so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in
 * all copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
 * AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 * LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 * OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
 * THE SOFTWARE.
 *
 * This file may have been modified by CloudWeGo authors. All CloudWeGo
 * Modifications are Copyright 2022 CloudWeGo Authors
 */

package router

import (
	"context"
	"math"
	"path"
	"reflect"
	"regexp"
	"runtime"

	"github.com/caiflower/common-tools/pkg/basic"
	"github.com/caiflower/common-tools/pkg/tools"
	"github.com/caiflower/common-tools/web/app"
	"github.com/caiflower/common-tools/web/protocol/consts"
	"github.com/caiflower/common-tools/web/router/method"
	"google.golang.org/grpc"
)

// RouteRegistrar defines the interface for adding routes to the method tree.
// Handler implements this interface, allowing RouterGroup to register routes
// without depending on a concrete Engine type.
type RouteRegistrar interface {
	addRoute(httpMethod string, path string, handlers HandlersChain)
}

// IRouter defines all router handle interface includes single and group router.
type IRouter interface {
	IRoutes
	Group(string, ...app.HandlerFunc) *RouterGroup
}

// IRoutes defines all router handle interface.
type IRoutes interface {
	Use(...app.HandlerFunc) IRoutes
	Handle(string, string, ...interface{}) IRoutes
	Any(string, ...interface{}) IRoutes
	GET(string, ...interface{}) IRoutes
	POST(string, ...interface{}) IRoutes
	DELETE(string, ...interface{}) IRoutes
	PATCH(string, ...interface{}) IRoutes
	PUT(string, ...interface{}) IRoutes
	OPTIONS(string, ...interface{}) IRoutes
	HEAD(string, ...interface{}) IRoutes
}

// RouterGroup is used internally to configure router, a RouterGroup is associated with
// a prefix and an array of handlers (middleware).
type RouterGroup struct {
	middleware app.HandlersChain
	basePath   string
	engine     RouteRegistrar
	root       bool
}

var _ IRouter = (*RouterGroup)(nil)

// NewRouterGroup creates a root RouterGroup with the given RouteRegistrar.
func NewRouterGroup(engine RouteRegistrar) *RouterGroup {
	return &RouterGroup{
		engine: engine,
		root:   true,
	}
}

// Use adds middleware to the group, see example code in GitHub.
func (group *RouterGroup) Use(middleware ...app.HandlerFunc) IRoutes {
	group.middleware = append(group.middleware, middleware...)
	return group.returnObj()
}

// Group creates a new router group. You should add all the routes that have common middlewares or the same path prefix.
// For example, all the routes that use a common middleware for authorization could be grouped.
func (group *RouterGroup) Group(relativePath string, handlers ...app.HandlerFunc) *RouterGroup {
	return &RouterGroup{
		middleware: group.combineMiddleware(handlers),
		basePath:   group.calculateAbsolutePath(relativePath),
		engine:     group.engine,
	}
}

// BasePath returns the base path of router group.
// For example, if v := router.Group("/rest/n/v1/api"), v.BasePath() is "/rest/n/v1/api".
func (group *RouterGroup) BasePath() string {
	return group.basePath
}

// handlerFuncType is the reflect.Type of app.HandlerFunc, used for signature matching.
var handlerFuncType = reflect.TypeOf((*app.HandlerFunc)(nil)).Elem()

// wrapHandler auto-detects the handler type and wraps it as method.Method.
// - app.HandlerFunc signature (func(context.Context, *RequestCtx)) → HandlerFuncTypeOfMethod (direct call, no parameter parsing)
// - method.Method   → use directly (preserving original MethodType)
// - other function  → basic.NewMethod(nil, handler) → DefaultTypeOfMethod (auto parameter parsing)
// - other           → panic
func wrapHandler(handler interface{}) method.Method {
	switch h := handler.(type) {
	case app.HandlerFunc:
		return *method.NewHandlerFuncTypeMethod(h)
	case method.Method:
		return h
	default:
		rv := reflect.ValueOf(handler)
		if rv.Kind() == reflect.Func {
			// Check if the function signature matches app.HandlerFunc
			if rv.Type().ConvertibleTo(handlerFuncType) {
				hf := rv.Convert(handlerFuncType).Interface().(app.HandlerFunc)
				return *method.NewHandlerFuncTypeMethod(hf)
			}
			targetMethod := basic.NewMethod(nil, handler)
			return *method.NewDefaultTypeMethod(targetMethod)
		}
		panic("unsupported handler type: " + reflect.TypeOf(handler).String())
	}
}

func (group *RouterGroup) handle(httpMethod, relativePath string, handlers ...interface{}) IRoutes {
	absolutePath := group.calculateAbsolutePath(relativePath)

	// Wrap handlers to method.Method and combine with group middleware
	methodHandlers := make(HandlersChain, 0, len(handlers)+len(group.middleware))
	// Add group middleware as HandlerFuncTypeOfMethod
	for _, mw := range group.middleware {
		methodHandlers = append(methodHandlers, *method.NewHandlerFuncTypeMethod(mw))
	}
	// Add route handlers
	for _, h := range handlers {
		methodHandlers = append(methodHandlers, wrapHandler(h))
	}

	group.engine.addRoute(httpMethod, absolutePath, methodHandlers)
	return group.returnObj()
}

var upperLetterReg = regexp.MustCompile("^[A-Z]+$")

// Handle registers a new request handle and middleware with the given path and method.
// The last handler should be the real handler, the other ones should be middleware that can and should be shared among different routes.
// See the example code in GitHub.
//
// For GET, POST, PUT, PATCH and DELETE requests the respective shortcut
// functions can be used.
//
// This function is intended for bulk loading and to allow the usage of less
// frequently used, non-standardized or custom methods (e.g. for internal
// communication with a proxy).
func (group *RouterGroup) Handle(httpMethod, relativePath string, handlers ...interface{}) IRoutes {
	if matches := upperLetterReg.MatchString(httpMethod); !matches {
		panic("http method " + httpMethod + " is not valid")
	}
	return group.handle(httpMethod, relativePath, handlers...)
}

// POST is a shortcut for router.Handle("POST", path, handle).
func (group *RouterGroup) POST(relativePath string, handlers ...interface{}) IRoutes {
	return group.handle(consts.MethodPost, relativePath, handlers...)
}

// GET is a shortcut for router.Handle("GET", path, handle).
func (group *RouterGroup) GET(relativePath string, handlers ...interface{}) IRoutes {
	return group.handle(consts.MethodGet, relativePath, handlers...)
}

// DELETE is a shortcut for router.Handle("DELETE", path, handle).
func (group *RouterGroup) DELETE(relativePath string, handlers ...interface{}) IRoutes {
	return group.handle(consts.MethodDelete, relativePath, handlers...)
}

// PATCH is a shortcut for router.Handle("PATCH", path, handle).
func (group *RouterGroup) PATCH(relativePath string, handlers ...interface{}) IRoutes {
	return group.handle(consts.MethodPatch, relativePath, handlers...)
}

// PUT is a shortcut for router.Handle("PUT", path, handle).
func (group *RouterGroup) PUT(relativePath string, handlers ...interface{}) IRoutes {
	return group.handle(consts.MethodPut, relativePath, handlers...)
}

// OPTIONS is a shortcut for router.Handle("OPTIONS", path, handle).
func (group *RouterGroup) OPTIONS(relativePath string, handlers ...interface{}) IRoutes {
	return group.handle(consts.MethodOptions, relativePath, handlers...)
}

// HEAD is a shortcut for router.Handle("HEAD", path, handle).
func (group *RouterGroup) HEAD(relativePath string, handlers ...interface{}) IRoutes {
	return group.handle(consts.MethodHead, relativePath, handlers...)
}

// Any registers a route that matches all the HTTP methods.
// GET, POST, PUT, PATCH, HEAD, OPTIONS, DELETE, CONNECT, TRACE.
func (group *RouterGroup) Any(relativePath string, handlers ...interface{}) IRoutes {
	group.handle(consts.MethodGet, relativePath, handlers...)
	group.handle(consts.MethodPost, relativePath, handlers...)
	group.handle(consts.MethodPut, relativePath, handlers...)
	group.handle(consts.MethodPatch, relativePath, handlers...)
	group.handle(consts.MethodHead, relativePath, handlers...)
	group.handle(consts.MethodOptions, relativePath, handlers...)
	group.handle(consts.MethodDelete, relativePath, handlers...)
	group.handle(consts.MethodConnect, relativePath, handlers...)
	group.handle(consts.MethodTrace, relativePath, handlers...)
	return group.returnObj()
}

// GRPC registers a gRPC route with the given HTTP method, protoc-generated handler and service instance.
// The handler is the function generated by protoc (e.g. _IService_Search_Handler).
// The srv is the service implementation instance.
// Internally, it extracts the method name from the handler function name and constructs
// a GrpcTypeOfMethod for non-reflective dispatch.
func (group *RouterGroup) GRPC(httpMethod string, relativePath string, handler func(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error), srv interface{}) IRoutes {
	absolutePath := group.calculateAbsolutePath(relativePath)

	// Extract method name from handler function name
	// e.g. "_IService_Search_Handler" → "Search"
	funcName := runtime.FuncForPC(reflect.ValueOf(handler).Pointer()).Name()
	methodName := extractGRPCMethodName(funcName)

	// Get targetMethod from srv via reflection
	srvValue := reflect.ValueOf(srv)
	if srvValue.Kind() == reflect.Ptr {
		srvValue = srvValue.Elem()
	}
	srvMethod := srvValue.MethodByName(methodName)
	if !srvMethod.IsValid() {
		// Try on pointer type
		srvMethod = reflect.ValueOf(srv).MethodByName(methodName)
	}
	if !srvMethod.IsValid() {
		panic("GRPC: method " + methodName + " not found on " + reflect.TypeOf(srv).String())
	}

	// Create targetMethod using basic.NewMethod
	cls := basic.NewClass(srv)
	srvReflectMethod, _ := reflect.TypeOf(srv).MethodByName(methodName)
	targetMethod := basic.NewMethod(cls, srvReflectMethod)

	// Construct grpc.MethodDesc
	methodDesc := &grpc.MethodDesc{
		MethodName: methodName,
		Handler:    handler,
	}

	// Create GrpcTypeOfMethod
	m := method.NewGrpcTypeMethod(methodDesc, srv, targetMethod)

	methodHandlers := make(HandlersChain, 0, 1+len(group.middleware))
	for _, mw := range group.middleware {
		methodHandlers = append(methodHandlers, *method.NewHandlerFuncTypeMethod(mw))
	}
	methodHandlers = append(methodHandlers, *m)
	group.engine.addRoute(httpMethod, absolutePath, methodHandlers)
	return group.returnObj()
}

// extractGRPCMethodName extracts the method name from a gRPC handler function name.
// Supports two naming patterns:
//  1. Protoc-generated: "proto._IService_Search_Handler" → "Search"
//  2. Wrapper functions: "proto.ExecutionServiceGetHandler" → "Get"
//     (wrapper naming convention: XxxServiceYyyHandler, where XxxService is the
//     service name ending with "Service" and Yyy is the method name)
func extractGRPCMethodName(funcName string) string {
	// Pattern 1: Protoc-generated handler with underscores
	// e.g. "_IService_Search_Handler" → capture "IService_Search" → last part = "Search"
	re := regexp.MustCompile(`_([^_]+)_Handler`)
	matches := re.FindStringSubmatch(funcName)
	if len(matches) >= 2 {
		parts := regexp.MustCompile(`_`).Split(matches[1], -1)
		return parts[len(parts)-1]
	}

	// Pattern 2: Wrapper functions ending with "Handler"
	// e.g. "ExecutionServiceGetHandler" → service="ExecutionService", method="Get"
	// The service name must end with "Service" (gRPC convention)
	reWrapper := regexp.MustCompile(`([A-Z][a-zA-Z]*Service)([A-Z][a-zA-Z]*)Handler`)
	wrapperMatches := reWrapper.FindStringSubmatch(funcName)
	if len(wrapperMatches) >= 3 {
		return wrapperMatches[2]
	}

	// Fallback: strip package path and type suffix
	name := tools.RegReplace(funcName, ".*/", "")
	name = tools.RegReplace(name, `\..*`, "")
	return name
}

func (group *RouterGroup) combineMiddleware(handlers app.HandlersChain) app.HandlersChain {
	finalSize := len(group.middleware) + len(handlers)
	if finalSize >= int(abortIndex) {
		panic("too many handlers")
	}
	mergedHandlers := make(app.HandlersChain, finalSize)
	copy(mergedHandlers, group.middleware)
	copy(mergedHandlers[len(group.middleware):], handlers)
	return mergedHandlers
}

func (group *RouterGroup) calculateAbsolutePath(relativePath string) string {
	return joinPaths(group.basePath, relativePath)
}

func (group *RouterGroup) returnObj() IRoutes {
	return group
}

// GETEX adds a handlerName param. When handler is decorated or handler is an anonymous function,
// Hertz cannot get handler name directly. In this case, pass handlerName explicitly.
func (group *RouterGroup) GETEX(relativePath string, handler app.HandlerFunc, handlerName string) IRoutes {
	app.SetHandlerName(handler, handlerName)
	return group.GET(relativePath, handler)
}

// POSTEX adds a handlerName param. When handler is decorated or handler is an anonymous function,
// Hertz cannot get handler name directly. In this case, pass handlerName explicitly.
func (group *RouterGroup) POSTEX(relativePath string, handler app.HandlerFunc, handlerName string) IRoutes {
	app.SetHandlerName(handler, handlerName)
	return group.POST(relativePath, handler)
}

// PUTEX adds a handlerName param. When handler is decorated or handler is an anonymous function,
// Hertz cannot get handler name directly. In this case, pass handlerName explicitly.
func (group *RouterGroup) PUTEX(relativePath string, handler app.HandlerFunc, handlerName string) IRoutes {
	app.SetHandlerName(handler, handlerName)
	return group.PUT(relativePath, handler)
}

// DELETEEX adds a handlerName param. When handler is decorated or handler is an anonymous function,
// Hertz cannot get handler name directly. In this case, pass handlerName explicitly.
func (group *RouterGroup) DELETEEX(relativePath string, handler app.HandlerFunc, handlerName string) IRoutes {
	app.SetHandlerName(handler, handlerName)
	return group.DELETE(relativePath, handler)
}

// HEADEX adds a handlerName param. When handler is decorated or handler is an anonymous function,
// Hertz cannot get handler name directly. In this case, pass handlerName explicitly.
func (group *RouterGroup) HEADEX(relativePath string, handler app.HandlerFunc, handlerName string) IRoutes {
	app.SetHandlerName(handler, handlerName)
	return group.HEAD(relativePath, handler)
}

// AnyEX adds a handlerName param. When handler is decorated or handler is an anonymous function,
// Hertz cannot get handler name directly. In this case, pass handlerName explicitly.
func (group *RouterGroup) AnyEX(relativePath string, handler app.HandlerFunc, handlerName string) IRoutes {
	app.SetHandlerName(handler, handlerName)
	return group.Any(relativePath, handler)
}

// HandleEX adds a handlerName param. When handler is decorated or handler is an anonymous function,
// Hertz cannot get handler name directly. In this case, pass handlerName explicitly.
func (group *RouterGroup) HandleEX(httpMethod, relativePath string, handler app.HandlerFunc, handlerName string) IRoutes {
	app.SetHandlerName(handler, handlerName)
	return group.Handle(httpMethod, relativePath, handler)
}

const abortIndex int8 = math.MaxInt8 / 2

func joinPaths(absolutePath, relativePath string) string {
	if relativePath == "" {
		return absolutePath
	}

	finalPath := path.Join(absolutePath, relativePath)
	appendSlash := lastChar(relativePath) == '/' && lastChar(finalPath) != '/'
	if appendSlash {
		return finalPath + "/"
	}
	return finalPath
}

func lastChar(str string) uint8 {
	if str == "" {
		panic("The length of the string can't be 0")
	}
	return str[len(str)-1]
}
