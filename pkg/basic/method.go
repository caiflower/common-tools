package basic

import (
	"fmt"
	"github.com/caiflower/common-tools/pkg/tools"
	"reflect"
	"runtime"
	"strings"
)

type Method struct {
	pkgName string         //包名
	name    string         //方法名称
	path    string         //方法路径
	class   *Class         //所属类，当方法不是某类中的方法时，则所属类是空的，只有类实现的方法，此参数才有值并有意义
	funC    interface{}    //目标函数
	args    []reflect.Type //入参类型
	rets    []reflect.Type //出参类型
}

func NewMethod(cls *Class, v interface{}) *Method {
	var pkgName, name, path string
	var args, rets []reflect.Type
	var funC interface{}

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
			args = append(args, methodType.In(i))
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
	} else if reflect.TypeOf(v).Kind() == reflect.Func {
		panic("Func not iml. ")
	} else {
		panic(fmt.Sprintf("NewMethod not support type %v. ", reflect.TypeOf(v).Kind()))
	}

	return &Method{
		class:   cls,
		pkgName: pkgName,
		name:    name,
		path:    path,
		args:    args,
		rets:    rets,
		funC:    funC,
	}
}

func getNameAndPkgNameAndPath(fullPath string) (name, pkgName, path string) {
	name = tools.RegReplace(fullPath, ".*/", "")
	path = strings.Replace(fullPath, name, "", 1)
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

// Invoke 反射调用方法
func (m *Method) Invoke(args []reflect.Value) []reflect.Value {
	if m.HasArgs() == false {
		return reflect.ValueOf(m.funC).Call(nil)
	}
	if m.GetArgs()[0].Kind() == reflect.Struct && len(args) > 0 {
		args[0] = args[0].Elem()
	}
	return reflect.ValueOf(m.funC).Call(args)
}
