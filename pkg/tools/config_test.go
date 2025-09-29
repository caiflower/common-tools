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

 package tools

import (
	"fmt"
	"os"
	"reflect"
	"testing"
)

type TestConfig struct {
	Name   string       `yaml:"name" default:"test"`
	Age    int          `yaml:"age" default:"30"`
	Age2   int8         `yaml:"age2" default:"1"`
	Money  float64      `yaml:"money" default:"1.231321"`
	Test   *TestConfig1 `yaml:"test1"`
	Test2  TestConfig1  `yaml:"test2"`
	PtrInt *int         `yaml:"ptrInt" default:"1"`
	Str1   *string      `yaml:"str1" default:"test"`
	Float1 *float64     `yaml:"str2" default:"1.24"`
	B      *bool        `yaml:"b" default:"true"`
	d      string       `yaml:"d" default:"test"` // test case: no can set
}

type TestConfig1 struct {
	Name1 string  `yaml:"name" default:"test1"`
	Age   int     `yaml:"age" default:"30"`
	Age2  int8    `yaml:"age2" default:"1"`
	Money float64 `yaml:"money" default:"1.231321"`
}

func TestLoadConfig(t *testing.T) {
	config := &TestConfig{}
	err := LoadConfig(os.Getenv("HOME")+"/common-tools/config/test.yaml", config)
	if err != nil {
		panic(err)
	}

	fmt.Printf("%+v\n", config)
}

func TestLoadConfig2(t *testing.T) {
	tc := TestConfig{}
	rt := reflect.TypeOf(tc)

	// 获取 TestConfig 结构体中的 Test 字段
	testField, _ := rt.FieldByName("Test")

	// 获取 TestConfig1 结构体
	testConfig1Value := reflect.New(testField.Type.Elem()).Elem()

	// 获取 TestConfig1 结构体中的 Name1 字段
	testConfig1Field, _ := testConfig1Value.Type().FieldByName("Name1")

	tagValue := testConfig1Field.Tag.Get("yaml")
	fmt.Println("Tag for Name1 in TestConfig1:", tagValue)
}
