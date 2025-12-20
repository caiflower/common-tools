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
	"runtime"
	"strings"

	"github.com/caiflower/common-tools/pkg/tools"
)

type Method struct {
	pkgName  string         //包名
	name     string         //方法名称
	path     string         //方法路径
	class    *Class         //所属类，当方法不是某类中的方法时，则所属类是空的，只有类实现的方法，此参数才有值并有意义
	funC     interface{}    //目标函数
	args     []reflect.Type //入参类型
	rets     []reflect.Type //出参类型
	argsInfo []ArgInfo      //参数详细信息，包含标签
}

func NewMethod(cls *Class, v interface{}) *Method {
	var pkgName, name, path string
	var args, rets []reflect.Type
	var funC interface{}
	var argsInfo []ArgInfo

	if method, ok := v.(reflect.Method); ok {

		// 获取方法的全路径
		fullPath := runtime.FuncForPC(method.Func.Pointer()).Name()

		// 去除类相关的路径(*.TestMethodStruct)
		fullPath = tools.RegReplace(fullPath, `[.]\([^()]*\)`, "")

		name, pkgName, path = getNameAndPkgNameAndPath(fullPath)

		if cls == nil {
			panic(fmt.Sprintf("Method %v class must be not nil. ", name))
		}

		methodType := method.Type

		// i=0时，该参数是对象指针，所以忽略
		for i := 1; i < methodType.NumIn(); i++ {
			argType := methodType.In(i)
			args = append(args, argType)
		}

		for i := 0; i < methodType.NumOut(); i++ {
			rets = append(rets, methodType.Out(i))
		}

		replace := tools.RegReplace(name, `[^.]*\.`, "")
		_tm := reflect.ValueOf(cls.cls).MethodByName(replace)
		if _tm.IsValid() == false {
			return nil
		}
		funC = _tm.Interface()

		// 初始化argsInfo
		argsInfo = make([]ArgInfo, 0, len(args))
		for i := 1; i < methodType.NumIn(); i++ {
			argType := methodType.In(i)
			argInfo := extractArgTags(argType)
			argsInfo = append(argsInfo, argInfo)
		}
	} else if reflect.TypeOf(v).Kind() == reflect.Func {
		fullPath := runtime.FuncForPC(reflect.ValueOf(v).Pointer()).Name()

		//说明此方法属于某个类，去掉类信息
		if tools.MatchReg(fullPath, `[.]\([^()]*\)`) {
			fullPath = tools.RegReplace(fullPath, `[.]\([^()]*\)`, "")
		}

		name, pkgName, path = getNameAndPkgNameAndPath(fullPath)

		fType := reflect.TypeOf(v)
		for i := 0; i < fType.NumIn(); i++ {
			argType := fType.In(i)
			args = append(args, argType)
		}
		for i := 0; i < fType.NumOut(); i++ {
			rets = append(rets, fType.Out(i))
		}

		funC = v

		// 初始化argsInfo
		argsInfo = make([]ArgInfo, 0, len(args))
		for i := 0; i < fType.NumIn(); i++ {
			argType := fType.In(i)
			argInfo := extractArgTags(argType)
			argsInfo = append(argsInfo, argInfo)
		}
	} else {
		panic(fmt.Sprintf("NewMethod not support type %v. ", reflect.TypeOf(v).Kind()))
	}

	// 构建索引映射
	tagIndex := make(map[string]map[string][]int)
	fieldNameIndex := make(map[string][]int)
	buildIndexMaps(argsInfo, tagIndex, fieldNameIndex)

	return &Method{
		class:    cls,
		pkgName:  pkgName,
		name:     name,
		path:     path,
		args:     args,
		rets:     rets,
		funC:     funC,
		argsInfo: argsInfo,
	}
}

func getNameAndPkgNameAndPath(fullPath string) (name, pkgName, path string) {
	name = tools.RegReplace(fullPath, ".*/", "")
	path = strings.Replace(fullPath, name, "", 1)
	if len(path) > 0 {
		path = path[:len(path)-1]
	}
	pkgName = tools.RegReplace(name, `[.].*`, "")

	return
}

func (m *Method) GetName() string {
	return m.name
}

func (m *Method) GetPkgName() string {
	return m.pkgName
}

