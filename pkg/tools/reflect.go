/*
 * Copyright 2024 caiflower Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

 package tools

import (
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

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

	flag := false
	structTag := structField.Tag
	if ContainTag(structTag, "default") || vValue.Kind() == reflect.Struct || vValue.Kind() == reflect.Ptr {
		switch vValue.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			if vValue.Int() == 0 {
				if v, err := strconv.Atoi(structTag.Get("default")); err == nil {
					vValue.SetInt(int64(v))
					flag = true
				} else {
					if v, err := time.ParseDuration(structTag.Get("default")); err == nil {
						vValue.SetInt(int64(v))
						flag = true
					}
				}
			}
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			if vValue.Uint() == 0 {
				v, _ := strconv.Atoi(structTag.Get("default"))
				vValue.SetUint(uint64(v))
				flag = true
			}
		case reflect.String:
			if vValue.String() == "" {
				vValue.SetString(structTag.Get("default"))
				flag = true
			}
		case reflect.Float32:
			if vValue.Float() == 0 {
				v, _ := strconv.ParseFloat(structTag.Get("default"), 32)
				vValue.SetFloat(v)
				flag = true
			}
		case reflect.Float64:
			if vValue.Float() == 0 {
				v, _ := strconv.ParseFloat(structTag.Get("default"), 64)
				vValue.SetFloat(v)
				flag = true
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
			if vValue.IsNil() {
				switch pValue.Kind() {
				case reflect.Int:
					if v := structTag.Get("default"); v != "" {
						i, _ := strconv.Atoi(v)
						vValue.Set(reflect.ValueOf(&i))
						flag = true
					}
				case reflect.Int8:
					if v := structTag.Get("default"); v != "" {
						i, _ := strconv.Atoi(v)
						_i := int8(i)
						vValue.Set(reflect.ValueOf(&_i))
						flag = true
					}
				case reflect.Int16:
					if v := structTag.Get("default"); v != "" {
						i, _ := strconv.Atoi(v)
						_i := int16(i)
						vValue.Set(reflect.ValueOf(&_i))
						flag = true
					}
				case reflect.Int32:
					if v := structTag.Get("default"); v != "" {
						i, _ := strconv.Atoi(v)
						_i := int32(i)
						vValue.Set(reflect.ValueOf(&_i))
						flag = true
					}
				case reflect.Int64:
					if v := structTag.Get("default"); v != "" {
						i, _ := strconv.Atoi(v)
						_i := int64(i)
						vValue.Set(reflect.ValueOf(&_i))
						flag = true
					}
				case reflect.Uint:
					if v := structTag.Get("default"); v != "" {
						i, _ := strconv.Atoi(v)
						_i := uint(i)
						vValue.Set(reflect.ValueOf(&_i))
						flag = true
					}
				case reflect.Uint8:
					if v := structTag.Get("default"); v != "" {
						i, _ := strconv.Atoi(v)
						_i := uint8(i)
						vValue.Set(reflect.ValueOf(&_i))
						flag = true
					}
				case reflect.Uint16:
					if v := structTag.Get("default"); v != "" {
						i, _ := strconv.Atoi(v)
						_i := uint16(i)
						vValue.Set(reflect.ValueOf(&_i))
						flag = true
					}
				case reflect.Uint32:
					if v := structTag.Get("default"); v != "" {
						i, _ := strconv.Atoi(v)
						_i := uint32(i)
						vValue.Set(reflect.ValueOf(&_i))
						flag = true
					}
				case reflect.Uint64:
					if v := structTag.Get("default"); v != "" {
						i, _ := strconv.Atoi(v)
						_i := uint64(i)
						vValue.Set(reflect.ValueOf(&_i))
						flag = true
					}
				case reflect.String:
					if v := structTag.Get("default"); v != "" {
						vValue.Set(reflect.ValueOf(&v))
						flag = true
					}
				case reflect.Float32:
					if v := structTag.Get("default"); v != "" {
						f, _ := strconv.ParseFloat(v, 32)
						_f := float32(f)
						vValue.Set(reflect.ValueOf(&_f))
						flag = true
					}
				case reflect.Float64:
					if v := structTag.Get("default"); v != "" {
						f, _ := strconv.ParseFloat(v, 64)
						vValue.Set(reflect.ValueOf(&f))
						flag = true
					}
				case reflect.Bool:
					if v := structTag.Get("default"); v != "" {
						b, _ := strconv.ParseBool(v)
						vValue.Set(reflect.ValueOf(&b))
						flag = true
					}
				case reflect.Ptr:
					fmt.Println("ptr ptr no support Func[SetDefaultValueIfNil]")
				case reflect.Struct:
					newValue := vValue
					if vValue.IsZero() {
						newValue = reflect.New(structField.Type.Elem())
					}

					m := make(map[string]string)
					for i := 0; i < pValue.NumField(); i++ {
						field := newValue.Elem().Field(i)
						fieldStruct := pValue.Type().Field(i)
						if err = SetDefaultValueIfNil(fieldStruct, field, m); err != nil {
							return
						}
					}
					if len(m) != 0 {
						vValue.Set(newValue)
					}
				default:

				}
			}
		default:
			fmt.Printf("SetDefaultValueIfNil failed. %v", vValue.Kind())
		}
	}

	if flag && data != nil {
		data.(map[string]string)["flag"] = "true"
	}

	return
}

func CheckNil(structField reflect.StructField, vValue reflect.Value, data interface{}) (err error) {
	verf := structField.Tag.Get("verf")

	switch vValue.Kind() {
	case reflect.Ptr:
		pValue := reflect.New(structField.Type.Elem()).Elem()
		switch pValue.Kind() {
		case reflect.Struct:
			newValue := vValue
			if vValue.IsZero() {
				newValue = reflect.New(structField.Type.Elem())
			}

			for i := 0; i < pValue.NumField(); i++ {
				field := newValue.Elem().Field(i)
				fieldStruct := pValue.Type().Field(i)
				if err = CheckNil(fieldStruct, field, data); err != nil {
					return
				}
			}
		default:

		}
	case reflect.Struct:
		t := structField.Type
		for i := 0; i < t.NumField(); i++ {
			fieldStruct := t.Field(i)
			if err = CheckNil(fieldStruct, vValue.Field(i), data); err != nil {
				return
			}
		}
	default:

	}

	if !ContainTag(structField.Tag, "verf") {
		return
	}

	if verf != "nilable" && vValue.IsZero() {
		return fmt.Errorf("%s is missing", structField.Name)
	}

	return
}

func CheckInList(structField reflect.StructField, vValue reflect.Value, data interface{}) (err error) {
	if structField.Tag.Get("verf") == "nilable" && vValue.IsZero() {
		return
	}

	tagName := "inList"
	inList := strings.Split(structField.Tag.Get(tagName), ",")

	var values []string
	switch vValue.Kind() {
	case reflect.Ptr:
		// 获取指针指向的值
		indirectValue := vValue.Elem()

		// 递归处理指针指向的值
		switch indirectValue.Kind() {
		case reflect.Struct:
			t := indirectValue.Type()
			for i := 0; i < t.NumField(); i++ {
				fieldStruct := t.Field(i)
				if err = CheckInList(fieldStruct, indirectValue.Field(i), data); err != nil {
					return
				}
			}
			return
		default:
			return CheckInList(structField, indirectValue, data)
		}
	case reflect.Struct:
		t := structField.Type
		for i := 0; i < t.NumField(); i++ {
			fieldStruct := t.Field(i)
			if err = CheckInList(fieldStruct, vValue.Field(i), data); err != nil {
				return
			}
		}
		return
	case reflect.Slice:
		if !ContainTag(structField.Tag, tagName) {
			return
		}
		for i := 0; i < vValue.Len(); i++ {
			elementValue := vValue.Index(i)
			switch elementValue.Kind() {
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				values = append(values, fmt.Sprintf("%d", elementValue.Int()))
			case reflect.Float32, reflect.Float64:
				values = append(values, fmt.Sprintf("%f", elementValue.Float()))
			case reflect.String:
				values = append(values, elementValue.String())
			default:
				return
			}
		}
	case reflect.Map:
		if !ContainTag(structField.Tag, tagName) {
			return
		}
		keys := vValue.MapKeys()
		for _, key := range keys {
			elementValue := vValue.MapIndex(key)
			switch elementValue.Kind() {
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				values = append(values, fmt.Sprintf("%d", elementValue.Int()))
			case reflect.Float32, reflect.Float64:
				values = append(values, fmt.Sprintf("%f", elementValue.Float()))
			case reflect.String:
				values = append(values, elementValue.String())
			default:
				return
			}
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if !ContainTag(structField.Tag, tagName) {
			return
		}
		values = append(values, fmt.Sprintf("%d", vValue.Int()))
	case reflect.Float32, reflect.Float64:
		if !ContainTag(structField.Tag, tagName) {
			return
		}
		values = append(values, fmt.Sprintf("%f", vValue.Float()))
	case reflect.String:
		if !ContainTag(structField.Tag, tagName) {
			return
		}
		values = append(values, vValue.String())
	default:
		return
	}

	if len(values) == 0 {
		return fmt.Errorf("%s is not in %v", structField.Name, strings.Join(inList, ","))
	}

	for _, value := range values {
		if !StringSliceContains(inList, value) {
			return fmt.Errorf("%s is not in %v", structField.Name, strings.Join(inList, ","))
		}
	}

	return
}

func CheckRegxp(structField reflect.StructField, vValue reflect.Value, data interface{}) (err error) {
	if structField.Tag.Get("verf") == "nilable" && vValue.IsZero() {
		return
	}

	tagName := "reg"
	tagValueStr := structField.Tag.Get(tagName)

	var values []string
	switch vValue.Kind() {
	case reflect.Ptr:
		// 获取指针指向的值
		indirectValue := vValue.Elem()

		// 递归处理指针指向的值
		switch indirectValue.Kind() {
		case reflect.Struct:
			t := indirectValue.Type()
			for i := 0; i < t.NumField(); i++ {
				fieldStruct := t.Field(i)
				if err = CheckRegxp(fieldStruct, indirectValue.Field(i), data); err != nil {
					return
				}
			}
			return
		default:
			return CheckRegxp(structField, indirectValue, data)
		}
	case reflect.Struct:
		t := structField.Type
		for i := 0; i < t.NumField(); i++ {
			fieldStruct := t.Field(i)
			if err = CheckRegxp(fieldStruct, vValue.Field(i), data); err != nil {
				return
			}
		}
		return
	default:
		if len(tagValueStr) == 0 {
			return
		}
	}

	values = ReflectCommonGet(structField, vValue)
	if len(values) == 0 {
		return fmt.Errorf("%s is not match %s", structField.Name, tagValueStr)
	}

	for _, value := range values {
		if !MatchReg(value, tagValueStr) {
			return fmt.Errorf("%s is not match %s", structField.Name, tagValueStr)
		}
	}

	return
}

func CheckBetween(structField reflect.StructField, vValue reflect.Value, data interface{}) (err error) {
	if structField.Tag.Get("verf") == "nilable" && vValue.IsZero() {
		return
	}

	tagName := "between"
	tagValueStr := structField.Tag.Get(tagName)

	switch vValue.Kind() {
	case reflect.Ptr:
		// 获取指针指向的值
		indirectValue := vValue.Elem()

		// 递归处理指针指向的值
		switch indirectValue.Kind() {
		case reflect.Struct:
			t := indirectValue.Type()
			for i := 0; i < t.NumField(); i++ {
				fieldStruct := t.Field(i)
				if err = CheckBetween(fieldStruct, indirectValue.Field(i), data); err != nil {
					return
				}
			}
			return
		default:
			return CheckBetween(structField, indirectValue, data)
		}
	case reflect.Struct:
		t := structField.Type
		for i := 0; i < t.NumField(); i++ {
			fieldStruct := t.Field(i)
			if err = CheckBetween(fieldStruct, vValue.Field(i), data); err != nil {
				return
			}
		}
		return
	default:
		if len(tagValueStr) == 0 {
			return
		}
	}

	tagValue := strings.Split(tagValueStr, ",")
	values := ReflectCommonGet(structField, vValue)

	for _, value := range values {
		if len(tagValue) == 2 && tagValue[0] != "" && tagValue[1] != "" && (value < tagValue[0] || value > tagValue[1]) {
			return fmt.Errorf("%s is not between %s", structField.Name, strings.Join(tagValue, "-"))
		} else if tagValue[0] != "" && value < tagValue[0] {
			return fmt.Errorf("%s is must >= %s", structField.Name, tagValue[0])
		} else if len(tagValue) == 2 && tagValue[1] != "" && value > tagValue[1] {
			return fmt.Errorf("%s is must <= %s", structField.Name, tagValue[1])
		}
	}

	return
}

func CheckLen(structField reflect.StructField, vValue reflect.Value, data interface{}) (err error) {
	if structField.Tag.Get("verf") == "nilable" && vValue.IsZero() {
		return
	}

	tagName := "len"
	tagValueStr := structField.Tag.Get(tagName)

	switch vValue.Kind() {
	case reflect.Ptr:
		// 获取指针指向的值
		indirectValue := vValue.Elem()

		// 递归处理指针指向的值
		switch indirectValue.Kind() {
		case reflect.Struct:
			t := indirectValue.Type()
			for i := 0; i < t.NumField(); i++ {
				fieldStruct := t.Field(i)
				if err = CheckLen(fieldStruct, indirectValue.Field(i), data); err != nil {
					return
				}
			}
			return
		default:
			return CheckLen(structField, indirectValue, data)
		}
	case reflect.Struct:
		t := structField.Type
		for i := 0; i < t.NumField(); i++ {
			fieldStruct := t.Field(i)
			if err = CheckLen(fieldStruct, vValue.Field(i), data); err != nil {
				return
			}
		}
		return
	default:
		if len(tagValueStr) == 0 {
			return
		}
	}

	tagValues := strings.Split(tagValueStr, ",")

	var min, max int
	var minErr, maxErr error
	if tagValues[0] != "" {
		min, minErr = strconv.Atoi(tagValues[0])
		if minErr != nil {
			return
		}
	} else {
		minErr = fmt.Errorf("no min")
	}

	if len(tagValues) >= 2 && tagValues[1] != "" {
		max, maxErr = strconv.Atoi(tagValues[1])
		if maxErr != nil {
			return
		}
	} else {
		maxErr = fmt.Errorf("no max")
	}

	values := ReflectCommonGet(structField, vValue)
	for _, value := range values {
		if minErr == nil && maxErr == nil && (len(value) < min || len(value) > max) {
			return fmt.Errorf("%s len is not between %d-%d", structField.Name, min, max)
		} else if minErr == nil && len(value) < min {
			return fmt.Errorf("%s len must >= %d", structField.Name, min)
		} else if maxErr == nil && len(value) > max {
			return fmt.Errorf("%s len must <= %d", structField.Name, max)
		}
	}

	return
}

func ReflectCommonSet(structField reflect.StructField, vValue reflect.Value, values []string) (err error) {
	if len(values) <= 0 {
		return
	}
	value := values[0]

	switch vValue.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if vValue.Int() == 0 {
			v, _ := strconv.Atoi(value)
			vValue.SetInt(int64(v))
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if vValue.Uint() == 0 {
			v, _ := strconv.Atoi(value)
			vValue.SetUint(uint64(v))
		}
	case reflect.String:
		if vValue.String() == "" {
			vValue.SetString(value)
		}
	case reflect.Float32, reflect.Float64:
		if vValue.Float() == 0 {
			v, _ := strconv.ParseFloat(value, 64)
			vValue.SetFloat(v)
		}
	case reflect.Slice:
		elemType := vValue.Type().Elem()
		slice := reflect.MakeSlice(reflect.SliceOf(elemType), len(values), len(values))
		for i, param := range values {
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

			}
		}
		vValue.Set(slice)
	case reflect.Bool:
		v, _ := strconv.ParseBool(value)
		vValue.Set(reflect.ValueOf(&v))
	case reflect.Ptr:
		pValue := reflect.New(structField.Type.Elem()).Elem()
		switch pValue.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			if vValue.IsNil() {
				v, _ := strconv.Atoi(value)
				vValue.Set(reflect.ValueOf(&v))
			}
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			if vValue.IsNil() {
				v, _ := strconv.Atoi(value)
				vValue.SetUint(uint64(v))
			}
		case reflect.String:
			if vValue.IsNil() {
				v := value
				vValue.Set(reflect.ValueOf(&v))
			}
		case reflect.Float32, reflect.Float64:
			if vValue.IsNil() {
				v, _ := strconv.ParseFloat(value, 64)
				vValue.Set(reflect.ValueOf(&v))
			}
		case reflect.Bool:
			if vValue.IsNil() {
				v, _ := strconv.ParseBool(value)
				vValue.Set(reflect.ValueOf(&v))
			}
		default:

		}
	default:

	}
	return
}

func ReflectCommonGet(structField reflect.StructField, vValue reflect.Value) (values []string) {
	switch vValue.Kind() {
	case reflect.Ptr:
		// 获取指针指向的值
		indirectValue := vValue.Elem()

		// 递归处理指针指向的值
		switch indirectValue.Kind() {
		case reflect.Struct:
			t := indirectValue.Type()
			for i := 0; i < t.NumField(); i++ {
				fieldStruct := t.Field(i)
				values = append(values, ReflectCommonGet(fieldStruct, indirectValue.Field(i))...)
			}
		default:
			values = append(values, ReflectCommonGet(structField, indirectValue)...)
		}
	case reflect.Struct:
		t := structField.Type
		for i := 0; i < t.NumField(); i++ {
			fieldStruct := t.Field(i)
			values = append(values, ReflectCommonGet(fieldStruct, vValue.Field(i))...)
		}
	case reflect.Slice:
		for i := 0; i < vValue.Len(); i++ {
			elementValue := vValue.Index(i)
			switch elementValue.Kind() {
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				values = append(values, fmt.Sprintf("%d", elementValue.Int()))
			case reflect.Float32, reflect.Float64:
				values = append(values, fmt.Sprintf("%f", elementValue.Float()))
			case reflect.String:
				values = append(values, elementValue.String())
			default:

			}
		}
	case reflect.Map:
		keys := vValue.MapKeys()
		for _, key := range keys {
			elementValue := vValue.MapIndex(key)
			switch elementValue.Kind() {
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				values = append(values, fmt.Sprintf("%d", elementValue.Int()))
			case reflect.Float32, reflect.Float64:
				values = append(values, fmt.Sprintf("%f", elementValue.Float()))
			case reflect.String:
				values = append(values, elementValue.String())
			default:

			}
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		values = append(values, fmt.Sprintf("%d", vValue.Int()))
	case reflect.Float32, reflect.Float64:
		values = append(values, fmt.Sprintf("%f", vValue.Float()))
	case reflect.String:
		values = append(values, vValue.String())
	default:

	}

	return
}

func ContainTag(tag reflect.StructTag, tagName string) bool {
	return regexp.MustCompile(`\b` + tagName + `\b`).Match([]byte(tag))
}
