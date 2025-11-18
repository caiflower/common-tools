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

package bean

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type TestAutoWrite struct {
	*TestAutoWrite1 `autowired:""`
	TestAutoWrite2  *TestAutoWrite2 `autowired:""`
}

type TestAutoWrite1 struct {
}

type TestAutoWrite2 struct {
	TestAutoWrite3 *TestAutoWrite3 `autowired:""`
}

type TestAutoWrite3 struct {
	TestAutoWrite2 *TestAutoWrite2 `autowired:""`
	TestAutoWrite4 TestAutoWrite4  `autowired:""`
}

type TestAutoWrite4 interface {
	TestNameXxx() string
}

type testAutoWrite4 struct {
}

func (t *testAutoWrite4) TestNameXxx() string {
	return "testAutoWrite4"
}

func TestIoc(t *testing.T) {
	// 清空之前的Bean，避免干扰
	ClearBeans()

	test := &TestAutoWrite{}
	test1 := &TestAutoWrite1{}
	test2 := &TestAutoWrite2{}
	test3 := &TestAutoWrite3{}
	test4 := &testAutoWrite4{}
	AddBean(test)
	AddBean(test1)
	AddBean(test2)
	AddBean(test3)
	AddBean(test4)

	Ioc()

	assert.Same(t, test.TestAutoWrite1, test1)
	assert.Same(t, test.TestAutoWrite2, test2)
	assert.Same(t, test2.TestAutoWrite3, test3)
	assert.Same(t, test3.TestAutoWrite2, test2)
	assert.Same(t, test3.TestAutoWrite4, test4)
}

// TestAddBeanPanic 测试添加非指针类型Bean时panic
func TestAddBeanPanic(t *testing.T) {
	ClearBeans()

	defer func() {
		if r := recover(); r != nil {
			assert.Contains(t, r.(string), "Bean kind must be interface or ptr")
		}
	}()

	// 应该panic
	type NotPtr struct{}
	AddBean(NotPtr{})
	t.Fatal("should panic")
}

// TestSetBeanNilPanic 测试设置nil Bean时panic
func TestSetBeanNilPanic(t *testing.T) {
	ClearBeans()

	defer func() {
		if r := recover(); r != nil {
			assert.Contains(t, r.(string), "Bean can't be nil")
		}
	}()

	SetBean("test", nil)
	t.Fatal("should panic")
}

// TestSetBeanConflict 测试Bean名称冲突
func TestSetBeanConflict(t *testing.T) {
	ClearBeans()

	test1 := &TestAutoWrite1{}
	test2 := &TestAutoWrite1{}

	SetBean("test", test1)

	defer func() {
		if r := recover(); r != nil {
			assert.Contains(t, r.(string), "Bean conflict")
		}
	}()

	// 应该panic
	SetBean("test", test2)
	t.Fatal("should panic")
}

// TestSetBeanOverwrite 测试覆盖Bean
func TestSetBeanOverwrite(t *testing.T) {
	ClearBeans()

	// 使用有字段的结构体，避免Go编译器优化导致地址相同
	type BeanWithField struct {
		Value int
	}

	test1 := &BeanWithField{Value: 1}
	test2 := &BeanWithField{Value: 2}

	SetBean("test", test1)
	assert.Same(t, GetBean("test"), test1)
	assert.Equal(t, 1, GetBean("test").(*BeanWithField).Value)

	// 覆盖
	SetBeanOverwrite("test", test2)
	assert.Same(t, GetBean("test"), test2)
	assert.Equal(t, 2, GetBean("test").(*BeanWithField).Value)
	
	// 验证已经不是test1了
	if GetBean("test") == test1 {
		t.Error("GetBean should not return test1 after overwrite")
	}
}

// TestIocFieldNotPointer 测试注入非指针字段时panic
func TestIocFieldNotPointer(t *testing.T) {
	ClearBeans()

	type InvalidBean struct {
		Field TestAutoWrite1 `autowired:""`
	}

	bean := &InvalidBean{}
	AddBean(bean)
	AddBean(&TestAutoWrite1{})

	defer func() {
		if r := recover(); r != nil {
			assert.Contains(t, r.(string), "Only can autowrite pointer or interface")
		}
	}()

	Ioc()
	t.Fatal("should panic")
}

