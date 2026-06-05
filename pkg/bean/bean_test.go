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
		Logger    *Logger   `autowired:""`       // 自动注入
		PrimaryDB *Database `autowired:"main"`   // 指定名称
		BackupDB  *Database `autowired:"backup"` // 指定名称
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

// ========== conditional_on_property 多配置源测试 ==========

// TestConditionalOnPropertyDefaultBean 测试原有 default bean 格式兼容性
func TestConditionalOnPropertyDefaultBean(t *testing.T) {
	ClearBeans()

	type DefaultConfig struct {
		FeatureEnabled string `json:"featureEnabled"`
		ClusterMode    string `json:"clusterMode"`
	}

	type FeatureService struct {
		Name string
	}

	type ServiceWithDefaultCondition struct {
		Feature *FeatureService `autowired:"featureBean" conditional_on_property:"default.featureEnabled=true"`
		Cluster *FeatureService `autowired:"clusterBean" conditional_on_property:"default.clusterMode=cluster"`
	}

	config := &DefaultConfig{
		FeatureEnabled: "true",
		ClusterMode:    "cluster",
	}

	feature := &FeatureService{Name: "feature"}
	clusterBean := &FeatureService{Name: "cluster"}

	SetBean("default", config)
	SetBean("clusterBean", clusterBean)
	SetBean("featureBean", feature)

	svc := &ServiceWithDefaultCondition{}
	AddBean(svc)

	Ioc()

	assert.NotNil(t, svc.Feature)
	assert.Equal(t, "feature", svc.Feature.Name)
	assert.NotNil(t, svc.Cluster)
	assert.Equal(t, "cluster", svc.Cluster.Name)
}

// TestConditionalOnPropertyDefaultBeanNotMatch 测试 default bean 条件不匹配时不注入
func TestConditionalOnPropertyDefaultBeanNotMatch(t *testing.T) {
	ClearBeans()

	type DefaultConfig struct {
		FeatureEnabled string `json:"featureEnabled"`
	}

	type FeatureService struct {
		Name string
	}

	type ServiceWithCondition struct {
		Feature *FeatureService `autowired:"featureBean" conditional_on_property:"default.featureEnabled=true"`
	}

	config := &DefaultConfig{FeatureEnabled: "false"}

	SetBean("default", config)
	SetBean("featureBean", &FeatureService{Name: "feature"})

	svc := &ServiceWithCondition{}
	AddBean(svc)

	Ioc()

	assert.Nil(t, svc.Feature)
}

// TestConditionalOnPropertyCustomBean 测试从自定义 bean 读取配置
func TestConditionalOnPropertyCustomBean(t *testing.T) {
	ClearBeans()

	type AppConfig struct {
		CacheEnabled string `json:"cacheEnabled"`
	}

	type CacheService struct {
		Name string
	}

	type ServiceWithCustomCondition struct {
		Cache *CacheService `autowired:"cacheBean" conditional_on_property:"myApp.cacheEnabled=true"`
	}

	appConfig := &AppConfig{CacheEnabled: "true"}

	SetBean("myApp", appConfig)
	SetBean("cacheBean", &CacheService{Name: "cache"})

	svc := &ServiceWithCustomCondition{}
	AddBean(svc)

	Ioc()

	assert.NotNil(t, svc.Cache)
	assert.Equal(t, "cache", svc.Cache.Name)
}

// TestConditionalOnPropertyCustomBeanNotMatch 测试自定义 bean 条件不匹配
func TestConditionalOnPropertyCustomBeanNotMatch(t *testing.T) {
	ClearBeans()

	type AppConfig struct {
		CacheEnabled string `json:"cacheEnabled"`
	}

	type CacheService struct {
		Name string
	}

	type ServiceWithCustomCondition struct {
		Cache *CacheService `autowired:"cacheBean" conditional_on_property:"myApp.cacheEnabled=true"`
	}

	appConfig := &AppConfig{CacheEnabled: "false"}

	SetBean("myApp", appConfig)
	SetBean("cacheBean", &CacheService{Name: "cache"})

	svc := &ServiceWithCustomCondition{}
	AddBean(svc)

	Ioc()

	assert.Nil(t, svc.Cache)
}

