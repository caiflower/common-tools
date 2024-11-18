package v1

import (
	"fmt"
	"net/http"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/caiflower/common-tools/pkg/tools"
)

func setHeader(structField reflect.StructField, vValue reflect.Value, data interface{}) (err error) {
	if !vValue.CanSet() {
		return
	}

	header := data.(http.Header)
	tagName := "header"
	structTag := structField.Tag
	if containTag(structTag, tagName) {
		tagValue := structTag.Get(tagName)
		if v, ok := header[tagValue]; ok {
			err = tools.ReflectCommonSet(structField, vValue, v)
		}
	} else {
		switch vValue.Kind() {
		case reflect.Ptr:
			// 获取指针指向的值
			pValue := reflect.New(structField.Type.Elem()).Elem()

			// 递归处理指针指向的值
			switch pValue.Kind() {
			case reflect.Struct:
				newValue := vValue
				if vValue.IsZero() {
					newValue = reflect.New(structField.Type.Elem())
				}

				for i := 0; i < pValue.NumField(); i++ {
					field := newValue.Elem().Field(i)
					fieldStruct := pValue.Type().Field(i)
					if err = setHeader(fieldStruct, field, data); err != nil {
						return
					}
				}

				if vValue.IsZero() {
					setFlag := false
					for i := 0; i < pValue.NumField(); i++ {
						field := newValue.Elem().Field(i)
						if !field.IsZero() {
							setFlag = true
							break
						}
					}

					if setFlag {
						vValue.Set(newValue)
					}
				}
			default:
				return setHeader(structField, pValue, data)
			}
		case reflect.Struct:
			t := structField.Type
			for i := 0; i < t.NumField(); i++ {
				fieldStruct := t.Field(i)
				if err = setHeader(fieldStruct, vValue.Field(i), data); err != nil {
					return
				}
			}
		default:
		}
	}

	return
}

func setPath(structField reflect.StructField, vValue reflect.Value, data interface{}) (err error) {
	m := data.(map[string]string)

	var value string
	name := structField.Name
	// 首字母变小
	lName := strings.ToLower(name[:1]) + name[1:]
	for k, v := range m {
		if tools.ToCamel(k) == lName || name == k {
			value = v
			break
		}
	}

	if value != "" {
		err = tools.ReflectCommonSet(structField, vValue, []string{value})
	}
	return
}

func setParam(structField reflect.StructField, vValue reflect.Value, data interface{}) (err error) {
	if !vValue.CanSet() {
		return
	}

	var params []string
	m := data.(map[string][]string)
	structTag := structField.Tag
	if containTag(structTag, "param") {
		params = m[structTag.Get("param")]
	} else {
		name := structField.Name
		// 首字母变小
		lName := strings.ToLower(name[:1]) + name[1:]
		for k, v := range m {
			if tools.ToCamel(k) == lName || name == k {
				params = v
				break
			}
		}
	}

	if len(params) > 0 {
		switch vValue.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			v, _ := strconv.Atoi(params[0])
			vValue.SetInt(int64(v))
		case reflect.Float32, reflect.Float64:
			v, _ := strconv.ParseFloat(params[0], 64)
			vValue.SetFloat(v)
		case reflect.String:
			vValue.SetString(params[0])
		case reflect.Slice:
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
					if structTag.Get("param") != "" {
						return fmt.Errorf("unsupported tag param:'%s'", structField.Name)
					}
				}
			}
			vValue.Set(slice)
		case reflect.Ptr:
			pValue := reflect.New(structField.Type.Elem()).Elem()
			switch pValue.Kind() {
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				v, _ := strconv.Atoi(params[0])
				vValue.Set(reflect.ValueOf(&v))
			case reflect.String:
				vValue.Set(reflect.ValueOf(&params[0]))
			case reflect.Float32, reflect.Float64:
				v, _ := strconv.ParseFloat(params[0], 64)
				vValue.Set(reflect.ValueOf(&v))
			case reflect.Bool:
				v, _ := strconv.ParseBool(params[0])
				vValue.Set(reflect.ValueOf(&v))
			default:
			}
		default:
			if structTag.Get("param") != "" {
				return fmt.Errorf("unsupported tag param:'%s'", structField.Name)
			}
		}
	}

	return
}

func containTag(tag reflect.StructTag, tagName string) bool {
	return regexp.MustCompile(`\b` + tagName + `\b`).Match([]byte(tag))
}
