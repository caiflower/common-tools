# Bean - 依赖注入容器

## 📖 概述

`bean` 包是一个轻量级的依赖注入（IoC）容器，设计初衷是为了更好地管理服务中的单例对象，简化初始化服务的复杂度。使用方法类似于 Java 的依赖注入框架（如 Spring），对于有 Java 经验的程序员非常容易上手。

## 🆚 对比其他方案

| 工具名 | 优点 | 缺点 | 导入路径 |
| ------ | ---- | ---- | -------- |
| **wire** | 编译时依赖注入，类型安全 | 需要安装工具，维护 gen 文件，生成的代码在大项目中可读性差 | github.com/google/wire |
| **bean** | 运行时注入，使用简单，通过标签自动注入 | 运行时反射有性能开销 | [github.com/caiflower/common-tools/pkg/bean](../pkg/bean) |

## ✨ 核心特性

- ✅ **自动依赖注入**：通过结构体标签 `autowired` 或 `autowrite` 自动注入依赖
- ✅ **循环依赖支持**：自动处理循环依赖关系
- ✅ **接口注入**：支持接口类型的自动注入
- ✅ **泛型支持**：提供 `GetBeanT[T]()` 泛型方法获取 Bean
- ✅ **线程安全**：内部使用读写锁保证并发安全
- ✅ **条件注入**：支持基于配置的条件注入

## 🚀 快速开始

### 基本使用

```go
package main

import (
    "fmt"
    "github.com/caiflower/common-tools/pkg/bean"
)

// 定义服务
type UserService struct {
    UserRepo *UserRepository `autowired:""`
}

func (s *UserService) GetUser(id int) string {
    return s.UserRepo.FindByID(id)
}

type UserRepository struct{}

func (r *UserRepository) FindByID(id int) string {
    return fmt.Sprintf("User-%d", id)
}

func main() {
    // 注册 Bean
    bean.AddBean(&UserService{})
    bean.AddBean(&UserRepository{})
    
    // 执行依赖注入
    bean.Ioc()
    
    // 获取 Bean
    userService := bean.GetBeanT[*UserService]()
    fmt.Println(userService.GetUser(1)) // 输出: User-1
}
```

## 📚 API 文档

### 注册 Bean

#### `AddBean(bean interface{})`

自动推断 Bean 名称并注册（Bean 必须是指针或接口类型）。

```go
bean.AddBean(&UserService{})
```

#### `SetBean(name string, bean interface{})`

使用指定名称注册 Bean，如果名称已存在会 panic。

```go
bean.SetBean("userService", &UserService{})
```

#### `SetBeanOverwrite(name string, bean interface{})`

使用指定名称注册 Bean，如果名称已存在则覆盖。

```go
bean.SetBeanOverwrite("userService", &UserService{})
```

### 获取 Bean

#### `GetBean(name string) interface{}`

根据名称获取 Bean。

```go
service := bean.GetBean("userService").(*UserService)
```

#### `GetBeanT[T any](name ...string) T`

泛型方式获取 Bean，支持自动类型推断或指定名称。

```go
// 自动推断
service := bean.GetBeanT[*UserService]()

// 指定名称
service := bean.GetBeanT[*UserService]("userService")
```

### 管理 Bean

#### `HasBean(name string) bool`

检查 Bean 是否存在。

```go
if bean.HasBean("userService") {
    // Bean 存在
}
```

#### `RemoveBean(name string)`

移除指定的 Bean。

```go
bean.RemoveBean("userService")
```

#### `GetAllBeans() []string`

获取所有已注册的 Bean 名称。

```go
names := bean.GetAllBeans()
for _, name := range names {
    fmt.Println(name)
}
```

#### `ClearBeans()`

清空所有已注册的 Bean。

```go
bean.ClearBeans()
```

### 依赖注入

#### `Ioc()`

执行依赖注入，自动装配所有带有 `autowired` 或 `autowrite` 标签的字段。

```go
bean.Ioc()
```

## 🔖 标签说明

### `autowired` / `autowrite`

标记需要自动注入的字段，支持空值或指定Bean名称。

#### 基本用法

```go
type UserService struct {
    // 空值：自动根据类型查找Bean
    UserRepo *UserRepository `autowired:""`
    Cache    CacheInterface  `autowrite:""`
}
```

#### 指定Bean名称

当有多个相同类型的Bean时，可以通过指定名称精确注入：

```go
type UserService struct {
    // 注入名为 "primaryDB" 的Bean
    PrimaryDB   *Database `autowired:"primaryDB"`
    // 注入名为 "secondaryDB" 的Bean  
    SecondaryDB *Database `autowired:"secondaryDB"`
    // 注入名为 "redisCache" 的Bean
    Cache       Cache     `autowired:"redisCache"`
}
```

**字段要求：**
- 必须是指针或接口类型
- 必须是可导出字段（首字母大写）
- 如果字段已有值（非 nil），则跳过注入

