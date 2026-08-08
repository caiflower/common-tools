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

package router

import (
	"context"
	"reflect"
	"testing"

	"github.com/caiflower/common-tools/pkg/basic"
	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/caiflower/common-tools/web/app"
	"github.com/caiflower/common-tools/web/router/method"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

type cliTestReq struct {
	ID   int    `path:"id" verf:"required|between:1,1000"`
	Name string `query:"name"`
	Auth string `header:"X-Auth"`
	Note string `json:"note"`
}

type cliTestResp struct{}

func cliTestGet(req *cliTestReq) (*cliTestResp, error) {
	return &cliTestResp{}, nil
}

func cliTestPost(req *cliTestReq) (*cliTestResp, error) {
	return &cliTestResp{}, nil
}

func cliTestCtxFirstGet(_ *app.RequestCtx, req *cliTestReq) (*cliTestResp, error) {
	return &cliTestResp{}, nil
}

type cliGrpcService struct{}

func (s *cliGrpcService) Get(_ context.Context, req *cliTestReq) (*cliTestResp, error) {
	return &cliTestResp{}, nil
}

func TestDeriveVerb(t *testing.T) {
	tests := []struct {
		method string
		want   string
	}{
		{method: "GET", want: "get"},
		{method: "POST", want: "create"},
		{method: "PUT", want: "update"},
		{method: "PATCH", want: "patch"},
		{method: "DELETE", want: "delete"},
		{method: "HEAD", want: "call"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, deriveVerb(tt.method), "method %s", tt.method)
	}
}

func TestDeriveResourceFromName(t *testing.T) {
	assert.Equal(t, "user", deriveResourceFromName("GetUser"))
	assert.Equal(t, "order", deriveResourceFromName("CreateOrder"))
	assert.Equal(t, "", deriveResourceFromName("anonymous"))
}

func TestDeriveResourceFromPath(t *testing.T) {
	assert.Equal(t, "users", deriveResourceFromPath("/api/v1/users/:id"))
	assert.Equal(t, "orders", deriveResourceFromPath("/users/:id/orders/:orderId"))
	assert.Equal(t, "search", deriveResourceFromPath("/users/search"))
}

func TestExtractParams(t *testing.T) {
	target := basic.NewMethod(nil, cliTestGet)
	m := method.NewDefaultTypeMethod(target)

	params := extractParams(m, "GET")
	assert.Len(t, params, 4)

	byName := make(map[string]ParamInfo)
	for _, p := range params {
		byName[p.Name] = p
	}

	assert.Equal(t, "path", byName["id"].Source)
	assert.True(t, byName["id"].Required)
	assert.Equal(t, "int", byName["id"].Type)
	assert.Equal(t, "query", byName["name"].Source)
	assert.Equal(t, "header", byName["X-Auth"].Source)
	assert.Equal(t, "query", byName["note"].Source)

	postParams := extractParams(method.NewDefaultTypeMethod(basic.NewMethod(nil, cliTestPost)), "POST")
	postByName := make(map[string]ParamInfo)
	for _, p := range postParams {
		postByName[p.Name] = p
	}
	assert.Equal(t, "body", postByName["note"].Source)
}

func TestExtractParamsCtxFirstHandler(t *testing.T) {
	target := basic.NewMethod(nil, cliTestCtxFirstGet)
	params := extractParams(method.NewDefaultTypeMethod(target), "GET")

	byName := make(map[string]ParamInfo)
	for _, p := range params {
		byName[p.Name] = p
	}
	assert.Equal(t, "path", byName["id"].Source)
	assert.Equal(t, "query", byName["name"].Source)
	assert.Equal(t, "query", byName["note"].Source)
}

func TestExtractParamsGrpcHandler(t *testing.T) {
	service := &cliGrpcService{}
	cls := basic.NewClass(service)
	methodValue, _ := reflect.TypeOf(service).MethodByName("Get")
	target := basic.NewMethod(cls, methodValue)
	m := method.NewGrpcTypeMethod(&grpc.MethodDesc{MethodName: "Get"}, service, target)

	params := extractParams(m, "GET")
	byName := make(map[string]ParamInfo)
	for _, p := range params {
		byName[p.Name] = p
	}
	assert.Equal(t, "path", byName["id"].Source)
	assert.Equal(t, "query", byName["name"].Source)
}

func TestRoutesAndOverride(t *testing.T) {
	handler := NewHandler(HandlerCfg{
		Name:                   "cli-test",
		DisableOptimization:    true,
		EnableActionController: false,
	}, logger.DefaultLogger())
	group := NewRouterGroup(handler)
	group.GET("/users/:id", cliTestGet)

	routes := handler.Routes()
	assert.Len(t, routes, 1)
	route := routes[0]
	assert.Equal(t, "GET", route.Method)
	assert.Equal(t, "/users/:id", route.Path)
	assert.Equal(t, "cliTestGet", route.OperationID)
	assert.Equal(t, "get", route.Verb)
	assert.Equal(t, "users", route.Resource)
	assert.Len(t, route.Params, 4)

	handler.CLIRoute("GET", "/users/:id", "customers", "list")
	route = handler.Routes()[0]
	assert.Equal(t, "customers", route.Resource)
	assert.Equal(t, "list", route.Verb)
}

func TestPathParamsFilteredByRouteTemplate(t *testing.T) {
	handler := NewHandler(HandlerCfg{
		Name:                   "cli-test",
		DisableOptimization:    true,
		EnableActionController: false,
	}, logger.DefaultLogger())
	group := NewRouterGroup(handler)
	group.GET("/users", cliTestGet)

	routes := handler.Routes()
	require.Len(t, routes, 1)
	for _, param := range routes[0].Params {
		assert.NotEqual(t, "path", param.Source)
	}
}
