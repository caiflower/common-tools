package bean

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"sync"
)

const (
	AutoWrite = "autowrite"
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

func writeBean(name string, bean interface{}) {
	beanType := reflect.TypeOf(bean)
	beanValue := reflect.Indirect(reflect.ValueOf(bean))

	for i := 0; i < beanValue.NumField(); i++ {
		field := beanValue.Field(i)
		fieldType := beanType.Elem().Field(i)

		if !needAutoWrite(string(fieldType.Tag)) {
			continue
		}

		if field.Kind() != reflect.Pointer && field.Kind() != reflect.Interface {
			panic(fmt.Sprintf("Ioc failed. Only can autowrite pointer or interface. Bean=%s DependentBean=%s. ", name, fieldType.Name))
		}

		// 如果字段不是nil忽略
		if !field.IsNil() {
			continue
		}

		// 非公开值
		if !field.CanSet() {
			panic(fmt.Sprintf("Ioc failed. Field can't set. Bean=%s DependentBean=%s. ", name, fieldType.Name))
		}

		// 1. 根据autowrite的value获取bean
		// 2. 根据名称获取bean
		// 3. 根据package.StructName获取bean
		// 4. 根据package.InterfaceName获取bean
		fieldBeanName := fieldType.Tag.Get(AutoWrite)
		var fieldBean interface{}

		if fieldBeanName != "" {
			fieldBean = GetBean(fieldBeanName)
		}
		if fieldBean == nil {
			fieldBean = GetBean(fieldType.Name)
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
			panic(fmt.Sprintf("Ioc failed. Field autowrite failed. Bean=%s DependentBean=%s. ", name, fieldType.Name))
		}

		// 递归
		if field.Kind() == reflect.Ptr {
			writeBean(fieldType.Name, field.Interface())
		}
	}
}

func needAutoWrite(tag string) bool {
	return regexp.MustCompile(`\b` + AutoWrite + `\b`).Match([]byte(tag))
}

func AddBean(bean interface{}) {
	if reflect.TypeOf(bean).Kind() != reflect.Interface && reflect.TypeOf(bean).Kind() != reflect.Pointer {
		panic(fmt.Sprintf("Add bean failed. Bean kind must be interface or ptr. "))
	}

	var name string
	kind := reflect.TypeOf(bean).Kind()
	if kind == reflect.Pointer || kind == reflect.Interface {
		name = strings.Replace(reflect.TypeOf(bean).String(), "*", "", 1)
	} else {
		panic("Class must be interface or ptr. ")
	}

	SetBean(name, bean)
}

func SetBean(name string, bean interface{}) {
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
