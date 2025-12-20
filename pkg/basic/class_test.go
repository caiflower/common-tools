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
	"strconv"
	"testing"
)

type TestMethodStruct struct {
	Args string
}

func (tms *TestMethodStruct) TestMethod(args1, args2 string) (ret1, ret2 string) {
	return args1, args2
}

func (tms *TestMethodStruct) TestMethod1() string {
	return tms.Args + "testMethod2Ret"
}

type TestArgs struct {
	Args string
}

func (tms *TestMethodStruct) TestMethod2(m TestArgs, m1 *TestArgs) (TestArgs, *TestArgs) {
	return m, m1
}

func TestNewMethod(t *testing.T) {
	v := &TestMethodStruct{
		Args: "args",
	}

	class := createClass(v)

	fmt.Printf("className = %s\n", class.GetName())
	for _, v := range class.GetAllMethod() {
		fmt.Printf("classMethod = %v\n", v)
	}

	method := class.GetMethod("basic.TestMethod")
	var values []reflect.Value
	args := method.GetArgs()
	for i := 0; i < len(args); i++ {
		arg := args[i]
		kind := arg.Kind()
		if kind == reflect.Ptr {
			values = append(values, reflect.New(arg.Elem())) // 参数是指针
		} else if kind == reflect.Struct {
			values = append(values, reflect.New(arg)) // 参数不是指针
		} else {
			values = append(values, reflect.New(arg).Elem())
		}
		values[i].SetString("test" + strconv.Itoa(i))
	}
	invoke := method.Invoke(values)
	fmt.Printf("invoke ret = %s %s\n", invoke[0].String(), invoke[1].String())

	method = class.GetMethod("basic.TestMethod1")
	invoke = method.Invoke(nil)
	fmt.Printf("invoke ret = %s\n", invoke[0].String())

	method = class.GetMethod("basic.TestMethod2")
	values = values[:0]
	arg := method.GetArgs()[0]
	values = append(values, reflect.New(arg).Elem()) // 参数不是指针
	testArgs := TestArgs{Args: "test"}
	values[0].Set(reflect.ValueOf(testArgs))

	arg = method.GetArgs()[1]
	values = append(values, reflect.New(arg).Elem()) // 参数不是指针
	testArgs1 := &TestArgs{Args: "test1"}
	values[1].Set(reflect.ValueOf(testArgs1))

	invoke = method.Invoke(values)
	fmt.Printf("invoke ret = %s %s\n", invoke[0].Interface(), invoke[1].Interface())
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
