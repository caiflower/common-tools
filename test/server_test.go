package test

import (
	"reflect"
	"testing"
	"time"

	"github.com/caiflower/common-tools/pkg/tools"
	testv1 "github.com/caiflower/common-tools/test/controller/v1/test"
	"github.com/caiflower/common-tools/web/v1"
)

func TestHttpServer(t *testing.T) {
	config := v1.Config{}
	err := tools.DoTagFunc(&config, nil, []func(reflect.StructField, reflect.Value, interface{}) error{tools.SetDefaultValueIfNil})
	if err != nil {
		panic(err)
	}

	server := v1.NewHttpServer(config)
	server.AddController(&testv1.StructService{})
	server.StartUp()

	time.Sleep(1 * time.Hour)
}
