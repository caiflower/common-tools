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

package web

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/caiflower/common-tools/pkg/basic"
	"github.com/caiflower/common-tools/pkg/tools"
)

const (
	verf    = "verf"
	inList  = "inList"
	reg     = "reg"
	between = "between"

	lenTag     = "len"     // 数组，map等所有元素长度总和
	itemLenTag = "itemLen" // 单个元素长度
)

type ValidFunc func([]string, reflect.StructField, reflect.Value, string) error

var validMap = make(map[string][]ValidFunc)

func buildValid(structField reflect.StructField, vValue reflect.Value, data interface{}) (err error) {
	switch vValue.Kind() {
	case reflect.Ptr:
		pValue := reflect.New(structField.Type.Elem()).Elem()
		switch pValue.Kind() {
		case reflect.Struct:
			newValue := vValue
			if vValue.IsZero() {
				newValue = reflect.New(structField.Type.Elem())
			}

			if isTime(structField, newValue) {

			} else {
				for i := 0; i < pValue.NumField(); i++ {
					field := newValue.Elem().Field(i)
					fieldStruct := pValue.Type().Field(i)
					dataTmp := data.(string)
					if !structField.Anonymous {
						dataTmp = data.(string) + "." + structField.Name
					}
					if err = buildValid(fieldStruct, field, dataTmp); err != nil {
						return
					}
				}
			}
		default:
		}
	case reflect.Struct:
		t := structField.Type
		if isTime(structField, vValue) {

		} else {
			for i := 0; i < t.NumField(); i++ {
				fieldStruct := t.Field(i)
				dataTmp := data.(string)
				if !structField.Anonymous {
					dataTmp = data.(string) + "." + structField.Name
				}
				if err = buildValid(fieldStruct, vValue.Field(i), dataTmp); err != nil {
					return
				}
			}
			return
		}
	default:

	}

	fieldName := data.(string) + "." + structField.Name
	if _, ok := validMap[fieldName]; ok {
		return
	}

	var validFuncs []ValidFunc

	if tools.ContainTag(structField.Tag, verf) {
		validFuncs = append(validFuncs, verfValidFunc)
	}
	if tools.ContainTag(structField.Tag, inList) {
		validFuncs = append(validFuncs, inListValidFunc)
	}
	if tools.ContainTag(structField.Tag, reg) {
		validFuncs = append(validFuncs, regValidFunc)
	}
	if tools.ContainTag(structField.Tag, between) {
		if len(strings.Split(structField.Tag.Get(between), ",")) != 2 {
			panic(fmt.Sprintf("%s tag[%s] value is not vaild", structField.Name, between))
		}
		validFuncs = append(validFuncs, betweenValidFunc)
	}
	if tools.ContainTag(structField.Tag, lenTag) {
		splits := strings.Split(structField.Tag.Get(lenTag), ",")
		if len(splits) > 2 {
			panic(fmt.Sprintf("%s tag[%s] value is not vaild", structField.Name, lenTag))
		}
		if splits[0] != "" {
			if _, err = strconv.Atoi(splits[0]); err != nil {
				panic(fmt.Sprintf("%s tag[%s] value is not vaild", structField.Name, lenTag))
			}
		}
		if len(splits) == 2 {
			if _, err = strconv.Atoi(splits[1]); err != nil {
				panic(fmt.Sprintf("%s tag[%s] value is not vaild", structField.Name, lenTag))
			}
		}
		validFuncs = append(validFuncs, lenValidFunc)
	}
	if tools.ContainTag(structField.Tag, itemLenTag) {
		splits := strings.Split(structField.Tag.Get(itemLenTag), ",")
		if len(splits) > 2 {
			panic(fmt.Sprintf("%s tag[%s] value is not vaild", structField.Name, itemLenTag))
		}
		if splits[0] != "" {
			if _, err = strconv.Atoi(splits[0]); err != nil {
				panic(fmt.Sprintf("%s tag[%s] value is not vaild", structField.Name, itemLenTag))
			}
		}
		if len(splits) == 2 {
			if _, err = strconv.Atoi(splits[1]); err != nil {
				panic(fmt.Sprintf("%s tag[%s] value is not vaild", structField.Name, itemLenTag))
			}
		}
		validFuncs = append(validFuncs, itemLenValidFunc)
	}

	//fmt.Println(fieldName)
	validMap[fieldName] = validFuncs

	return nil
}

type validObject struct {
	pkgPath   string
	filedName string
}