func (m *Method) GetPath() string {
	return m.path
}

func (m *Method) GetClass() *Class {
	return m.class
}

func (m *Method) HasArgs() bool {
	return len(m.args) > 0
}

func (m *Method) GetArgs() []reflect.Type {
	return m.args
}

func (m *Method) GetRets() []reflect.Type {
	return m.rets
}

func (m *Method) HasRets() bool {
	return len(m.rets) > 0
}

// Invoke 反射调用方法
func (m *Method) Invoke(args []reflect.Value) []reflect.Value {
	if m.HasArgs() == false {
		return reflect.ValueOf(m.funC).Call(nil)
	}
	return reflect.ValueOf(m.funC).Call(args)
}

// extractArgTags 提取参数的标签信息
func extractArgTags(argType reflect.Type) ArgInfo {
	argInfo := ArgInfo{
		Type:          argType,
		Tags:          make(map[string]string),
		Fields:        make(map[int]*ArgInfo),
		tagIndex:      make(map[string]map[string][]int),
		fieldIndexMap: make(map[string][]int),
		//FieldTypeNameMap: make(map[string][]string),
	}

	// 如果参数是结构体，提取结构体字段的标签
	if argType.Kind() == reflect.Struct {
		extractStructFields(argType, argInfo.Fields, argInfo.Tags)
		// 构建索引
		buildArgInfoIndexes(&argInfo)
	}

	return argInfo
}

// extractStructFields 提取结构体字段的标签信息
func extractStructFields(structType reflect.Type, fieldTags map[int]*ArgInfo, argTags map[string]string) {
	for i := 0; i < structType.NumField(); i++ {
		field := structType.Field(i)
		fieldInfo := ArgInfo{
			Type:          field.Type,
			FieldName:     field.Name,
			FieldType:     field.Type.String(),
			Tags:          make(map[string]string),
			Fields:        make(map[int]*ArgInfo),
			tagIndex:      make(map[string]map[string][]int),
			fieldIndexMap: make(map[string][]int),
			//FieldTypeNameMap: make(map[string][]string),
		}

		// 提取所有标签
		if field.Tag != "" {
			// 解析struct tag，支持常见的tag格式
			tags := strings.Split(string(field.Tag), " ")
			for _, tag := range tags {
				tag = strings.TrimSpace(tag)
				if tag != "" {
					parts := strings.Split(tag, ":")
					if len(parts) == 2 {
						key := strings.TrimSpace(parts[0])
						value := strings.Trim(strings.TrimSpace(parts[1]), `"`)
						fieldInfo.Tags[key] = value
						argTags[key+":"+value] = value
					}
				}
			}
		}

		// 如果字段本身也是结构体，递归处理
		fieldType := field.Type
		if fieldType.Kind() == reflect.Ptr {
			fieldType = fieldType.Elem()
		}
		if fieldType.Kind() == reflect.Struct {
			extractStructFields(fieldType, fieldInfo.Fields, argTags)
		}

		fieldTags[i] = &fieldInfo
	}
}

// buildArgInfoIndexes 构建 ArgInfo 的索引映射
func buildArgInfoIndexes(argInfo *ArgInfo) {
	// 构建字段索引映射，从空路径开始
	buildFieldIndexMap(nil, argInfo, 0)

	// 构建标签索引映射
	buildTagIndexes(argInfo)
}

// buildTagIndexes 递归构建标签索引映射，存储字段索引
func buildTagIndexes(argInfo *ArgInfo) {
	if argInfo.Type.Kind() != reflect.Struct && argInfo.Type.Kind() != reflect.Ptr {
		return
	}

	structType := argInfo.Type
	if structType.Kind() == reflect.Ptr {
		structType = structType.Elem()
	}

	// 构建当前层级的索引
	for i := 0; i < structType.NumField(); i++ {
		fieldInfo, exists := argInfo.Fields[i]
		if !exists {
			continue
		}

		// 字段标签索引 - 支持多个相同标签的字段，使用字段索引
		for tagName, tagValue := range fieldInfo.Tags {
			if argInfo.tagIndex[tagName] == nil {
				argInfo.tagIndex[tagName] = make(map[string][]int)
			}
			// 将字段索引添加到数组中，支持多个相同标签的字段
			argInfo.tagIndex[tagName][tagValue] = append(argInfo.tagIndex[tagName][tagValue], i)
		}

		// 递归构建嵌套字段的标签索引
		if len(fieldInfo.Fields) > 0 {
			// 递归处理嵌套结构体的标签索引
			buildTagIndexes(fieldInfo)

			// 将嵌套结构体的标签索引合并到当前层级
			for tagName, tagValueMap := range fieldInfo.tagIndex {
				if argInfo.tagIndex[tagName] == nil {
					argInfo.tagIndex[tagName] = make(map[string][]int)
				}
				for tagValue := range tagValueMap {
					argInfo.tagIndex[tagName][tagValue] = append(argInfo.tagIndex[tagName][tagValue], i)
				}
			}
		}
	}
}