// TestIocFieldCannotSet 测试注入私有字段时panic
func TestIocFieldCannotSet(t *testing.T) {
	ClearBeans()

	type InvalidBean struct {
		field *TestAutoWrite1 `autowired:""`
	}

	bean := &InvalidBean{}
	AddBean(bean)
	AddBean(&TestAutoWrite1{})

	defer func() {
		if r := recover(); r != nil {
			assert.Contains(t, r.(string), "Field can't set")
		}
	}()

	Ioc()
	t.Fatal("should panic")
}

// TestIocFieldNotFound 测试找不到依赖Bean时panic
func TestIocFieldNotFound(t *testing.T) {
	ClearBeans()

	type BeanWithDep struct {
		Dep *TestAutoWrite1 `autowired:""`
	}

	bean := &BeanWithDep{}
	AddBean(bean)
	// 没有添加TestAutoWrite1

	defer func() {
		if r := recover(); r != nil {
			assert.Contains(t, r.(string), "Field autowrite failed")
		}
	}()

	Ioc()
	t.Fatal("should panic")
}

// TestIocSkipNonNilField 测试跳过已赋值的字段
func TestIocSkipNonNilField(t *testing.T) {
	ClearBeans()

	preSet := &TestAutoWrite1{}
	test := &TestAutoWrite{
		TestAutoWrite1: preSet,
	}

	test2 := &TestAutoWrite2{}
	test3 := &TestAutoWrite3{}

	AddBean(test)
	AddBean(&TestAutoWrite1{}) // 不同的实例
	AddBean(test2)
	AddBean(test3)
	AddBean(&testAutoWrite4{})

	Ioc()

	// 应该保持原来的值
	assert.Same(t, test.TestAutoWrite1, preSet)
	// TestAutoWrite2应该被正常注入
	assert.Same(t, test.TestAutoWrite2, test2)
}

// TestGetBeanT 测试泛型获取Bean
func TestGetBeanT(t *testing.T) {
	ClearBeans()

	test1 := &TestAutoWrite1{}
	AddBean(test1)

	// 使用泛型获取
	result := GetBeanT[*TestAutoWrite1]()
	assert.Same(t, result, test1)

	// 指定名称获取
	SetBean("custom", test1)
	result2 := GetBeanT[*TestAutoWrite1]("custom")
	assert.Same(t, result2, test1)

	// 获取不存在的Bean
	result3 := GetBeanT[*TestAutoWrite2]()
	assert.Nil(t, result3)
}

// TestRemoveBean 测试移除Bean
func TestRemoveBean(t *testing.T) {
	ClearBeans()

	test1 := &TestAutoWrite1{}
	SetBean("test", test1)

	assert.True(t, HasBean("test"))
	assert.Same(t, GetBean("test"), test1)

	RemoveBean("test")

	assert.False(t, HasBean("test"))
	assert.Nil(t, GetBean("test"))
}

// TestHasBean 测试检查Bean是否存在
func TestHasBean(t *testing.T) {
	ClearBeans()

	assert.False(t, HasBean("test"))

	test1 := &TestAutoWrite1{}
	SetBean("test", test1)

	assert.True(t, HasBean("test"))
}

// TestGetAllBeans 测试获取所有Bean名称
func TestGetAllBeans(t *testing.T) {
	ClearBeans()

	assert.Empty(t, GetAllBeans())

	SetBean("bean1", &TestAutoWrite1{})
	SetBean("bean2", &TestAutoWrite2{})
	SetBean("bean3", &TestAutoWrite3{})

	beans := GetAllBeans()
	assert.Len(t, beans, 3)
	assert.Contains(t, beans, "bean1")
	assert.Contains(t, beans, "bean2")
	assert.Contains(t, beans, "bean3")
}

// TestClearBeans 测试清空所有Bean
func TestClearBeans(t *testing.T) {
	ClearBeans()

	SetBean("bean1", &TestAutoWrite1{})
	SetBean("bean2", &TestAutoWrite2{})

	assert.Len(t, GetAllBeans(), 2)

	ClearBeans()

	assert.Empty(t, GetAllBeans())
}

// TestAutowriteTag 测试autowrite和autowired标签
func TestAutowriteTag(t *testing.T) {
	ClearBeans()

	type BeanWithAutowrite struct {
		Field1 *TestAutoWrite1 `autowrite:""`
	}

	bean := &BeanWithAutowrite{}
	test1 := &TestAutoWrite1{}

	AddBean(bean)
	AddBean(test1)

	Ioc()

	assert.Same(t, bean.Field1, test1)
}

