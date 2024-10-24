package global

import (
	"fmt"
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
}

type TestConfig1 struct {
	Name1 string  `yaml:"name" default:"test1"`
	Age   int     `yaml:"age" default:"30"`
	Age2  int8    `yaml:"age2" default:"1"`
	Money float64 `yaml:"money" default:"1.231321"`
}

func TestLoadConfig(t *testing.T) {
	config := &TestConfig{}
	err := LoadConfig("./test.yaml", config)
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
