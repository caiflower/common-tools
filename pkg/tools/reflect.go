package tools

import (
	"fmt"
	"reflect"
	"regexp"
	"strconv"

	"github.com/modern-go/reflect2"
)

func DoTagFunc(v interface{}, fn []func(reflect.StructField, reflect.Value)) {
	if reflect2.IsNil(v) {
		return
	}

	// default tag处理
	vType := reflect2.TypeOf(v)
	vType1 := vType.Type1()

	switch vType1.Kind() {
	case reflect.Interface, reflect.Ptr:
	default:
		// interface能生效
		return
	}

	indirect := reflect.Indirect(reflect.ValueOf(v))
	for i := 0; i < indirect.NumField(); i++ {
		field := indirect.Field(i)
		fieldStruct := vType1.Elem().Field(i)

		for _, f := range fn {
			f(fieldStruct, field)
		}
	}

}

func SetDefaultValueIfNil(structField reflect.StructField, vValue reflect.Value) {
	structTag := structField.Tag
	if containTag(structTag, "default") || vValue.Kind() == reflect.Struct || vValue.Kind() == reflect.Ptr {
		switch vValue.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32:
			if vValue.Int() == 0 {
				v, _ := strconv.Atoi(structTag.Get("default"))
				vValue.SetInt(int64(v))
			}
		case reflect.String:
			if vValue.String() == "" {
				vValue.SetString(structTag.Get("default"))
			}
		case reflect.Float32:
			if vValue.Float() == 0 {
				v, _ := strconv.ParseFloat(structTag.Get("default"), 32)
				vValue.SetFloat(v)
			}
		case reflect.Float64:
			if vValue.Float() == 0 {
				v, _ := strconv.ParseFloat(structTag.Get("default"), 64)
				vValue.SetFloat(v)
			}
		case reflect.Struct:
			t := structField.Type
			for i := 0; i < t.NumField(); i++ {
				fieldStruct := t.Field(i)
				SetDefaultValueIfNil(fieldStruct, vValue.Field(i))
			}
		case reflect.Ptr:
			pValue := reflect.New(structField.Type.Elem()).Elem()
			for i := 0; i < pValue.NumField(); i++ {
				field := vValue.Elem().Field(i)
				fieldStruct := pValue.Type().Field(i)
				SetDefaultValueIfNil(fieldStruct, field)
			}
		case reflect.Bool:
			fmt.Println("bool can't use Func[SetDefaultValueIfNil]")
		default:

		}
	}
}

func containTag(tag reflect.StructTag, tagName string) bool {
	return regexp.MustCompile(`\b` + tagName + `\b`).Match([]byte(tag))
}
