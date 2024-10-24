package global

import (
	"reflect"

	"github.com/caiflower/common-tools/pkg/tools"
)

func LoadDefaultConfig() {

}

func LoadWebConfig() {

}

func LoadDBConfig() {

}

func LoadRedisConfig() {

}

func LoadConfig(filename string, v interface{}) error {
	err := tools.UnmarshalFileYaml(filename, v)
	if err != nil {
		return err
	}

	tools.DoTagFunc(v, []func(reflect.StructField, reflect.Value){tools.SetDefaultValueIfNil})
	return nil
}
