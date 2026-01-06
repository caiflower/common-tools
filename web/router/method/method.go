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

package method

import (
	"github.com/caiflower/common-tools/pkg/basic"
	"google.golang.org/grpc"
)

type MethodType uint8

const (
	DefaultTypeOfMethod = iota
	GrpcTypeOfMethod
)

type Method struct {
	targetMethod *basic.Method
	methodDesc   *grpc.MethodDesc
	srv          interface{}
	t            MethodType
}

func NewDefaultTypeMethod(method *basic.Method) *Method {
	return &Method{
		targetMethod: method,
	}
}

func NewGrpcTypeMethod(methodDesc *grpc.MethodDesc, srv interface{}, targetMethod *basic.Method) *Method {
	return &Method{
		targetMethod: targetMethod,
		methodDesc:   methodDesc,
		srv:          srv,
		t:            GrpcTypeOfMethod,
	}
}

func (m *Method) GetType() MethodType {
	return m.t
}

func (m *Method) GetAction() string {
	switch m.t {
	case DefaultTypeOfMethod:
		return m.targetMethod.GetName()
	case GrpcTypeOfMethod:
		return m.methodDesc.MethodName
	default:
		return ""
	}
}

func (m *Method) GetInfo() (MethodType, *basic.Method, *grpc.MethodDesc, interface{}) {
	return m.t, m.targetMethod, m.methodDesc, m.srv
}

func (m *Method) HasArgs() bool {
	return m.targetMethod.HasArgs()
}

func (m *Method) GetTargetMethod() *basic.Method {
	return m.targetMethod
}
