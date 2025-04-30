package bean

import (
	"bytes"
	"fmt"
	"github.com/caiflower/common-tools/pkg/tools/jsonpath"
	"reflect"
	"regexp"
	"strings"
	"sync"
)

const (
	AutoWrite = "autowrite"
	Autowired = "autowired"

	ConditionalOnProperty = "conditional_on_property"
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
		fieldBeanName := fieldType.Tag.Get(AutoWrite)
		if fieldBeanName == "" {
			fieldBeanName = fieldType.Tag.Get(Autowired)
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
				name := field.Type().PkgPath() + "/" + field.Type().String()
				fieldBean = GetBean(name)
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
				pkgPath := fieldType.Type.Elem().PkgPath()
				splits := strings.Split(pkgPath, "/")
				path := strings.TrimSuffix(pkgPath, splits[len(splits)-1])
				name := path + strings.Replace(fieldType.Type.String(), "*", "", 1)
				fieldBean = GetBean(name)
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
	conditionalOnProperty := tag.Get(ConditionalOnProperty)
	if strings.HasPrefix(conditionalOnProperty, "default.") {
		splits := strings.Split(strings.TrimPrefix(conditionalOnProperty, "default."), "=")
		if len(splits) != 2 {
			panic("not supported conditionalOnProperty: " + conditionalOnProperty)
		}
		path := jsonpath.New("tagFilter")
		t := "{." + splits[0] + "}"
		if err := path.Parse(t); err != nil {
			panic(fmt.Sprintf("autowired failed. parse '%s' failed. error: %v", ConditionalOnProperty, err))
		}
		buf := new(bytes.Buffer)
		err := path.Execute(buf, GetBean("default"))
		if err != nil {
			panic(fmt.Sprintf("autowired failed. exec '%s' failed. error: %v", ConditionalOnProperty, err))
		}

		if buf.String() != splits[1] {
			return false
		}
	}

	return regexp.MustCompile(`\b`+AutoWrite+`\b`).Match([]byte(tag)) || regexp.MustCompile(`\b`+Autowired+`\b`).Match([]byte(tag))
}

func AddBean(bean interface{}) {
	if reflect.TypeOf(bean).Kind() != reflect.Interface && reflect.TypeOf(bean).Kind() != reflect.Ptr {
		panic(fmt.Sprintf("Add bean failed. Bean kind must be interface or ptr. "))
	}

	var name string
	kind := reflect.TypeOf(bean).Kind()
	if kind == reflect.Ptr || kind == reflect.Interface {
		pkgPath := reflect.TypeOf(bean).Elem().PkgPath()
		splits := strings.Split(pkgPath, "/")
		path := strings.TrimSuffix(pkgPath, splits[len(splits)-1])
		name = path + strings.Replace(reflect.TypeOf(bean).String(), "*", "", 1)
	} else {
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
