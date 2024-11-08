package tools

import (
	"fmt"
	"reflect"
	"regexp"
	"strconv"

	"github.com/modern-go/reflect2"
)

func DoTagFunc(v interface{}, data interface{}, fn []func(reflect.StructField, reflect.Value, interface{}) error) (err error) {
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
			if err = f(fieldStruct, field, data); err != nil {
				return
			}
		}
	}

	return
}

func SetDefaultValueIfNil(structField reflect.StructField, vValue reflect.Value, data interface{}) (err error) {
	if !vValue.CanSet() {
		return
	}
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
				if err = SetDefaultValueIfNil(fieldStruct, vValue.Field(i), data); err != nil {
					return
				}
			}
		case reflect.Ptr:
			pValue := reflect.New(structField.Type.Elem()).Elem()
			switch pValue.Kind() {
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				if vValue.IsNil() {
					v, _ := strconv.Atoi(structTag.Get("default"))
					vValue.Set(reflect.ValueOf(&v))
				}
			case reflect.String:
				if vValue.IsNil() {
					v := structTag.Get("default")
					vValue.Set(reflect.ValueOf(&v))
				}
			case reflect.Float32:
				if vValue.IsNil() {
					v, _ := strconv.ParseFloat(structTag.Get("default"), 32)
					vValue.Set(reflect.ValueOf(&v))
				}
			case reflect.Float64:
				if vValue.IsNil() {
					v, _ := strconv.ParseFloat(structTag.Get("default"), 64)
					vValue.Set(reflect.ValueOf(&v))
				}
			case reflect.Bool:
				if vValue.IsNil() {
					v, _ := strconv.ParseBool(structTag.Get("default"))
					vValue.Set(reflect.ValueOf(&v))
				}
			case reflect.Ptr:
				fmt.Println("ptr ptr no support Func[SetDefaultValueIfNil]")
			case reflect.Struct:
				for i := 0; i < pValue.NumField(); i++ {
					field := vValue.Elem().Field(i)
					fieldStruct := pValue.Type().Field(i)
					if err = SetDefaultValueIfNil(fieldStruct, field, data); err != nil {
						return
					}
				}
			default:
			}
		case reflect.Bool:
			fmt.Println("bool no support Func[SetDefaultValueIfNil]")
		default:

		}
	}

	return
}

func SetParam(structField reflect.StructField, vValue reflect.Value, data interface{}) (err error) {
	if !vValue.CanSet() {
		return
	}
	structTag := structField.Tag
	if containTag(structTag, "param") {
		m := data.(map[string][]string)
		params := m[structTag.Get("param")]
		switch vValue.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			if len(params) > 0 {
				v, _ := strconv.Atoi(params[0])
				vValue.SetInt(int64(v))
			}
		case reflect.Float32, reflect.Float64:
			if len(params) > 0 {
				v, _ := strconv.ParseFloat(params[0], 64)
				vValue.SetFloat(v)
			}
		case reflect.String:
			if len(params) > 0 {
				vValue.SetString(params[0])
			}
		case reflect.Slice:
			if len(params) > 0 {
				elemType := vValue.Type().Elem()
				slice := reflect.MakeSlice(reflect.SliceOf(elemType), len(params), len(params))
				for i, param := range params {
					switch elemType.Kind() {
					case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
						v, _ := strconv.ParseInt(param, 10, 64)
						slice.Index(i).SetInt(v)
					case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
						v, _ := strconv.ParseUint(param, 10, 64)
						slice.Index(i).SetUint(v)
					case reflect.Float32, reflect.Float64:
						v, _ := strconv.ParseFloat(param, 64)
						slice.Index(i).SetFloat(v)
					case reflect.String:
						slice.Index(i).SetString(param)
					default:
						return fmt.Errorf("unsupported tag param:'%s'", structTag.Get("param"))
					}
				}
				vValue.Set(slice)
			}
		default:
			return fmt.Errorf("unsupported tag param:'%s'", structTag.Get("param"))
		}
	}

	return
}

func containTag(tag reflect.StructTag, tagName string) bool {
	return regexp.MustCompile(`\b` + tagName + `\b`).Match([]byte(tag))
}