**Bean 查找顺序：**
1. 如果标签指定了值（如 `autowired:"beanName"`），优先根据该名称查找
2. 根据字段名称查找
3. 根据包路径 + 结构体名称查找
4. 对于接口，查找实现了该接口的 Bean

### `conditional_on_property`

条件注入，基于配置决定是否注入。支持两种写法：独立标签和内联语法。

#### 独立标签写法

```go
type OptionalService struct {
    Feature *FeatureService `autowired:"" conditional_on_property:"default.feature.enabled=true"`
}
```

#### 内联语法写法

使用 `|` 分隔符将条件直接写在 `autowired`/`autowrite` 标签值中，更紧凑：

```go
type OptionalService struct {
    // 指定 bean 名称 + 条件
    Feature *FeatureService `autowired:"featureBean|conditional_on_property:default.feature.enabled=true"`
    // 空 bean 名称 + 条件（自动按类型查找）
    Cache   *CacheService   `autowired:"|conditional_on_property:default.cache.enabled=true"`
}
```

#### 多配置源

条件表达式支持从任意已注册的 Bean 中读取配置，不再局限于 `default`：

| 格式 | 说明 | 示例 |
|------|------|------|
| `default.xxx=yyy` | 从 `default` Bean 读取（原有格式，完全兼容） | `default.cluster.mode=redis` |
| `<beanName>.xxx=yyy` | 从指定名称的 Bean 读取 | `myApp.cache.enabled=true` |
| `xxx=yyy` | 无前缀，默认从 `default` Bean 读取 | `cluster.mode=redis` |

```go
// 从自定义配置 Bean 读取
type ClusterService struct {
    Redis RedisClient `autowired:"redisBean|conditional_on_property:redisConfig.cluster.mode=redis"`
}

// 简写格式，默认从 default 读取
type FeatureService struct {
    Mode *ModeService `autowired:"modeBean|conditional_on_property:mode=redis"`
}
```

**注意：** 需要先注册对应的配置 Bean（如 `default`、`redisConfig` 等），配置结构体的字段需添加 `json` tag 以便 jsonpath 解析。

## 💡 使用示例

### 示例 1：基本依赖注入

```go
package main

import (
    "github.com/caiflower/common-tools/pkg/bean"
)

type Database struct{}

type UserRepository struct {
    DB *Database `autowired:""`
}

type UserService struct {
    Repo *UserRepository `autowired:""`
}

func main() {
    bean.AddBean(&Database{})
    bean.AddBean(&UserRepository{})
    bean.AddBean(&UserService{})
    
    bean.Ioc()
    
    service := bean.GetBeanT[*UserService]()
    // service.Repo.DB 已自动注入
}
```

### 示例 2：接口注入

```go
package main

import (
    "github.com/caiflower/common-tools/pkg/bean"
)

type Logger interface {
    Log(msg string)
}

type ConsoleLogger struct{}

func (c *ConsoleLogger) Log(msg string) {
    println(msg)
}

type UserService struct {
    Logger Logger `autowired:""`
}

func main() {
    bean.AddBean(&UserService{})
    bean.AddBean(&ConsoleLogger{}) // 会自动匹配到 Logger 接口
    
    bean.Ioc()
    
    service := bean.GetBeanT[*UserService]()
    service.Logger.Log("Hello")
}
```

### 示例 3：循环依赖

```go
package main

import (
    "github.com/caiflower/common-tools/pkg/bean"
)

type ServiceA struct {
    B *ServiceB `autowired:""`
}

type ServiceB struct {
    A *ServiceA `autowired:""`
}

func main() {
    bean.AddBean(&ServiceA{})
    bean.AddBean(&ServiceB{})
    
    bean.Ioc() // 自动处理循环依赖
    
    a := bean.GetBeanT[*ServiceA]()
    b := bean.GetBeanT[*ServiceB]()
    // a.B == b && b.A == a
}
```

### 示例 4：指定 Bean 名称注入

#### 4.1 使用 `autowired:"beanName"` 精确注入

```go
package main

import (
    "github.com/caiflower/common-tools/pkg/bean"
)

type Cache interface {
    Get(key string) string
}

type RedisCache struct{}
func (r *RedisCache) Get(key string) string { return "redis:" + key }

type MemoryCache struct{}
func (m *MemoryCache) Get(key string) string { return "memory:" + key }

type UserService struct {
    // 指定使用 redisCache
    Cache Cache `autowired:"redisCache"`
}

func main() {
    bean.SetBean("redisCache", &RedisCache{})
    bean.SetBean("memoryCache", &MemoryCache{})
    bean.AddBean(&UserService{})
    
    bean.Ioc()
    
    service := bean.GetBeanT[*UserService]()
    // service.Cache 使用的是 RedisCache
}
```

#### 4.2 多数据源场景

