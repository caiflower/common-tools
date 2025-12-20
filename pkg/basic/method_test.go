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
	"testing"

	"github.com/stretchr/testify/assert"
)

// 测试用的结构体
type DoRequestReq struct {
	Input string `verf:"required"`
	Name  string `json:"name" verf:"optional"`
	Age   int    `json:"age"`
}

type ComplexStruct struct {
	Request  DoRequestReq  `verf:"nested"`
	Request1 *DoRequestReq `verf:"nested"`
	DoRequestReq
	Token string `auth:"bearer"`
}

// 测试函数：带结构体参数
func SampleMethodWithStruct(req DoRequestReq, test1 ComplexStruct) string {
	return req.Input
}

// 测试函数：带多个参数
func SampleMethodWithMultipleParams(name string, req DoRequestReq) string {
	return name + ":" + req.Input
}

// 测试函数：带复杂结构体
func SampleMethodWithComplexStruct(data ComplexStruct) string {
	return data.Request.Input + ":" + data.Token
}

// 测试函数：无参数
func SampleMethodNoParams() string {
	return "no params"
}

// 测试带多个参数的方法
func TestMethodWithMultipleParams(t *testing.T) {
	method := NewMethod(nil, SampleMethodWithMultipleParams)

	if method == nil {
		t.Fatal("NewMethod returned nil")
	}

	// 验证参数数量
	argsInfo := method.GetArgsInfo()
	if len(argsInfo) != 2 {
		t.Errorf("Expected 2 args info, got %d", len(argsInfo))
	}

	// 验证第一个参数（string类型）
	argInfo := method.GetArgInfo(0)
	if argInfo == nil {
		t.Fatal("GetArgInfo(0) returned nil")
	}
	if argInfo.Type.Kind() != reflect.String {
		t.Errorf("Expected first arg to be string, got %v", argInfo.Type.Kind())
	}

	// 验证第二个参数（结构体类型）
	argInfo = method.GetArgInfo(1)
	if argInfo == nil {
		t.Fatal("GetArgInfo(1) returned nil")
	}
	if argInfo.Type.Name() != "DoRequestReq" {
		t.Errorf("Expected second arg to be DoRequestReq, got %s", argInfo.Type.Name())
	}
}

// 测试无参数方法
func TestMethodNoParams(t *testing.T) {
	method := NewMethod(nil, SampleMethodNoParams)

	if method == nil {
		t.Fatal("NewMethod returned nil")
	}

	// 验证无参数
	if method.HasArgs() != false {
		t.Error("Expected method to have no args")
	}

	argsInfo := method.GetArgsInfo()
	if len(argsInfo) != 0 {
		t.Errorf("Expected 0 args info, got %d", len(argsInfo))
	}
}

// TestSetArgInfo 测试使用TagIndex在O(1)时间复杂度下为参数赋值
func TestSetArgInfo(t *testing.T) {
	method := NewMethod(nil, SampleMethodWithComplexStruct)

	// map - 注意字段名要匹配结构体中的字段名
	params := make(map[string]interface{})
	params["Name"] = "testName"   // DoRequestReq.Name 字段
	params["Age"] = 12            // DoRequestReq.Age 字段
	params["Input"] = "testInput" // DoRequestReq.Input 字段
	params["Token"] = "testToken" // ComplexStruct.Token 字段

	argInfo := method.GetArgInfo(0)
	if argInfo == nil {
		t.Fatal("GetArgInfo(0) returned nil")
	}

	// 创建ComplexStruct的实例
	argValue := reflect.New(argInfo.Type).Elem()

	// 使用TagIndex进行O(1)字段查找和赋值
	for fieldName, value := range params {
		// 找到字段，使用反射设置值
		if err := SetFieldValueUsingIndex(argValue, fieldName, value, argInfo); err != nil {
			t.Errorf("Failed to set field %s: %v", fieldName, err)
		}
	}

	// 验证设置的值是否正确
	complexStruct := argValue.Interface().(ComplexStruct)

	assert.Equal(t, "testName", complexStruct.Request.Name, fmt.Sprintf("Expected field name 'testName', got '%s'", complexStruct.Request.Name))
	assert.Equal(t, "testName", complexStruct.Request1.Name, fmt.Sprintf("Expected field name 'testName', got '%s'", complexStruct.Request1.Name))
	assert.Equal(t, "testName", complexStruct.DoRequestReq.Name, fmt.Sprintf("Expected field name 'testName', got '%s'", complexStruct.DoRequestReq.Name))
	assert.Equal(t, 12, complexStruct.Request.Age, fmt.Sprintf("Expected age to be 12, got %d'", complexStruct.Request.Age))
	assert.Equal(t, 12, complexStruct.Request1.Age, fmt.Sprintf("Expected age to be 12, got %d'", complexStruct.Request1.Age))
	assert.Equal(t, 12, complexStruct.DoRequestReq.Age, fmt.Sprintf("Expected age to be 12, got %d'", complexStruct.DoRequestReq.Age))
	assert.Equal(t, "testInput", complexStruct.Request.Input, fmt.Sprintf("Expected Input to be 12, got %d'", complexStruct.Request.Age))
	assert.Equal(t, "testInput", complexStruct.Request1.Input, fmt.Sprintf("Expected Input to be 12, got %d'", complexStruct.Request1.Age))
	assert.Equal(t, "testInput", complexStruct.DoRequestReq.Input, fmt.Sprintf("Expected Input to be 12, got %d'", complexStruct.DoRequestReq.Age))
	assert.Equal(t, "testToken", complexStruct.Token, fmt.Sprintf("Expected testToken to be testToken, got %s'", complexStruct.Token))
}
