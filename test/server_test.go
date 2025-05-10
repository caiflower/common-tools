package test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/caiflower/common-tools/pkg/logger"
	testv1 "github.com/caiflower/common-tools/test/controller/v1/test"
	"github.com/caiflower/common-tools/web"
	"github.com/caiflower/common-tools/web/e"
	"github.com/caiflower/common-tools/web/v1"
)

type TestInterceptor1 struct {
}

func (t *TestInterceptor1) Before(ctx *web.Context) (err e.ApiError) {
	fmt.Println("BeforeCallTargetMethod order:1")
	return
}

func (t *TestInterceptor1) After(ctx *web.Context, err e.ApiError) e.ApiError {
	fmt.Println("AfterCallTargetMethod order:1")
	if err != nil {
		fmt.Printf("AfterCallTargetMethod order:1 err: %v\n", err.GetMessage())
	}
	return nil
}

func (t *TestInterceptor1) OnPanic(ctx *web.Context) (err e.ApiError) {
	fmt.Println("OnPanic order:1")
	return e.NewApiError(e.Unknown, "请稍后再试", nil)
}

type TestInterceptor2 struct {
}

func (t *TestInterceptor2) Before(ctx *web.Context) (err e.ApiError) {
	fmt.Println("BeforeCallTargetMethod order:2")
	return
}

func (t *TestInterceptor2) After(ctx *web.Context, err e.ApiError) e.ApiError {
	fmt.Println("AfterCallTargetMethod order:2")
	if err != nil {
		fmt.Printf("AfterCallTargetMethod order:2 err: %v\n", err.GetMessage())
	}
	return nil
}

func (t *TestInterceptor2) OnPanic(ctx *web.Context) (err e.ApiError) {
	fmt.Println("OnPanic order:2")
	return
}

func TestHttpServer(t *testing.T) {
	logger.InitLogger(&logger.Config{
		EnableColor: "True",
	})
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		fmt.Println("加载时区出错:", err)
		return
	}
	// 设置time.Local为中国标准时间
	time.Local = loc
	config := webv1.Config{
		RootPath: "testhttp",
	}

	server := webv1.NewHttpServer(config)
	server.AddController(&testv1.StructService{})
	server.Register(webv1.NewRestFul().Method(http.MethodPost).Version("v1").Controller("test.StructService").Path("/tests/{testId}").Action("Test1"))
	server.Register(webv1.NewRestFul().Method(http.MethodGet).Version("v1").Controller("test.StructService").Path("/tests/{testId}").Action("Test1"))
	server.Register(webv1.NewRestFul().Method(http.MethodPost).Version("v1").Controller("test.StructService").Path("/tests/{testId}/test2s/{test2Id}").Action("Test2"))
	server.Register(webv1.NewRestFul().Method(http.MethodPost).Version("v1").Controller("test.StructService").Path("/tests/{testId}/test2s/{test2Id}/test").Action("Test2"))
	server.AddInterceptor(&TestInterceptor2{}, 2)
	server.AddInterceptor(&TestInterceptor1{}, 1)
	server.StartUp()

	time.Sleep(1 * time.Hour)
}