func valid(structField reflect.StructField, vValue reflect.Value, data interface{}) (err error) {
	value, ok := structField.Tag.Lookup(verf)
	if (value == "nilable" || !ok) && vValue.IsZero() {
		return
	}

	object := data.(validObject)
	switch vValue.Kind() {
	case reflect.Ptr:
		// 获取指针指向的值
		pValue := reflect.New(structField.Type.Elem()).Elem()
		switch pValue.Kind() {
		case reflect.Struct:
			newValue := vValue
			if vValue.IsZero() {
				newValue = reflect.New(structField.Type.Elem())
			}

			if isTime(structField, newValue) {
			} else {
				for i := 0; i < pValue.NumField(); i++ {
					field := newValue.Elem().Field(i)
					fieldStruct := pValue.Type().Field(i)
					objectTmp := object
					if !structField.Anonymous {
						objectTmp.filedName += "." + structField.Name
					}
					if err = valid(fieldStruct, field, objectTmp); err != nil {
						return
					}
				}
			}

		default:
		}
	case reflect.Struct:
		t := structField.Type
		if isTime(structField, vValue) {
		} else {
			for i := 0; i < t.NumField(); i++ {
				fieldStruct := t.Field(i)
				objectTmp := object
				if !structField.Anonymous {
					objectTmp.filedName += "." + structField.Name
				}
				if err = valid(fieldStruct, vValue.Field(i), objectTmp); err != nil {
					return
				}
			}
			return
		}
	default:
	}

	fieldName := object.pkgPath + "." + object.filedName + "." + structField.Name
	//fmt.Printf("valid %s\n", fieldName)
	if fnList, ok := validMap[fieldName]; ok {
		var values []string
		if value, ok1 := getTimeValue(structField, vValue); ok1 {
			values = append(values, value)
		} else {
			values = tools.ReflectCommonGet(structField, vValue)
		}

		for _, fn := range fnList {
			if err = fn(values, structField, vValue, object.filedName+"."+structField.Name); err != nil {
				return err
			}
		}
	}

	return
}

func getTimeValue(structField reflect.StructField, vValue reflect.Value) (string, bool) {
	if vValue.Kind() == reflect.Struct && reflect.New(structField.Type).Type().AssignableTo(reflect.TypeOf(new(basic.TimeStandard))) {
		t := vValue.Interface().(basic.TimeStandard)
		return t.String(), true
	} else if vValue.Kind() == reflect.Struct && reflect.New(structField.Type).Type().AssignableTo(reflect.TypeOf(new(basic.Time))) {
		t := vValue.Interface().(basic.Time)
		return t.String(), true
	} else if vValue.Kind() == reflect.Struct && reflect.New(structField.Type).Type().AssignableTo(reflect.TypeOf(new(time.Time))) {
		t := vValue.Interface().(time.Time)
		return t.String(), true
	} else if vValue.Kind() == reflect.Ptr && reflect.New(structField.Type.Elem()).Type().AssignableTo(reflect.TypeOf(new(basic.TimeStandard))) {
		t := reflect.New(structField.Type.Elem()).Elem().Interface().(basic.TimeStandard)
		return t.String(), true
	} else if vValue.Kind() == reflect.Ptr && reflect.New(structField.Type.Elem()).Type().AssignableTo(reflect.TypeOf(new(basic.Time))) {
		t := reflect.New(structField.Type.Elem()).Elem().Interface().(basic.Time)
		return t.String(), true
	} else if vValue.Kind() == reflect.Ptr && reflect.New(structField.Type.Elem()).Type().AssignableTo(reflect.TypeOf(new(time.Time))) {
		t := reflect.New(structField.Type.Elem()).Elem().Interface().(time.Time)
		return t.String(), true
	} else {
		return "", false
	}
}

func isTime(structField reflect.StructField, value reflect.Value) bool {
	switch value.Kind() {
	case reflect.Ptr:
		return reflect.New(structField.Type.Elem()).Type().AssignableTo(reflect.TypeOf(new(basic.TimeStandard))) || reflect.New(structField.Type.Elem()).Type().AssignableTo(reflect.TypeOf(new(basic.Time))) || reflect.New(structField.Type.Elem()).Type().AssignableTo(reflect.TypeOf(new(time.Time)))
	case reflect.Struct:
		return reflect.New(structField.Type).Type().AssignableTo(reflect.TypeOf(new(basic.TimeStandard))) || reflect.New(structField.Type).Type().AssignableTo(reflect.TypeOf(new(basic.Time))) || reflect.New(structField.Type).Type().AssignableTo(reflect.TypeOf(new(time.Time)))
	default:
		return false
	}
}