```go
package main

import (
    "github.com/caiflower/common-tools/pkg/bean"
)

type Database struct {
    Name string
}

type MultiDBService struct {
    PrimaryDB   *Database `autowired:"primaryDB"`
    SecondaryDB *Database `autowired:"secondaryDB"`
    CacheDB     *Database `autowired:"cacheDB"`
}

func main() {
    bean.SetBean("primaryDB", &Database{Name: "MySQL"})
    bean.SetBean("secondaryDB", &Database{Name: "PostgreSQL"})
    bean.SetBean("cacheDB", &Database{Name: "Redis"})
    bean.AddBean(&MultiDBService{})
    
    bean.Ioc()
    
    service := bean.GetBeanT[*MultiDBService]()
    // service.PrimaryDB.Name == "MySQL"
    // service.SecondaryDB.Name == "PostgreSQL"  
    // service.CacheDB.Name == "Redis"
}
```

#### 4.3 混合使用自动注入和指定名称

```go
package main

import (
    "github.com/caiflower/common-tools/pkg/bean"
)

type Logger interface {
    Log(msg string)
}

type ConsoleLogger struct{}
func (c *ConsoleLogger) Log(msg string) { println(msg) }

type Database struct{}

type UserService struct {
    // 自动查找 Logger 接口的实现
    Logger Logger `autowired:""`
    // 指定使用名为 "mainDB" 的 Database
    DB     *Database `autowired:"mainDB"`
}

func main() {
    bean.AddBean(&ConsoleLogger{})  // 自动匹配到 Logger 接口
    bean.SetBean("mainDB", &Database{})
    bean.AddBean(&UserService{})
    
    bean.Ioc()
    
    service := bean.GetBeanT[*UserService]()
    // service.Logger 和 service.DB 都已正确注入
}
```

## ⚠️ 注意事项

1. **Bean 必须是指针或接口**：`AddBean()` 只接受指针或接口类型
2. **字段必须可导出**：需要注入的字段首字母必须大写
3. **字段必须是指针或接口**：只能注入指针或接口类型的字段
4. **避免重复注册**：使用 `SetBean()` 注册同名 Bean 会 panic，使用 `SetBeanOverwrite()` 可覆盖
5. **先注册后注入**：必须先通过 `AddBean()`/`SetBean()` 注册所有 Bean，然后调用 `Ioc()` 执行注入
6. **性能考虑**：依赖注入使用反射，有一定性能开销，建议在程序启动时一次性完成

## 🔧 错误处理

Bean 包在以下情况会 panic：

- 尝试注册非指针/非接口类型的 Bean
- 尝试注册 nil Bean
- 使用 `SetBean()` 注册已存在的 Bean 名称
- 尝试注入非指针/非接口类型的字段
- 尝试注入私有字段（不可导出）
- 找不到依赖的 Bean

建议在程序启动阶段完成所有 Bean 的注册和注入，这样可以尽早发现配置错误。

## 📝 完整示例

```go
package main

import (
    "fmt"
    "github.com/caiflower/common-tools/pkg/bean"
)

// 定义接口
type Logger interface {
    Log(msg string)
}

type Database interface {
    Query(sql string) string
}

// 实现类
type ConsoleLogger struct{}

func (c *ConsoleLogger) Log(msg string) {
    fmt.Println("[LOG]", msg)
}

type MySQLDatabase struct{}

func (m *MySQLDatabase) Query(sql string) string {
    return "Result of: " + sql
}

// 业务层
type UserRepository struct {
    DB     Database `autowired:""`
    Logger Logger   `autowired:""`
}

func (r *UserRepository) FindByID(id int) string {
    r.Logger.Log(fmt.Sprintf("Finding user %d", id))
    return r.DB.Query(fmt.Sprintf("SELECT * FROM users WHERE id=%d", id))
}

type UserService struct {
    Repo   *UserRepository `autowired:""`
    Logger Logger          `autowired:""`
}

func (s *UserService) GetUser(id int) string {
    s.Logger.Log(fmt.Sprintf("GetUser called with id=%d", id))
    return s.Repo.FindByID(id)
}

func main() {
    // 注册所有 Bean
    bean.AddBean(&ConsoleLogger{})
    bean.AddBean(&MySQLDatabase{})
    bean.AddBean(&UserRepository{})
    bean.AddBean(&UserService{})
    
    // 执行依赖注入
    bean.Ioc()
    
    // 获取并使用
    userService := bean.GetBeanT[*UserService]()
    result := userService.GetUser(123)
    fmt.Println(result)
    
    // 输出:
    // [LOG] GetUser called with id=123
    // [LOG] Finding user 123
    // Result of: SELECT * FROM users WHERE id=123
}
```

## 🔗 相关链接

- [源代码](../pkg/bean/bean.go)
- [测试用例](../pkg/bean/bean_test.go)
- [返回主文档](../README.md)