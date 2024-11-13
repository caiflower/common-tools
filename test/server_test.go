package test

import (
	"fmt"
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/caiflower/common-tools/pkg/tools"
	testv1 "github.com/caiflower/common-tools/test/controller/v1/test"
	"github.com/caiflower/common-tools/web/e"
	"github.com/caiflower/common-tools/web/v1"
)

type testInterceptor1 struct {
}

func (t *testInterceptor1) BeforeCallTargetMethod(w http.ResponseWriter, r *http.Request, ctx *v1.RequestCtx) e.ApiError {
	fmt.Println("BeforeCallTargetMethod order:1")
	return nil
}

func (t *testInterceptor1) AfterCallTargetMethod(w http.ResponseWriter, r *http.Request, ctx *v1.RequestCtx) e.ApiError {
	fmt.Println("AfterCallTargetMethod order:1")
	return nil
}

type testInterceptor2 struct {
}

func (t *testInterceptor2) BeforeCallTargetMethod(w http.ResponseWriter, r *http.Request, ctx *v1.RequestCtx) e.ApiError {
	fmt.Println("BeforeCallTargetMethod order:2")
	return nil
}

func (t *testInterceptor2) AfterCallTargetMethod(w http.ResponseWriter, r *http.Request, ctx *v1.RequestCtx) e.ApiError {
	fmt.Println("AfterCallTargetMethod order:2")
	return nil
}

func TestHttpServer(t *testing.T) {
	config := v1.Config{}
	err := tools.DoTagFunc(&config, nil, []func(reflect.StructField, reflect.Value, interface{}) error{tools.SetDefaultValueIfNil})
	if err != nil {
		panic(err)
	}

	server := v1.NewHttpServer(config)
	server.AddController(&testv1.StructService{})
	server.Register(v1.NewRestFul().Method(http.MethodPost).Version("v1").Controller("test.StructService").Path("/tests/{testId}").Action("Test1"))
	server.Register(v1.NewRestFul().Method(http.MethodGet).Version("v1").Controller("test.StructService").Path("/tests/{testId}").Action("Test1"))
	server.Register(v1.NewRestFul().Method(http.MethodPost).Version("v1").Controller("test.StructService").Path("/tests/{testId}/test2s/{test2Id}").Action("Test2"))
	server.Register(v1.NewRestFul().Method(http.MethodPost).Version("v1").Controller("test.StructService").Path("/tests/{testId}/test2s/{test2Id}/test").Action("Test2"))
	server.AddInterceptor(&testInterceptor2{}, 2)
	server.AddInterceptor(&testInterceptor1{}, 1)
	server.StartUp()

	time.Sleep(1 * time.Hour)
}
