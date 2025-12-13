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

package reflectx

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

type validItem struct {
	validField
	validFunc
}

type validField struct {
	FieldKind reflect.Kind
	TagValue  string
}

type validFunc func([]string, string, *validField) error

var validMap = make(map[string][]validItem)

func BuildValid(structField reflect.StructField, vValue reflect.Value, data interface{}) (err error) {
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
					if err = BuildValid(fieldStruct, field, dataTmp); err != nil {
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
				if err = BuildValid(fieldStruct, vValue.Field(i), dataTmp); err != nil {
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

	var validItems []validItem
	fieldKind := vValue.Kind()

	if tools.ContainTag(structField.Tag, verf) {
		validItems = append(validItems, validItem{
			validField: validField{
				FieldKind: fieldKind,
				TagValue:  structField.Tag.Get(verf),
			},
			validFunc: verfValidFunc,
		})
	}
	if tools.ContainTag(structField.Tag, inList) {
		validItems = append(validItems, validItem{
			validField: validField{
				FieldKind: fieldKind,
				TagValue:  structField.Tag.Get(inList),
			},
			validFunc: inListValidFunc,
		})
	}
	if tools.ContainTag(structField.Tag, reg) {
		validItems = append(validItems, validItem{
			validField: validField{
				FieldKind: fieldKind,
				TagValue:  structField.Tag.Get(reg),
			},
			validFunc: regValidFunc,
		})
	}
	numCheckFn := func(splits []string, tag string) {
		if len(splits) > 2 {
			panic(fmt.Sprintf("%s tag[%s] value is not vaild", structField.Name, tag))
		}
		if splits[0] != "" {
			if _, err = strconv.Atoi(splits[0]); err != nil {
				panic(fmt.Sprintf("%s tag[%s] value is not vaild", structField.Name, tag))
			}
		}
		if len(splits) == 2 {
			if _, err = strconv.Atoi(splits[1]); err != nil {
				panic(fmt.Sprintf("%s tag[%s] value is not vaild", structField.Name, tag))
			}
		}
	}

	if tools.ContainTag(structField.Tag, between) {
		numCheckFn(strings.Split(structField.Tag.Get(between), ","), between)
		validItems = append(validItems, validItem{
			validField: validField{
				FieldKind: fieldKind,
				TagValue:  structField.Tag.Get(between),
			},
			validFunc: betweenValidFunc,
		})
	}
	if tools.ContainTag(structField.Tag, lenTag) {
		numCheckFn(strings.Split(structField.Tag.Get(lenTag), ","), lenTag)
		validItems = append(validItems, validItem{
			validField: validField{
				FieldKind: fieldKind,
				TagValue:  structField.Tag.Get(lenTag),
			},
			validFunc: lenValidFunc,
		})
	}
	if tools.ContainTag(structField.Tag, itemLenTag) {
		numCheckFn(strings.Split(structField.Tag.Get(itemLenTag), ","), itemLenTag)
		validItems = append(validItems, validItem{
			validField: validField{
				FieldKind: fieldKind,
				TagValue:  structField.Tag.Get(itemLenTag),
			},
			validFunc: itemLenValidFunc,
		})
	}

	validMap[fieldName] = validItems

	return nil
}

type ValidObject struct {
	PkgPath   string
	FiledName string
}

func CheckParam(structField reflect.StructField, vValue reflect.Value, data interface{}) (err error) {
	value, ok := structField.Tag.Lookup(verf)
	if (value == "nilable" || !ok) && vValue.IsZero() {
		return
	}

	object := data.(ValidObject)
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
						objectTmp.FiledName += "." + structField.Name
					}
					if err = CheckParam(fieldStruct, field, objectTmp); err != nil {
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
					objectTmp.FiledName += "." + structField.Name
				}
				if err = CheckParam(fieldStruct, vValue.Field(i), objectTmp); err != nil {
					return
				}
			}
			return
		}
	default:
	}

	fieldName := object.PkgPath + "." + object.FiledName + "." + structField.Name
	if validItems, ok := validMap[fieldName]; ok {
		var values []string
		if value, ok1 := getTimeValue(structField, vValue); ok1 {
			values = append(values, value)
		} else {
			values = tools.ReflectCommonGet(structField, vValue)
		}

		for _, v := range validItems {
			if err = v.validFunc(values, object.FiledName+"."+structField.Name, &v.validField); err != nil {
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

func verfValidFunc(values []string, fieldName string, field *validField) error {
	if len(values) == 0 {
		return fmt.Errorf("%s is missing", fieldName)
	}
	switch field.FieldKind {
	case reflect.Slice:
		for i, v := range values {
			if v == "" {
				return fmt.Errorf("%s[%d] is missing", fieldName, i)
			}
		}
	default:
		if values[0] == "" {
			return fmt.Errorf("%s is missing", fieldName)
		}
	}

	return nil
}

func inListValidFunc(values []string, fieldName string, field *validField) error {
	splits := strings.Split(field.TagValue, ",")
	switch field.FieldKind {
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

func regValidFunc(values []string, fieldName string, field *validField) error {
	tagValue := field.TagValue
	switch field.FieldKind {
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

func betweenValidFunc(values []string, fieldName string, field *validField) error {
	tagValue := field.TagValue
	splits := strings.Split(tagValue, ",")
	low, _ := strconv.Atoi(splits[0])
	high, _ := strconv.Atoi(splits[1])
	switch field.FieldKind {
	case reflect.Struct, reflect.Ptr, reflect.Interface:
	case reflect.Slice:
		for i, v := range values {
			if v < splits[0] || v > splits[1] {
				return fmt.Errorf("%s[%d] '%s' is not between %s and %s", fieldName, i, v, splits[0], splits[1])
			}
		}
	default:
		for _, v := range values {
			val, _ := strconv.Atoi(v)
			if val < low || val > high {
				return fmt.Errorf("%s is not between %s and %s", fieldName, splits[0], splits[1])
			}
		}
	}

	return nil
}

func lenValidFunc(values []string, fieldName string, field *validField) error {
	tagValue := field.TagValue
	splits := strings.Split(tagValue, ",")
	switch field.FieldKind {
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

func itemLenValidFunc(values []string, fieldName string, field *validField) error {
	splits := strings.Split(field.TagValue, ",")
	switch field.FieldKind {
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