// buildFieldIndexMap 构建字段索引映射，记录每个字段的访问路径
func buildFieldIndexMap(parentArgInfo, argInfo *ArgInfo, parentArgIndex int) {
	if argInfo.Type.Kind() != reflect.Struct && argInfo.Type.Kind() != reflect.Ptr {
		return
	}

	structType := argInfo.Type
	if structType.Kind() == reflect.Ptr {
		structType = structType.Elem()
	}

	for i := 0; i < structType.NumField(); i++ {
		field := structType.Field(i)

		// 记录直接字段的索引路径
		if parentArgInfo != nil {
			parentArgInfo.fieldIndexMap[field.Name] = append(parentArgInfo.fieldIndexMap[field.Name], parentArgIndex)
		}
		argInfo.fieldIndexMap[field.Name] = append(argInfo.fieldIndexMap[field.Name], i)

		// 如果字段是结构体，递归处理嵌套字段
		fieldType := field.Type
		if fieldType.Kind() == reflect.Ptr {
			fieldType = fieldType.Elem()
		}
		if fieldType.Kind() == reflect.Struct {
			nestedArgInfo := argInfo.Fields[i]

			// 递归构建嵌套结构体的字段索引，传入当前字段路径作为基础路径
			buildFieldIndexMap(argInfo, nestedArgInfo, i)

			// 将嵌套字段的索引路径合并到当前层级
			for nestedFieldName, nestedFieldPath := range nestedArgInfo.fieldIndexMap {
				// 确保嵌套字段名不会覆盖直接字段名
				if _, exists := argInfo.fieldIndexMap[nestedFieldName]; !exists {
					argInfo.fieldIndexMap[nestedFieldName] = nestedFieldPath
				}
			}
		}
	}
}

// GetArgInfo 获取指定索引的参数信息
func (m *Method) GetArgInfo(argIndex int) *ArgInfo {
	if argIndex < 0 || argIndex >= len(m.argsInfo) {
		return nil
	}
	return &m.argsInfo[argIndex]
}

// GetArgsInfo 获取所有参数信息
func (m *Method) GetArgsInfo() []ArgInfo {
	return m.argsInfo
}

// buildIndexMaps 构建标签和字段名的索引映射
func buildIndexMaps(argsInfo []ArgInfo, tagIndex map[string]map[string][]int, fieldNameIndex map[string][]int) {
	for argIndex, argInfo := range argsInfo {
		// 构建参数标签索引
		for tagName, tagValue := range argInfo.Tags {
			if tagIndex[tagName] == nil {
				tagIndex[tagName] = make(map[string][]int)
			}
			tagIndex[tagName][tagValue] = append(tagIndex[tagName][tagValue], argIndex)
		}

		// 构建结构体字段标签和字段名索引
		if argInfo.Type.Kind() == reflect.Struct {
			structType := argInfo.Type
			for i := 0; i < structType.NumField(); i++ {
				field := structType.Field(i)
				if fieldInfo, exists := argInfo.Fields[i]; exists {
					// 字段名索引
					fieldNameIndex[field.Name] = append(fieldNameIndex[field.Name], argIndex)

					// 字段标签索引
					for tagName, tagValue := range fieldInfo.Tags {
						if tagIndex[tagName] == nil {
							tagIndex[tagName] = make(map[string][]int)
						}
						tagIndex[tagName][tagValue] = append(tagIndex[tagName][tagValue], argIndex)
					}
				}
			}
		}
	}
}
