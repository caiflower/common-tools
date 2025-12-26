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