// TestConditionalOnPropertyShorthand 测试无前缀简写格式（默认从 default bean 读取）
func TestConditionalOnPropertyShorthand(t *testing.T) {
	ClearBeans()

	type DefaultConfig struct {
		Mode string `json:"mode"`
	}

	type ModeService struct {
		Name string
	}

	type ServiceWithShorthand struct {
		ModeSvc *ModeService `autowired:"modeBean" conditional_on_property:"mode=redis"`
	}

	config := &DefaultConfig{Mode: "redis"}

	SetBean("default", config)
	SetBean("modeBean", &ModeService{Name: "mode"})

	svc := &ServiceWithShorthand{}
	AddBean(svc)

	Ioc()

	assert.NotNil(t, svc.ModeSvc)
	assert.Equal(t, "mode", svc.ModeSvc.Name)
}

// TestConditionalOnPropertyShorthandNotMatch 测试简写格式条件不匹配
func TestConditionalOnPropertyShorthandNotMatch(t *testing.T) {
	ClearBeans()

	type DefaultConfig struct {
		Mode string `json:"mode"`
	}

	type ModeService struct {
		Name string
	}

	type ServiceWithShorthand struct {
		ModeSvc *ModeService `autowired:"modeBean" conditional_on_property:"mode=cluster"`
	}

	config := &DefaultConfig{Mode: "redis"}

	SetBean("default", config)
	SetBean("modeBean", &ModeService{Name: "mode"})

	svc := &ServiceWithShorthand{}
	AddBean(svc)

	Ioc()

	assert.Nil(t, svc.ModeSvc)
}

// TestConditionalOnPropertyMultipleConditions 测试多个字段使用不同配置源
func TestConditionalOnPropertyMultipleConditions(t *testing.T) {
	ClearBeans()

	type DefaultConfig struct {
		FeatureEnabled string `json:"featureEnabled"`
	}

	type RedisConfig struct {
		ClusterMode string `json:"clusterMode"`
	}

	type ServiceA struct{ Name string }
	type ServiceB struct{ Name string }

	type ServiceWithMultiConditions struct {
		SvcA *ServiceA `autowired:"svcABean" conditional_on_property:"default.featureEnabled=yes"`
		SvcB *ServiceB `autowired:"svcBBean" conditional_on_property:"redisConfig.clusterMode=redis"`
	}

	defaultConfig := &DefaultConfig{FeatureEnabled: "yes"}
	redisConfig := &RedisConfig{ClusterMode: "redis"}

	SetBean("default", defaultConfig)
	SetBean("redisConfig", redisConfig)
	SetBean("svcABean", &ServiceA{Name: "a"})
	SetBean("svcBBean", &ServiceB{Name: "b"})

	svc := &ServiceWithMultiConditions{}
	AddBean(svc)

	Ioc()

	assert.NotNil(t, svc.SvcA)
	assert.Equal(t, "a", svc.SvcA.Name)
	assert.NotNil(t, svc.SvcB)
	assert.Equal(t, "b", svc.SvcB.Name)
}

// TestConditionalOnPropertyInvalidFormat 测试无效格式 panic
func TestConditionalOnPropertyInvalidFormat(t *testing.T) {
	ClearBeans()

	type Config struct {
		Key string `json:"key"`
	}

	type ServiceWithInvalidCondition struct {
		Field *TestAutoWrite1 `autowired:"" conditional_on_property:"no_equal_sign"`
	}

	SetBean("default", &Config{Key: "val"})
	AddBean(&TestAutoWrite1{})

	svc := &ServiceWithInvalidCondition{}
	AddBean(svc)

	defer func() {
		if r := recover(); r != nil {
			assert.Contains(t, r.(string), "not supported conditionalOnProperty")
		}
	}()

	Ioc()
	t.Fatal("should panic")
}

