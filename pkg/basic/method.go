package basic

import (
	"fmt"
	"reflect"
	"runtime"
	"strings"

	"github.com/caiflower/common-tools/pkg/tools"
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
		fullPath := runtime.FuncForPC(reflect.ValueOf(v).Pointer()).Name()

		//说明此方法属于某个类，去掉类信息
		if tools.MatchReg(fullPath, `[.]\([^()]*\)`) {
			fullPath = tools.RegReplace(fullPath, `[.]\([^()]*\)`, "")
		}

		name, pkgName, path = getNameAndPkgNameAndPath(fullPath)

		fType := reflect.TypeOf(v)
		for i := 0; i < fType.NumIn(); i++ {
			args = append(args, fType.In(i))
		}
		for i := 0; i < fType.NumOut(); i++ {
			rets = append(rets, fType.Out(i))
		}

		funC = v
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
