# Bean 依赖注入容器

`bean` 是一个轻量级的运行时依赖注入（IoC）容器，用于管理服务中的单例对象，简化初始化顺序和依赖装配。使用方式接近 Spring，适合从 Java 迁移的团队。

> 导入路径：`github.com/caiflower/common-tools/pkg/bean`

## 特性

| 能力 | 说明 |
| --- | --- |
| 自动注入 | 通过 `autowired` / `autowrite` 标签注入依赖 |
| 循环依赖 | 自动处理循环引用 |
| 接口注入 | 按接口类型查找实现 |
| 泛型获取 | `GetBeanT[T]()` 类型安全地获取 Bean |
| 并发安全 | 内部使用读写锁 |
| 条件注入 | 基于配置决定是否注入 |

## 与其他方案对比

| 方案 | 优点 | 缺点 |
| --- | --- | --- |
| **wire** | 编译期生成代码，类型安全，无运行时反射 | 需要安装工具、维护生成文件，大项目中生成代码可读性差 |
| **bean** | 运行时注入，使用简单，标签驱动 | 运行时反射有性能开销，错误在启动阶段才暴露 |

## 快速开始

```go
package main

import (
    "fmt"

    "github.com/caiflower/common-tools/pkg/bean"
)

type UserRepository struct{}

func (r *UserRepository) FindByID(id int) string {
    return fmt.Sprintf("User-%d", id)
}

type UserService struct {
    UserRepo *UserRepository `autowired:""`
}

func (s *UserService) GetUser(id int) string {
    return s.UserRepo.FindByID(id)
}

func main() {
    // 1. 注册所有 Bean
    bean.AddBean(&UserRepository{})
    bean.AddBean(&UserService{})

    // 2. 执行依赖注入
    bean.Ioc()

    // 3. 获取并使用
    service := bean.GetBeanT[*UserService]()
    fmt.Println(service.GetUser(1)) // User-1
}
```

> [!IMPORTANT]
> 必须先注册所有 Bean，再调用 `bean.Ioc()`。`Ioc()` 之后新增的 Bean 不会自动注入。

## API

### 注册 Bean

#### `AddBean(bean interface{})`

自动推断 Bean 名称并注册。Bean 必须是指针或接口类型。

```go
bean.AddBean(&UserService{})
```

#### `SetBean(name string, bean interface{})`

使用指定名称注册。名称已存在时 panic。

```go
bean.SetBean("userService", &UserService{})
```

#### `SetBeanOverwrite(name string, bean interface{})`

使用指定名称注册，名称已存在时覆盖。

```go
bean.SetBeanOverwrite("userService", &UserService{})
```

### 获取 Bean

#### `GetBean(name string) interface{}`

按名称获取，不存在时返回 `nil`。

```go
service := bean.GetBean("userService").(*UserService)
```

#### `GetBeanT[T any](name ...string) T`

泛型获取。不传名称时按类型推断；找不到时返回类型零值。

```go
// 按类型推断
service := bean.GetBeanT[*UserService]()

// 指定名称
service := bean.GetBeanT[*UserService]("userService")
```

### 管理 Bean

| 函数 | 说明 |
| --- | --- |
| `HasBean(name string) bool` | 判断 Bean 是否存在 |
| `RemoveBean(name string)` | 移除指定 Bean |
| `GetAllBeans() []string` | 获取所有 Bean 名称 |
| `ClearBeans()` | 清空所有 Bean |

### 依赖注入

#### `Ioc()`

执行依赖注入，自动装配所有带 `autowired` 或 `autowrite` 标签的字段。

```go
bean.Ioc()
```

## 标签

### `autowired` / `autowrite`

标记需要自动注入的字段。两个标签行为一致。

```go
type UserService struct {
    // 按类型自动查找
    UserRepo *UserRepository `autowired:""`
    Cache    CacheInterface  `autowrite:""`
}
```

#### 指定 Bean 名称

同一类型存在多个 Bean 时，通过名称精确注入：

```go
type UserService struct {
    PrimaryDB   *Database `autowired:"primaryDB"`
    SecondaryDB *Database `autowired:"secondaryDB"`
    Cache       Cache     `autowired:"redisCache"`
}
```

#### 字段要求

- 必须是指针或接口类型；
- 必须是可导出字段（首字母大写）；
- 字段已有非 nil 值时跳过注入。

#### 查找顺序

1. 标签指定了名称：按名称查找；
2. 按字段名查找；
3. 按包路径 + 结构体名查找；
4. 字段是接口：查找实现了该接口的 Bean。

### `conditional_on_property`

基于配置决定是否注入，支持独立标签和内联语法。

#### 独立标签

```go
type OptionalService struct {
    Feature *FeatureService `autowired:"" conditional_on_property:"default.feature.enabled=true"`
}
```

#### 内联语法

