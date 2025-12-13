package server

import (
	"github.com/caiflower/common-tools/web/common/controller"
	"github.com/caiflower/common-tools/web/common/interceptor"
	"github.com/caiflower/common-tools/web/router"
)

type Core interface {
	Name() string
	Start() error
	Close()

	AddController(v interface{})
	Register(controller *controller.RestfulController)

	AddInterceptor(i interceptor.Interceptor, order int)
	SetBeforeDispatchCallBack(callbackFunc router.BeforeDispatchCallbackFunc)
}
