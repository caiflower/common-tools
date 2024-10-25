package global

import (
	"reflect"

	"github.com/caiflower/common-tools/pkg/global/env"
	"github.com/caiflower/common-tools/pkg/tools"
)

func LoadDefaultConfig(v interface{}) (err error) {
	err = LoadConfig(env.ConfigPath+"/default.yaml", v)
	return
}

func LoadConfig(filename string, v interface{}) error {
	err := tools.UnmarshalFileYaml(filename, v)
	if err != nil {
		return err
	}

	if err = tools.DoTagFunc(v, []func(reflect.StructField, reflect.Value) error{tools.SetDefaultValueIfNil}); err != nil {
		return err
	}

	return nil
}
