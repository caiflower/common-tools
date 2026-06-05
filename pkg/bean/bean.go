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

package bean

import (
	"bytes"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"sync"

	"github.com/caiflower/common-tools/pkg/tools/jsonpath"
)

const (
	AutoWrite = "autowrite"
	Autowired = "autowired"

	ConditionalOnProperty = "conditional_on_property"
)

var (
	autoWriteRegex = regexp.MustCompile(`\b` + AutoWrite + `\b`)
	autowiredRegex = regexp.MustCompile(`\b` + Autowired + `\b`)
)

var beanContext = beanManager{
	beanMap: make(map[string]interface{}),
}

type beanManager struct {
	beanMap map[string]interface{}
	lock    sync.RWMutex
}

func Ioc() {
	beanContext.lock.RLock()
	defer beanContext.lock.RUnlock()
	for k, v := range beanContext.beanMap {
		writeBean(k, v)
	}
}

func writeBean(beanName string, bean interface{}) {
	beanType := reflect.TypeOf(bean)
	beanValue := reflect.Indirect(reflect.ValueOf(bean))

	for i := 0; i < beanValue.NumField(); i++ {
		field := beanValue.Field(i)
		fieldType := beanType.Elem().Field(i)

		if !needAutoWrite(fieldType.Tag) {
			continue
		}

		if field.Kind() != reflect.Ptr && field.Kind() != reflect.Interface {
			panic(fmt.Sprintf("Ioc failed. Only can autowrite pointer or interface. Bean=%s DependentBean=%s. ", beanName, fieldType.Name))
		}

		// 如果字段不是nil忽略
		if !field.IsNil() {
			continue
		}

		// 非公开值
		if !field.CanSet() {
			panic(fmt.Sprintf("Ioc failed. Field can't set. Bean=%s DependentBean=%s. ", beanName, fieldType.Name))
		}

		// 1. 根据autowrite的value获取bean
		// 2. 根据名称获取bean
		// 3. 根据package.StructName获取bean
		// 4. 根据package.InterfaceName获取bean
		fieldBeanName := getBeanNameFromTag(fieldType.Tag.Get(AutoWrite))
		if fieldBeanName == "" {
			fieldBeanName = getBeanNameFromTag(fieldType.Tag.Get(Autowired))
		}
		var fieldBean interface{}

		if fieldBeanName != "" {
			fieldBean = GetBean(fieldBeanName)
		}
		if fieldBean == nil {
			fieldBean = GetBean(fieldType.Name)
		}
		if fieldBean == nil {
			switch field.Kind() {
			case reflect.Interface:
				fieldBean = GetBean(getBeanNameFromType(field.Type()))
				if fieldBean == nil {
					beanContext.lock.RLock()
					defer beanContext.lock.RUnlock()
					for _, v := range beanContext.beanMap {
						if reflect.TypeOf(v).AssignableTo(field.Type()) {
							fieldBean = v
						}
					}
				}
			case reflect.Ptr:
				fieldBean = GetBean(getBeanNameFromType(fieldType.Type))
			default:
				panic("unhandled default case")
			}
		}
		if fieldBean == nil {
			fieldBean = GetBean(strings.Replace(fieldType.Type.String(), "*", "", 1))
		}
		if fieldBean == nil {
			fieldBean = GetBean(strings.Replace(fieldType.Type.String(), fieldType.Type.Name(), fieldType.Name, 1))
		}

		if fieldBean != nil {
			field.Set(reflect.ValueOf(fieldBean))
		} else {
			panic(fmt.Sprintf("Ioc failed. Field autowrite failed. Bean=%s DependentBean=%s. ", beanName, fieldType.Name))
		}

		// 递归
		if field.Kind() == reflect.Ptr {
			writeBean(fieldType.Name, field.Interface())
		}
	}
}

func needAutoWrite(tag reflect.StructTag) bool {
	// 判断是否有 autowired/autowrite 标签
	hasAutoTag := autoWriteRegex.Match([]byte(tag)) || autowiredRegex.Match([]byte(tag))
	if !hasAutoTag {
		return false
	}

	// 1. 先从独立的 conditional_on_property 标签读取
	conditionalOnProperty := tag.Get(ConditionalOnProperty)

	// 2. 再从 autowired/autowrite 标签值中读取 |conditional_on_property:xxx=yyy
	if conditionalOnProperty == "" {
		tagValue := tag.Get(AutoWrite)
		if tagValue == "" {
			tagValue = tag.Get(Autowired)
		}
		conditionalOnProperty = parseConditionalFromTagValue(tagValue)
	}

	if conditionalOnProperty != "" {
		return checkConditionalOnProperty(conditionalOnProperty)
	}

	return true
}

// parseConditionalFromTagValue 从 autowired/autowrite 标签值中解析 |conditional_on_property:xxx=yyy
// 返回条件部分（不含前缀），如 "default.feature.enabled=true"
func parseConditionalFromTagValue(tagValue string) string {
	prefix := "|" + ConditionalOnProperty + ":"
	idx := strings.Index(tagValue, prefix)
	if idx < 0 {
		return ""
	}
	return tagValue[idx+len(prefix):]
}

// getBeanNameFromTag 从 autowired/autowrite 标签值中提取 bean 名称，剥离 | 后的条件部分
func getBeanNameFromTag(tagValue string) string {
	idx := strings.Index(tagValue, "|")
	if idx >= 0 {
		return tagValue[:idx]
	}
	return tagValue
}

