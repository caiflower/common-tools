package tools

import (
	"reflect"
)

func LoadConfig(filename string, v interface{}) error {
	err := UnmarshalFileYaml(filename, v)
	if err != nil {
		return err
	}

	if err = DoTagFunc(v, nil, []func(reflect.StructField, reflect.Value, interface{}) error{SetDefaultValueIfNil}); err != nil {
		return err
	}

	return nil
}