// TestConditionalOnPropertyMixedConditions 测试混合使用有条件和无条件注入
func TestConditionalOnPropertyMixedConditions(t *testing.T) {
	ClearBeans()

	type DefaultConfig struct {
		FeatureEnabled string `json:"featureEnabled"`
	}

	type OptionalService struct{ Name string }
	type RequiredService struct{ Name string }

	type MixedService struct {
		Optional *OptionalService `autowired:"optBean" conditional_on_property:"default.featureEnabled=true"`
		Required *RequiredService `autowired:""`
	}

	config := &DefaultConfig{FeatureEnabled: "false"}

	SetBean("default", config)
	SetBean("optBean", &OptionalService{Name: "optional"})
	AddBean(&RequiredService{Name: "required"})

	svc := &MixedService{}
	AddBean(svc)

	Ioc()

	assert.Nil(t, svc.Optional)
	assert.NotNil(t, svc.Required)
	assert.Equal(t, "required", svc.Required.Name)
}

// ========== conditional_on_property 新语法测试（| 分隔符） ==========

// TestNewSyntaxWithDefaultBean 测试新语法 autowired:"beanName|conditional_on_property:default.xxx=yyy"
func TestNewSyntaxWithDefaultBean(t *testing.T) {
	ClearBeans()

	type DefaultConfig struct {
		FeatureEnabled string `json:"featureEnabled"`
	}

	type FeatureService struct {
		Name string
	}

	type ServiceWithNewSyntax struct {
		Feature *FeatureService `autowired:"featureBean|conditional_on_property:default.featureEnabled=true"`
	}

	config := &DefaultConfig{FeatureEnabled: "true"}
	SetBean("default", config)
	SetBean("featureBean", &FeatureService{Name: "feature"})

	svc := &ServiceWithNewSyntax{}
	AddBean(svc)

	Ioc()

	assert.NotNil(t, svc.Feature)
	assert.Equal(t, "feature", svc.Feature.Name)
}

// TestNewSyntaxWithDefaultBeanNotMatch 测试新语法条件不匹配
func TestNewSyntaxWithDefaultBeanNotMatch(t *testing.T) {
	ClearBeans()

	type DefaultConfig struct {
		FeatureEnabled string `json:"featureEnabled"`
	}

	type FeatureService struct {
		Name string
	}

	type ServiceWithNewSyntax struct {
		Feature *FeatureService `autowired:"featureBean|conditional_on_property:default.featureEnabled=true"`
	}

	config := &DefaultConfig{FeatureEnabled: "false"}
	SetBean("default", config)
	SetBean("featureBean", &FeatureService{Name: "feature"})

	svc := &ServiceWithNewSyntax{}
	AddBean(svc)

	Ioc()

	assert.Nil(t, svc.Feature)
}

// TestNewSyntaxWithCustomBean 测试新语法使用自定义配置源
func TestNewSyntaxWithCustomBean(t *testing.T) {
	ClearBeans()

	type AppConfig struct {
		CacheEnabled string `json:"cacheEnabled"`
	}

	type CacheService struct {
		Name string
	}

	type ServiceWithNewSyntax struct {
		Cache *CacheService `autowired:"cacheBean|conditional_on_property:myApp.cacheEnabled=true"`
	}

	appConfig := &AppConfig{CacheEnabled: "true"}
	SetBean("myApp", appConfig)
	SetBean("cacheBean", &CacheService{Name: "cache"})

	svc := &ServiceWithNewSyntax{}
	AddBean(svc)

	Ioc()

	assert.NotNil(t, svc.Cache)
	assert.Equal(t, "cache", svc.Cache.Name)
}

// TestNewSyntaxWithCustomBeanNotMatch 测试新语法自定义配置源条件不匹配
func TestNewSyntaxWithCustomBeanNotMatch(t *testing.T) {
	ClearBeans()

	type AppConfig struct {
		CacheEnabled string `json:"cacheEnabled"`
	}

	type CacheService struct {
		Name string
	}

	type ServiceWithNewSyntax struct {
		Cache *CacheService `autowired:"cacheBean|conditional_on_property:myApp.cacheEnabled=true"`
	}

	appConfig := &AppConfig{CacheEnabled: "false"}
	SetBean("myApp", appConfig)
	SetBean("cacheBean", &CacheService{Name: "cache"})

	svc := &ServiceWithNewSyntax{}
	AddBean(svc)

	Ioc()

	assert.Nil(t, svc.Cache)
}

