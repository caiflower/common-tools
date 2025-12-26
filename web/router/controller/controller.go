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

package controller

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/caiflower/common-tools/pkg/basic"
	"github.com/caiflower/common-tools/pkg/tools"
	"github.com/caiflower/common-tools/web/common/reflectx"
	"github.com/caiflower/common-tools/web/router/method"
	"google.golang.org/grpc"
)

type Controller struct {
	paths []string
	*basic.Class
	grpcServiceDesc *grpc.ServiceDesc
	srv             interface{}
}

func NewController(v interface{}, controllerRootPkgName, rootPath string) (*Controller, error) {
	kind := reflect.TypeOf(v).Kind()
	switch kind {
	case reflect.Ptr, reflect.Interface:
		cls := basic.NewClass(v)
		path := cls.GetPath()
		if strings.Contains(path, controllerRootPkgName) {
			path = strings.Split(path, controllerRootPkgName)[1]
			if len(path) > 0 && strings.HasPrefix(path, "/") {
				path = path[1:]
			}
		} else {
			splits := strings.Split(path, "/")
			path = splits[len(splits)-1]
		}

		var paths []string
		paths = append(paths, cls.GetName())

		clsName := strings.Replace(cls.GetName(), ".", "/", 1)
		paths = append(paths, path+"/"+clsName)

		lowerClsName := strings.ToLower(clsName)
		paths = append(paths, path+"/"+lowerClsName)

		if strings.HasSuffix(lowerClsName, "service") {
			paths = append(paths, path+"/"+strings.Replace(lowerClsName, "service", "", 1))
		}

		if strings.HasSuffix(lowerClsName, "controller") {
			paths = append(paths, path+"/"+strings.Replace(lowerClsName, "controller", "", 1))
		}

		if rootPath != "" {
			if !strings.HasPrefix(rootPath, "/") {
				rootPath = "/" + rootPath
			}
		}
		for i, _ := range paths {
			if i == 0 {
				continue
			}
			if !strings.HasPrefix(paths[i], "/") {
				paths[i] = "/" + paths[i]
			}
			if rootPath != "" {
				paths[i] = rootPath + paths[i]
			}
		}

		for _, m := range cls.GetAllMethod() {
			if m.HasArgs() {
				arg := m.GetArgs()[0]
				var argValue reflect.Value
				switch arg.Kind() {
				case reflect.Ptr:
					argValue = reflect.New(arg.Elem())
				case reflect.Struct:
					argValue = reflect.New(arg)
				case reflect.Interface:
					continue
				default:
					panic(fmt.Sprintf("parse param failed. not support kind %s", arg.Kind()))
				}
				elem := reflect.TypeOf(argValue.Interface()).Elem()
				pkgPath := elem.PkgPath() + "." + elem.Name()
				if err := tools.DoTagFunc(argValue.Interface(), []tools.FnObj{
					{Fn: reflectx.BuildValid, Data: pkgPath},
				}); err != nil {
					panic(err.Error())
				}
			}
		}

		return &Controller{paths: paths, Class: cls}, nil
	default:
		return nil, fmt.Errorf("invalid type %s", kind)
	}
}

func (c *Controller) GetCls() *basic.Class {
	return c.Class
}

func (c *Controller) GetPaths() []string {
	return c.paths
}

func (c *Controller) GetTargetMethod(action string) *basic.Method {
	return c.GetMethod(c.GetPkgName() + "." + action)
}

func (c *Controller) SetGrpcService(srvDesc *grpc.ServiceDesc, srv interface{}) {
	c.grpcServiceDesc = srvDesc
	c.srv = srv
}

func (c *Controller) GetGrpcService() (*grpc.ServiceDesc, interface{}) {
	return c.grpcServiceDesc, c.srv
}

func (c *Controller) GetGrpcMethodDesc(methodName string) (*grpc.MethodDesc, interface{}, *basic.Method) {
	if c.grpcServiceDesc == nil {
		return nil, nil, nil
	}

	for _, v := range c.grpcServiceDesc.Methods {
		if v.MethodName == methodName {
			return &v, c.srv, c.GetTargetMethod(methodName)
		}
	}

	return nil, nil, nil
}

func (c *Controller) GetMethodDesc(action string) *method.Method {
	targetMethod := c.GetMethod(c.GetPkgName() + "." + action)
	if c.grpcServiceDesc == nil {
		if targetMethod == nil {
			return nil
		}

		return method.NewDefaultTypeMethod(targetMethod)
	}

	var m *grpc.MethodDesc
	for _, v := range c.grpcServiceDesc.Methods {
		if v.MethodName == action {
			m = &v
			break
		}
	}

	if m == nil {
		return nil
	}

	return method.NewGrpcTypeMethod(m, c.srv, targetMethod)
}