// 测试接口注入相关类型定义
type TestService interface {
	DoSomething() string
}

type TestServiceImpl struct{}

func (s *TestServiceImpl) DoSomething() string {
	return "done"
}

type TestConsumer struct {
	Svc TestService `autowired:""`
}

// TestInterfaceInjection 测试接口注入
func TestInterfaceInjection(t *testing.T) {
	ClearBeans()

	consumer := &TestConsumer{}
	service := &TestServiceImpl{}

	AddBean(consumer)
	AddBean(service)

	Ioc()

	assert.NotNil(t, consumer.Svc)
	assert.Equal(t, "done", consumer.Svc.DoSomething())
}

// TestCircularDependency 测试循环依赖
func TestCircularDependency(t *testing.T) {
	ClearBeans()

	test2 := &TestAutoWrite2{}
	test3 := &TestAutoWrite3{}

	AddBean(test2)
	AddBean(test3)
	AddBean(&testAutoWrite4{})

	Ioc()

	// 循环依赖应该能够正确处理
	assert.Same(t, test2.TestAutoWrite3, test3)
	assert.Same(t, test3.TestAutoWrite2, test2)
}

// TestNamedBeanInjection 测试指定名称的Bean注入
func TestNamedBeanInjection(t *testing.T) {
	ClearBeans()

	type BeanWithNamedDep struct {
		Custom *TestAutoWrite1 `autowired:"myBean"`
	}

	bean := &BeanWithNamedDep{}
	test1 := &TestAutoWrite1{}

	SetBean("myBean", test1)
	AddBean(bean)

	Ioc()

	assert.Same(t, bean.Custom, test1)
}

// TestGetBeanTWithWrongType 测试泛型获取错误类型
func TestGetBeanTWithWrongType(t *testing.T) {
	ClearBeans()

	test1 := &TestAutoWrite1{}
	SetBean("test", test1)

	// 尝试用错误的类型获取
	result := GetBeanT[*TestAutoWrite2]("test")
	assert.Nil(t, result)
}

// TestMultipleBeanRegistration 测试批量注册Bean
func TestMultipleBeanRegistration(t *testing.T) {
	ClearBeans()

	test1 := &TestAutoWrite1{}
	test2 := &TestAutoWrite2{}
	test3 := &TestAutoWrite3{}

	AddBean(test1)
	AddBean(test2)
	AddBean(test3)

	assert.True(t, HasBean("github.com/caiflower/common-tools/pkg/bean.TestAutoWrite1"))
	assert.Len(t, GetAllBeans(), 3)
}

// TestIocWithoutTag 测试没有标签的字段不被注入
func TestIocWithoutTag(t *testing.T) {
	ClearBeans()

	type BeanWithoutTag struct {
		Field1 *TestAutoWrite1 // 没有autowired标签
		Field2 *TestAutoWrite1 `autowired:""` // 有标签
	}

	bean := &BeanWithoutTag{}
	test1 := &TestAutoWrite1{}

	AddBean(bean)
	AddBean(test1)

	Ioc()

	// Field1没有标签，应该仍为nil
	assert.Nil(t, bean.Field1)
	// Field2有标签，应该被注入
	assert.Same(t, bean.Field2, test1)
}

// TestConcurrentBeanAccess 测试并发访问Bean
func TestConcurrentBeanAccess(t *testing.T) {
	ClearBeans()

	test1 := &TestAutoWrite1{}
	SetBean("test", test1)

	// 并发读取
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			result := GetBean("test")
			assert.Same(t, result, test1)
			done <- true
		}()
	}

	// 等待所有goroutine完成
	for i := 0; i < 10; i++ {
		<-done
	}
}

// TestRemoveNonExistentBean 测试移除不存在的Bean
func TestRemoveNonExistentBean(t *testing.T) {
	ClearBeans()

	// 移除不存在的Bean不应该panic
	assert.NotPanics(t, func() {
		RemoveBean("nonexistent")
	})
}

// TestGetBeanTWithEmptyName 测试泛型获取Bean时传入空名称
func TestGetBeanTWithEmptyName(t *testing.T) {
	ClearBeans()

	test1 := &TestAutoWrite1{}
	AddBean(test1)

	// 传入空名称应该使用类型推断
	result := GetBeanT[*TestAutoWrite1]()
	assert.Same(t, result, test1)
}

