package basic

import (
	"fmt"
	"github.com/caiflower/common-tools/pkg/queue"
	"reflect"
	"testing"
)

type MyClass struct {
}

func TestGetClassName(t *testing.T) {
	className := GetClassName(queue.NewDelayQueue())
	fmt.Printf("className = %v\n", className)
}

type TestMethodStruct struct{}

func (tms *TestMethodStruct) TestMethod(args1, args2 string) (ret1, ret2 string) {
	return "", ""
}

func (tms *TestMethodStruct) TestMethod1() string {
	return "testMethod2Ret"
}

func TestNewMethod(t *testing.T) {
	v := &TestMethodStruct{}

	class := createClass(v)

	fmt.Printf("className = %s\n", class.GetName())
	for _, v := range class.GetAllMethod() {
		fmt.Printf("classMethod = %v\n", v)
	}

	method := class.GetMethod("basic.TestMethod1")
	invoke := method.Invoke(nil)
	fmt.Printf("invoke ret = %s\n", invoke[0].String())
}

type MyStruct struct{}

func (m *MyStruct) MyMethod(a int, b string) {}

func Test(m *testing.T) {
	t := reflect.TypeOf(&MyStruct{})
	method := t.Method(0)
	methodType := method.Type
	firstParamType := methodType.In(0)
	s := firstParamType.String()
	fmt.Println(s, firstParamType.Kind()) // 输出：int
}
