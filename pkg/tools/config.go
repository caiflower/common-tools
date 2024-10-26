package tools

import (
	"reflect"
)

func LoadConfig(filename string, v interface{}) error {
	err := UnmarshalFileYaml(filename, v)
	if err != nil {
		return err
	}

	if err = DoTagFunc(v, []func(reflect.StructField, reflect.Value) error{SetDefaultValueIfNil}); err != nil {
		return err
	}

	return nil
}