// TestSetBeanOverwriteNil 测试覆盖为nil时panic
func TestSetBeanOverwriteNil(t *testing.T) {
	ClearBeans()

	test1 := &TestAutoWrite1{}
	SetBean("test", test1)

	defer func() {
		if r := recover(); r != nil {
			assert.Contains(t, r.(string), "Bean can't be nil")
		}
	}()

	SetBeanOverwrite("test", nil)
	t.Fatal("should panic")
}

// TestDeepDependencyChain 测试深层依赖链
func TestDeepDependencyChain(t *testing.T) {
	ClearBeans()

	// 定义类型
	type Level4 struct {
		Value string
	}

	type Level3 struct {
		Level4 *Level4 `autowired:""`
	}

	type Level2 struct {
		Level3 *Level3 `autowired:""`
	}

	type Level1 struct {
		Level2 *Level2 `autowired:""`
	}

	level1 := &Level1{}
	level2 := &Level2{}
	level3 := &Level3{}
	level4 := &Level4{Value: "deep"}

	AddBean(level1)
	AddBean(level2)
	AddBean(level3)
	AddBean(level4)

	Ioc()

	assert.Same(t, level1.Level2, level2)
	assert.Same(t, level2.Level3, level3)
	assert.Same(t, level3.Level4, level4)
	assert.Equal(t, "deep", level1.Level2.Level3.Level4.Value)
}

// TestGetBeanName 测试GetBeanName方法
func TestGetBeanName(t *testing.T) {
	// 测试指针类型
	name := GetBeanName[*TestAutoWrite1]()
	assert.Equal(t, "github.com/caiflower/common-tools/pkg/bean.TestAutoWrite1", name)

	// 测试接口类型（现在与指针类型统一格式）
	nameSvc := GetBeanName[TestService]()
	assert.Equal(t, "github.com/caiflower/common-tools/pkg/bean.TestService", nameSvc)

	// 测试非指针非接口类型应返回空字符串
	nameEmpty := GetBeanName[TestAutoWrite1]()
	assert.Equal(t, "", nameEmpty)
}

// TestGetBeanNameEdgeCases 测试GetBeanName的边界情况
func TestGetBeanNameEdgeCases(t *testing.T) {
	// 测试多级包路径
	type DeepPackageType struct{}
	name := GetBeanName[*DeepPackageType]()
	assert.Contains(t, name, "DeepPackageType")

	// 测试基础类型指针（PkgPath为空）
	intPtrName := GetBeanName[*int]()
	// *int类型的PkgPath为空，但仍然会返回"int"
	assert.Equal(t, "int", intPtrName)
}

// TestGetBeanTUsingGetBeanName 验证GetBeanT使用GetBeanName
func TestGetBeanTUsingGetBeanName(t *testing.T) {
	ClearBeans()

	test1 := &TestAutoWrite1{}
	name := GetBeanName[*TestAutoWrite1]()
	SetBean(name, test1)

	// 使用泛型获取应该能够正确通过GetBeanName找到bean
	result := GetBeanT[*TestAutoWrite1]()
	assert.Same(t, result, test1)
}

// TestGetBeanNameFromValue 测试getBeanNameFromValue方法
func TestGetBeanNameFromValue(t *testing.T) {
	ClearBeans()

	// 测试指针类型
	test1 := &TestAutoWrite1{}
	AddBean(test1)

	// 验证bean已正确添加
	assert.True(t, HasBean("github.com/caiflower/common-tools/pkg/bean.TestAutoWrite1"))

	// 测试接口类型
	testSvc := &TestServiceImpl{}
	AddBean(testSvc)

	// 应该通过接口类型注册
	allBeans := GetAllBeans()
	assert.Contains(t, allBeans, "github.com/caiflower/common-tools/pkg/bean.TestServiceImpl")
}