func verfValidFunc(values []string, structField reflect.StructField, vValue reflect.Value, fieldName string) error {
	if len(values) == 0 {
		return fmt.Errorf("%s is missing", fieldName)
	}
	switch vValue.Kind() {
	case reflect.Slice:
		for i, v := range values {
			if v == "" {
				return fmt.Errorf("%s[%d] is missing", fieldName, i)
			}
		}
	case reflect.Ptr:
		if vValue.IsZero() {
			return fmt.Errorf("%s is missing", fieldName)
		}
	default:
		if values[0] == "" {
			return fmt.Errorf("%s is missing", fieldName)
		}
	}

	return nil
}

func inListValidFunc(values []string, structField reflect.StructField, vValue reflect.Value, fieldName string) error {
	inListValue := structField.Tag.Get(inList)
	splits := strings.Split(inListValue, ",")
	switch vValue.Kind() {
	case reflect.Struct, reflect.Ptr, reflect.Interface:
	case reflect.Slice:
		for i, v := range values {
			if !tools.StringSliceContains(splits, v) {
				return fmt.Errorf("%s[%d] '%s' is not in %v", fieldName, i, v, splits)
			}
		}
	default:
		for _, v := range values {
			if !tools.StringSliceContains(splits, v) {
				return fmt.Errorf("%s is not in %v", fieldName, splits)
			}
		}
	}

	return nil
}

func regValidFunc(values []string, structField reflect.StructField, vValue reflect.Value, fieldName string) error {
	tagValue := structField.Tag.Get(reg)
	switch vValue.Kind() {
	case reflect.Struct, reflect.Ptr, reflect.Interface:
	case reflect.Slice:
		for i, v := range values {
			if !tools.MatchReg(v, tagValue) {
				return fmt.Errorf("%s[%d] '%s' is not match %v", fieldName, i, v, tagValue)
			}
		}
	default:
		for _, v := range values {
			if !tools.MatchReg(v, tagValue) {
				return fmt.Errorf("%s is not match %v", fieldName, tagValue)
			}
		}
	}

	return nil
}

func betweenValidFunc(values []string, structField reflect.StructField, vValue reflect.Value, fieldName string) error {
	tagValue := structField.Tag.Get(between)
	splits := strings.Split(tagValue, ",")
	switch vValue.Kind() {
	case reflect.Struct, reflect.Ptr, reflect.Interface:
	case reflect.Slice:
		for i, v := range values {
			if v < splits[0] || v > splits[1] {
				return fmt.Errorf("%s[%d] '%s' is not between %s and %s", fieldName, i, v, splits[0], splits[1])
			}
		}
	default:
		for _, v := range values {
			if v < splits[0] || v > splits[1] {
				return fmt.Errorf("%s is not between %s and %s", fieldName, splits[0], splits[1])
			}
		}
	}

	return nil
}

func lenValidFunc(values []string, structField reflect.StructField, vValue reflect.Value, fieldName string) error {
	tagValue := structField.Tag.Get(lenTag)
	splits := strings.Split(tagValue, ",")
	switch vValue.Kind() {
	case reflect.Struct, reflect.Ptr, reflect.Interface:
	default:
		str := ""
		for _, v := range values {
			str += v
		}
		if splits[0] != "" {
			minLen, _ := strconv.Atoi(splits[0])
			if len(str) < minLen {
				return fmt.Errorf("%s len is less than %d", fieldName, minLen)
			}
		}
		if len(splits) >= 2 && splits[1] != "" {
			maxLen, _ := strconv.Atoi(splits[1])
			if len(str) > maxLen {
				return fmt.Errorf("%s len is greater than %d", fieldName, maxLen)
			}
		}
	}

	return nil
}

func itemLenValidFunc(values []string, structField reflect.StructField, vValue reflect.Value, fieldName string) error {
	tagValue := structField.Tag.Get(itemLenTag)
	splits := strings.Split(tagValue, ",")
	switch vValue.Kind() {
	case reflect.Struct, reflect.Ptr, reflect.Interface:
	default:
		if splits[0] != "" {
			minLen, _ := strconv.Atoi(splits[0])
			for i, v := range values {
				if len(v) < minLen {
					return fmt.Errorf("%s[%d] len is less than %d", fieldName, i, minLen)
				}
			}
		}
		if len(splits) >= 2 && splits[1] != "" {
			maxLen, _ := strconv.Atoi(splits[1])
			for i, v := range values {
				if len(v) > maxLen {
					return fmt.Errorf("%s[%d] len is greater than %d", fieldName, i, maxLen)
				}
			}
		}
	}

	return nil
}
