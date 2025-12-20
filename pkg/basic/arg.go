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

package basic

import (
	"fmt"
	"reflect"
)

// ArgInfo 参数信息，包含类型和标签
type ArgInfo struct {
	Type      reflect.Type      //参数类型
	FieldName string            //参数名
	FieldType string            //类型名
	Tags      map[string]string //参数的所有标签
	Fields    map[int]*ArgInfo  //当参数是结构体时，存储字段信息，int对应structVal.Field(index)中的index

	// 索引映射，用于O(1)复杂度查找字段
	tagIndex      map[string]map[string][]int // tagName -> tagValue -> []fieldIndex (支持多个相同标签的字段)
	fieldIndexMap map[string][]int            // fieldName -> []fieldIndex (支持嵌套字段的索引路径)
}

// SetFieldValueUsingIndex setValue with argInfo
func SetFieldValueUsingIndex(structVal reflect.Value, fieldName string, value interface{}, argInfo *ArgInfo) (err error) {
	if structVal.Kind() != reflect.Struct && structVal.Kind() != reflect.Ptr {
		return fmt.Errorf("value is not a struct or pointer")
	}

	if structVal.Kind() == reflect.Ptr {
		structVal = structVal.Elem()
	}

	if indexPath, exists := argInfo.fieldIndexMap[fieldName]; exists {
		for _, index := range indexPath {
			nextValue := structVal.Field(index)
			if !nextValue.CanSet() {
				continue
			}

			t := nextValue.Type()
			if nextValue.Kind() == reflect.Struct {
				if nextValue.IsZero() {
					newStruct := reflect.New(t).Elem()
					nextValue.Set(newStruct)
				}
				err = SetFieldValueUsingIndex(nextValue, fieldName, value, argInfo.Fields[index])
				continue
			} else if nextValue.Kind() == reflect.Ptr {
				if nextValue.IsZero() {
					newStruct := reflect.New(t.Elem())
					nextValue.Set(newStruct)
				}
				err = SetFieldValueUsingIndex(nextValue, fieldName, value, argInfo.Fields[index])
				continue
			}

			if nextValue.CanSet() {
				val := reflect.ValueOf(value)
				if val.Type().ConvertibleTo(t) {
					nextValue.Set(val.Convert(t))
					return nil
				}
				err = fmt.Errorf("field %s: type %v cannot be converted to %v", fieldName, val.Type(), t)
			}

			err = fmt.Errorf("field %s cannot be set", fieldName)
		}
	}

	return
}