// TestGetBeanNameFromValueConsistency 测试getBeanNameFromValue与GetBeanName的一致性
func TestGetBeanNameFromValueConsistency(t *testing.T) {
	ClearBeans()

	// 创建一个bean实例
	test1 := &TestAutoWrite1{}

	// 使用AddBean添加（内部使用getBeanNameFromValue）
	AddBean(test1)

	// 使用泛型GetBeanName获取名称
	genericName := GetBeanName[*TestAutoWrite1]()

	// 应该能通过泛型获取到bean
	result := GetBeanT[*TestAutoWrite1]()
	assert.Same(t, result, test1)

	// 通过名称获取也应该成功
	result2 := GetBean(genericName)
	assert.Same(t, result2, test1)
}

// TestAutowiredWithBeanName 测试autowired指定bean名称
func TestAutowiredWithBeanName(t *testing.T) {
	ClearBeans()

	type Database struct {
		Name string
	}

	type ServiceWithNamedDeps struct {
		PrimaryDB   *Database `autowired:"primary"`
		SecondaryDB *Database `autowired:"secondary"`
	}

	primary := &Database{Name: "Primary"}
	secondary := &Database{Name: "Secondary"}
	service := &ServiceWithNamedDeps{}

	SetBean("primary", primary)
	SetBean("secondary", secondary)
	AddBean(service)

	Ioc()

	assert.Same(t, service.PrimaryDB, primary)
	assert.Same(t, service.SecondaryDB, secondary)
	assert.Equal(t, "Primary", service.PrimaryDB.Name)
	assert.Equal(t, "Secondary", service.SecondaryDB.Name)
}

// TestAutowiredWithAutowriteTag 测试autowrite标签指定bean名称
func TestAutowiredWithAutowriteTag(t *testing.T) {
	ClearBeans()

	type Cache struct {
		Type string
	}

	type ServiceWithCache struct {
		RedisCache  *Cache `autowrite:"redis"`
		MemoryCache *Cache `autowired:"memory"`
	}

	redis := &Cache{Type: "Redis"}
	memory := &Cache{Type: "Memory"}
	service := &ServiceWithCache{}

	SetBean("redis", redis)
	SetBean("memory", memory)
	AddBean(service)

	Ioc()

	assert.Same(t, service.RedisCache, redis)
	assert.Same(t, service.MemoryCache, memory)
	assert.Equal(t, "Redis", service.RedisCache.Type)
	assert.Equal(t, "Memory", service.MemoryCache.Type)
}

// TestAutowiredMixedAutoAndNamed 测试混合自动注入和指定名称注入
func TestAutowiredMixedAutoAndNamed(t *testing.T) {
	ClearBeans()

	type Logger struct{}
	type Database struct {
		Name string
	}

	type MixedService struct {
		Logger     *Logger   `autowired:""`      // 自动注入
		PrimaryDB  *Database `autowired:"main"`   // 指定名称
		BackupDB   *Database `autowired:"backup"` // 指定名称
	}

	logger := &Logger{}
	mainDB := &Database{Name: "Main"}
	backupDB := &Database{Name: "Backup"}
	service := &MixedService{}

	AddBean(logger)
	SetBean("main", mainDB)
	SetBean("backup", backupDB)
	AddBean(service)

	Ioc()

	assert.Same(t, service.Logger, logger)
	assert.Same(t, service.PrimaryDB, mainDB)
	assert.Same(t, service.BackupDB, backupDB)
}

// TestAutowiredNamedBeanNotFound 测试指定名称的bean不存在时panic
func TestAutowiredNamedBeanNotFound(t *testing.T) {
	ClearBeans()

	type ServiceWithMissingDep struct {
		DB *TestAutoWrite1 `autowired:"nonexistent"`
	}

	service := &ServiceWithMissingDep{}
	AddBean(service)

	defer func() {
		if r := recover(); r != nil {
			assert.Contains(t, r.(string), "Field autowrite failed")
		}
	}()

	Ioc()
	t.Fatal("should panic")
}

// TestAutowiredInterfaceWithNamedBean 测试接口类型指定名称注入
func TestAutowiredInterfaceWithNamedBean(t *testing.T) {
	ClearBeans()

	type ServiceWithNamedInterface struct {
		Primary   TestService `autowired:"service1"`
		Secondary TestService `autowired:"service2"`
	}

	service1 := &TestServiceImpl{}
	service2 := &TestServiceImpl{}
	service := &ServiceWithNamedInterface{}

	SetBean("service1", service1)
	SetBean("service2", service2)
	AddBean(service)

	Ioc()

	assert.Same(t, service.Primary, service1)
	assert.Same(t, service.Secondary, service2)
}