使用 `|` 分隔 Bean 名称和条件：

```go
type OptionalService struct {
    Feature *FeatureService `autowired:"featureBean|conditional_on_property:default.feature.enabled=true"`
    Cache   *CacheService   `autowired:"|conditional_on_property:default.cache.enabled=true"`
}
```

#### 多配置源

| 格式 | 说明 | 示例 |
| --- | --- | --- |
| `default.xxx=yyy` | 从 `default` Bean 读取 | `default.cluster.mode=redis` |
| `<beanName>.xxx=yyy` | 从指定名称的 Bean 读取 | `myApp.cache.enabled=true` |
| `xxx=yyy` | 无前缀，默认从 `default` 读取 | `cluster.mode=redis` |

```go
type ClusterService struct {
    Redis RedisClient `autowired:"redisBean|conditional_on_property:redisConfig.cluster.mode=redis"`
}

type FeatureService struct {
    Mode *ModeService `autowired:"modeBean|conditional_on_property:mode=redis"`
}
```

> [!NOTE]
> 使用前需要先注册对应的配置 Bean（如 `default`、`redisConfig`），
> 配置结构体字段需要 `json` tag，条件表达式通过 jsonpath 解析。

## 使用场景

### 接口注入

```go
type Logger interface {
    Log(msg string)
}

type ConsoleLogger struct{}

func (c *ConsoleLogger) Log(msg string) { println(msg) }

type UserService struct {
    Logger Logger `autowired:""`
}

func main() {
    bean.AddBean(&UserService{})
    bean.AddBean(&ConsoleLogger{}) // 自动匹配 Logger 接口
    bean.Ioc()

    service := bean.GetBeanT[*UserService]()
    service.Logger.Log("Hello")
}
```

### 循环依赖

```go
type ServiceA struct {
    B *ServiceB `autowired:""`
}

type ServiceB struct {
    A *ServiceA `autowired:""`
}

func main() {
    bean.AddBean(&ServiceA{})
    bean.AddBean(&ServiceB{})
    bean.Ioc()

    a := bean.GetBeanT[*ServiceA]()
    b := bean.GetBeanT[*ServiceB]()
    // a.B == b && b.A == a
}
```

### 命名 Bean 与多数据源

```go
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

### 混合使用

```go
type UserService struct {
    Logger Logger    `autowired:""`      // 按接口自动查找
    DB     *Database `autowired:"mainDB"` // 按名称指定
}

func main() {
    bean.AddBean(&ConsoleLogger{})
    bean.SetBean("mainDB", &Database{})
    bean.AddBean(&UserService{})
    bean.Ioc()
}
```

## 注意事项

1. **Bean 必须是指针或接口**：`AddBean()` 不接受值类型；
2. **字段必须可导出**：需要注入的字段首字母必须大写；
3. **字段必须是指针或接口**：不能注入值类型字段；
4. **避免重复注册**：`SetBean()` 注册同名 Bean 会 panic，使用 `SetBeanOverwrite()` 覆盖；
5. **先注册后注入**：所有 Bean 注册完成后才能调用 `Ioc()`；
6. **启动阶段完成注入**：反射有性能开销，避免在运行期反复注册和注入。

## 错误处理

以下情况会 panic：

- 注册非指针/非接口类型的 Bean；
- 注册 `nil` Bean；
- 使用 `SetBean()` 注册已存在的名称；
- 注入非指针/非接口类型的字段；
- 注入私有字段；
- 找不到依赖的 Bean。

建议在程序启动阶段完成注册和注入，尽早暴露配置错误。

## 完整示例

```go
package main

import (
    "fmt"

    "github.com/caiflower/common-tools/pkg/bean"
)

type Logger interface {
    Log(msg string)
}

type Database interface {
    Query(sql string) string
}

type ConsoleLogger struct{}

func (c *ConsoleLogger) Log(msg string) {
    fmt.Println("[LOG]", msg)
}

type MySQLDatabase struct{}

func (m *MySQLDatabase) Query(sql string) string {
    return "Result of: " + sql
}

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
    bean.AddBean(&ConsoleLogger{})
    bean.AddBean(&MySQLDatabase{})
    bean.AddBean(&UserRepository{})
    bean.AddBean(&UserService{})

    bean.Ioc()

    userService := bean.GetBeanT[*UserService]()
    fmt.Println(userService.GetUser(123))
    // [LOG] GetUser called with id=123
    // [LOG] Finding user 123
    // Result of: SELECT * FROM users WHERE id=123
}
```

## 相关文档

- 源代码：[pkg/bean/bean.go](../../pkg/bean/bean.go)
- 测试用例：[pkg/bean/bean_test.go](../../pkg/bean/bean_test.go)
- 配置加载：[global/config](../../global/config/config.go)
- 文档索引：[docs/README.md](../README.md)
