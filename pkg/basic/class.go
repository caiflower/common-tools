package basic

//**************************************
// 类
//**************************************

import (
	"fmt"
	"reflect"
	"strings"
	"sync"
)

// Class 声明类
type Class struct {
	mu      sync.RWMutex       //锁
	cls     interface{}        //目标对象
	name    string             //类名称
	pkgPath string             // 包名
	methods map[string]*Method //方法集合
}

// NewClass 实例化
func NewClass(cls interface{}) *Class {
	return createClass(cls)
}

// GetName 获得类名称
func (c *Class) GetName() string {
	return c.name
}

// GetMethod 根据方法名称获得方法，如：pkg.funcname
func (c *Class) GetMethod(methodName string) *Method {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.methods[methodName]
}

// GetAllMethod 获得此类中所有方法
func (c *Class) GetAllMethod() []*Method {
	c.mu.RLock()
	defer c.mu.RUnlock()
	methods := make([]*Method, 0)
	for _, v := range c.methods {
		methods = append(methods, v)
	}
	return methods
}

// GetPkgPath 获取包的路径
func (c *Class) GetPkgPath() string {
	return c.pkgPath
}

// **************************************
// 私有方法
// **************************************
func createClass(cls interface{}) *Class {
	kind := reflect.TypeOf(cls).Kind()

	if kind != reflect.Pointer && kind != reflect.Interface {
		panic(fmt.Sprintf("CrateClass failed. Class must be pointer or interface. "))
	}

	// 实例化
	obj := &Class{
		methods: make(map[string]*Method),
		cls:     cls,
		name:    GetClassName(cls),
		pkgPath: reflect.TypeOf(cls).Elem().PkgPath(),
	}

	// 遍历所有方法并实例化并缓存起来
	ele := reflect.TypeOf(cls)
	if ele.NumMethod() == 0 {
		return obj
	}

	for i := 0; i < ele.NumMethod(); i++ {
		obj.addMethod(NewMethod(obj, ele.Method(i)))
	}
	return obj
}

func (c *Class) addMethod(method *Method) {
	if method == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.methods[method.GetName()] = method
}

func GetClassName(v interface{}) string {
	kind := reflect.TypeOf(v).Kind()

	if kind == reflect.Pointer || kind == reflect.Interface {
		return strings.Replace(reflect.TypeOf(v).String(), "*", "", 1)
	} else {
		panic("Class must be interface or ptr. ")
	}
}