// checkConditionalOnProperty 检查条件是否满足，不满足返回 false
func checkConditionalOnProperty(conditionalOnProperty string) bool {
	splits := strings.SplitN(conditionalOnProperty, "=", 2)
	if len(splits) != 2 {
		panic("not supported conditionalOnProperty: " + conditionalOnProperty)
	}

	// 解析 beanName 和 jsonPath
	// 格式: beanName.jsonPath=value 或 jsonPath=value（默认从 default bean 读取）
	pathPart := splits[0]
	var beanName string
	var jsonPath string

	dotIndex := strings.Index(pathPart, ".")
	if dotIndex > 0 {
		beanName = pathPart[:dotIndex]
		jsonPath = pathPart[dotIndex+1:]
	} else {
		beanName = "default"
		jsonPath = pathPart
	}

	path := jsonpath.New("tagFilter")
	t := "{." + jsonPath + "}"
	if err := path.Parse(t); err != nil {
		panic(fmt.Sprintf("autowired failed. parse '%s' failed. error: %v", ConditionalOnProperty, err))
	}
	buf := new(bytes.Buffer)
	err := path.Execute(buf, GetBean(beanName))
	if err != nil {
		panic(fmt.Sprintf("autowired failed. exec '%s' failed. error: %v", ConditionalOnProperty, err))
	}

	return buf.String() == splits[1]
}

func AddBean(bean interface{}) {
	if reflect.TypeOf(bean).Kind() != reflect.Interface && reflect.TypeOf(bean).Kind() != reflect.Ptr {
		panic("Add bean failed. Bean kind must be interface or ptr. ")
	}

	name := getBeanNameFromValue(bean)
	if name == "" {
		panic("Class must be interface or ptr. ")
	}

	SetBean(name, bean)
}

func SetBeanOverwrite(name string, bean interface{}) {
	if bean == nil {
		panic("Bean error. Bean can't be nil.")
	}

	beanContext.lock.Lock()
	defer beanContext.lock.Unlock()

	beanContext.beanMap[name] = bean
}

func SetBean(name string, bean interface{}) {
	if bean == nil {
		panic("Bean error. Bean can't be nil.")
	}
	beanContext.lock.Lock()
	defer beanContext.lock.Unlock()

	if beanContext.beanMap[name] != nil {
		panic(fmt.Sprintf("Bean conflict. Bean %v has already exist. ", name))
	}

	beanContext.beanMap[name] = bean
}

func GetBean(name string) interface{} {
	beanContext.lock.RLock()
	defer beanContext.lock.RUnlock()

	return beanContext.beanMap[name]
}

// getBeanNameFromType 根据reflect.Type获取Bean名称（核心逻辑）
// 返回空字符串表示类型不是指针或接口类型
func getBeanNameFromType(typeOf reflect.Type) string {
	var pkgPath, typeName string

	switch typeOf.Kind() {
	case reflect.Interface:
		pkgPath = typeOf.PkgPath()
		typeName = typeOf.String()
	case reflect.Ptr:
		pkgPath = typeOf.Elem().PkgPath()
		typeName = strings.Replace(typeOf.String(), "*", "", 1)
	default:
		return ""
	}

	// 提取包路径前缀（去掉最后一级包名，保留/）
	if idx := strings.LastIndex(pkgPath, "/"); idx >= 0 {
		return pkgPath[:idx+1] + typeName
	}
	return typeName
}

// getBeanNameFromValue 根据bean实例获取Bean名称（非泛型版本）
// 返回空字符串表示类型不是指针或接口类型
func getBeanNameFromValue(bean interface{}) string {
	return getBeanNameFromType(reflect.TypeOf(bean))
}

func GetBeanNameFromValue(bean interface{}) string {
	return getBeanNameFromType(reflect.TypeOf(bean))
}

// GetBeanName 根据类型获取Bean名称（泛型版本）
// 返回空字符串表示类型不是指针或接口类型
func GetBeanName[T any]() string {
	var t T
	return getBeanNameFromType(reflect.TypeOf(&t).Elem())
}

// GetBeanT 泛型方式获取Bean
func GetBeanT[T any](name ...string) T {
	var zero T
	var beanName string
	if len(name) > 0 {
		beanName = name[0]
	} else {
		// 根据类型推断bean名称
		beanName = GetBeanName[T]()
		if beanName == "" {
			return zero
		}
	}

	bean := GetBean(beanName)
	if bean == nil {
		return zero
	}

	if result, ok := bean.(T); ok {
		return result
	}
	return zero
}

// RemoveBean 移除Bean
func RemoveBean(name string) {
	beanContext.lock.Lock()
	defer beanContext.lock.Unlock()
	delete(beanContext.beanMap, name)
}

// ClearBeans 清空所有Bean
func ClearBeans() {
	beanContext.lock.Lock()
	defer beanContext.lock.Unlock()
	beanContext.beanMap = make(map[string]interface{})
}

// HasBean 检查Bean是否存在
func HasBean(name string) bool {
	beanContext.lock.RLock()
	defer beanContext.lock.RUnlock()
	return beanContext.beanMap[name] != nil
}

// GetAllBeans 获取所有Bean名称
func GetAllBeans() []string {
	beanContext.lock.RLock()
	defer beanContext.lock.RUnlock()

	names := make([]string, 0, len(beanContext.beanMap))
	for name := range beanContext.beanMap {
		names = append(names, name)
	}
	return names
}