// TestNewSyntaxWithShorthand 测试新语法简写格式（无 beanName 前缀）
func TestNewSyntaxWithShorthand(t *testing.T) {
	ClearBeans()

	type DefaultConfig struct {
		Mode string `json:"mode"`
	}

	type ModeService struct {
		Name string
	}

	type ServiceWithNewSyntax struct {
		ModeSvc *ModeService `autowired:"modeBean|conditional_on_property:mode=redis"`
	}

	config := &DefaultConfig{Mode: "redis"}
	SetBean("default", config)
	SetBean("modeBean", &ModeService{Name: "mode"})

	svc := &ServiceWithNewSyntax{}
	AddBean(svc)

	Ioc()

	assert.NotNil(t, svc.ModeSvc)
	assert.Equal(t, "mode", svc.ModeSvc.Name)
}

// TestNewSyntaxEmptyBeanName 测试新语法空 bean 名（autowired:"|conditional_on_property:xxx=yyy"）
func TestNewSyntaxEmptyBeanName(t *testing.T) {
	ClearBeans()

	type DefaultConfig struct {
		FeatureEnabled string `json:"featureEnabled"`
	}

	type FeatureService struct {
		Name string
	}

	type ServiceWithNewSyntax struct {
		Feature *FeatureService `autowired:"|conditional_on_property:default.featureEnabled=true"`
	}

	config := &DefaultConfig{FeatureEnabled: "true"}
	SetBean("default", config)
	AddBean(&FeatureService{Name: "feature"})

	svc := &ServiceWithNewSyntax{}
	AddBean(svc)

	Ioc()

	assert.NotNil(t, svc.Feature)
}

// TestNewSyntaxMixedWithOldSyntax 测试新旧语法混用
func TestNewSyntaxMixedWithOldSyntax(t *testing.T) {
	ClearBeans()

	type DefaultConfig struct {
		FeatureEnabled string `json:"featureEnabled"`
		CacheEnabled   string `json:"cacheEnabled"`
	}

	type FeatureService struct{ Name string }
	type CacheService struct{ Name string }

	type MixedService struct {
		Feature *FeatureService `autowired:"featureBean|conditional_on_property:default.featureEnabled=true"`
		Cache   *CacheService   `autowired:"cacheBean" conditional_on_property:"default.cacheEnabled=true"`
	}

	config := &DefaultConfig{FeatureEnabled: "true", CacheEnabled: "true"}
	SetBean("default", config)
	SetBean("featureBean", &FeatureService{Name: "feature"})
	SetBean("cacheBean", &CacheService{Name: "cache"})

	svc := &MixedService{}
	AddBean(svc)

	Ioc()

	assert.NotNil(t, svc.Feature)
	assert.Equal(t, "feature", svc.Feature.Name)
	assert.NotNil(t, svc.Cache)
	assert.Equal(t, "cache", svc.Cache.Name)
}

// TestNewSyntaxAutowriteTag 测试 autowrite 标签也支持新语法
func TestNewSyntaxAutowriteTag(t *testing.T) {
	ClearBeans()

	type DefaultConfig struct {
		Mode string `json:"mode"`
	}

	type ModeService struct {
		Name string
	}

	type ServiceWithNewSyntax struct {
		ModeSvc *ModeService `autowrite:"modeBean|conditional_on_property:mode=redis"`
	}

	config := &DefaultConfig{Mode: "redis"}
	SetBean("default", config)
	SetBean("modeBean", &ModeService{Name: "mode"})

	svc := &ServiceWithNewSyntax{}
	AddBean(svc)

	Ioc()

	assert.NotNil(t, svc.ModeSvc)
	assert.Equal(t, "mode", svc.ModeSvc.Name)
}
