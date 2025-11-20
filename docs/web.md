# Web框架使用指南

## 概述

Web包是一个轻量级的RESTful Web框架，提供HTTP服务器、请求路由、参数校验、拦截器等功能。支持两种请求风格：
- **Action风格**：基于查询参数 `?action=xxx` 的传统风格
- **RESTful风格**：基于HTTP方法和路径的REST API风格


---
在CPU为Intel(R) Xeon(R) Platinum 8338C CPU，2c4g条件下，使用wrk工具压测，结果如下：
```bash
[root@k8s-node3 ~]# wrk -t12 -c500 -d60s http://127.0.0.1:8080/v1/req
Running 1m test @ http://127.0.0.1:8080/v1/req
  12 threads and 500 connections
  Thread Stats   Avg      Stdev     Max   +/- Stdev
    Latency    32.02ms  29.00ms    363.80ms 82.94%
    Req/Sec     1.46k   279.93     2.98k    75.98%
  1044171 requests in 1.00m, 251.94MB read
  Non-2xx or 3xx responses: 1045589
Requests/sec: 17414.91
Transfer/sec:      4.19MB
```
500个连接，请求/v1/req接口，1分钟处理了1045589次请求，平均每秒17414.91次，延迟均值32ms，标准差29ms，最大延迟363ms。


## 使用介绍

### 1. 初始化HTTP服务器

```go
import "github.com/caiflower/common-tools/web/v1"

// 定义配置
config := webv1.Config{
    Name:                  "myapp",
    Port:                  8080,
    ReadTimeout:           20,
    WriteTimeout:          35,
    HandleTimeout:         ptrToUint(60),
    RootPath:              "",
    HeaderTraceID:         "X-Request-Id",
    ControllerRootPkgName: "controller",
    EnablePprof:           false,
    WebLimiter: webv1.WebLimiter{
        Enable: false,
        Qos:    1000,
    },
}

// 初始化服务器
server := webv1.InitDefaultHttpServer(config)
server.StartUp()
```

### 2. 定义Controller

#### Action风格示例

```go
package controller

type UserController struct {
}

// 定义请求参数结构体
type GetUserReq struct {
    ID int `json:"id" verf:""`
}

// 定义响应结构体
type User struct {
    ID   int    `json:"id"`
    Name string `json:"name"`
}

// 定义处理方法，返回 (data, error)
func (c *UserController) GetUser(req *GetUserReq) (*User, error) {
    return &User{ID: req.ID, Name: "John"}, nil
}

// 处理错误返回 ApiError
func (c *UserController) DeleteUser(req *GetUserReq) (interface{}, webv1.ApiError) {
    if req.ID <= 0 {
        return nil, e.NewApiError(e.InvalidArgument, "Invalid ID", nil)
    }
    return nil, nil
}
```

**Action风格请求**：
```
POST /UserController?Action=GetUser
Content-Type: application/json

{
    "id": 1
}
```

#### RESTful风格示例

```go
package controller

type ProductController struct {
}

type CreateProductReq struct {
    Name  string `json:"name" verf:""`
    Price float64 `json:"price" verf:""`
}

type Product struct {
    ID    string  `json:"id"`
    Name  string  `json:"name"`
    Price float64 `json:"price"`
}

func (c *ProductController) CreateProduct(req *CreateProductReq) (*Product, error) {
    return &Product{
        ID:    "prod-123",
        Name:  req.Name,
        Price: req.Price,
    }, nil
}

func (c *ProductController) GetProductByID(req *GetProductReq) (*Product, error) {
    return &Product{
        ID:    req.ID,
        Name:  "Test Product",
        Price: 99.99,
    }, nil
}
```

### 3. 注册Controller

```go
// Action风格：自动注册
controller := &UserController{}
webv1.AddController(controller)
```

### 4. 注册RESTful路由

```go
import (
    "github.com/caiflower/common-tools/web/v1"
)

// 注册RESTful控制器
webv1.Register(
    webv1.NewRestFul().
        Version("/v1").
        Method("POST").
        Path("/products").
        Controller("ProductController").
        Action("CreateProduct"),
)

webv1.Register(
    webv1.NewRestFul().
        Version("/v1").
        Method("GET").
        Path("/products/{productId}").
        Controller("ProductController").
        Action("GetProductByID"),
)
```

**RESTful风格请求**：
```
POST /v1/products HTTP/1.1
Content-Type: application/json

{
    "name": "Product Name",
    "price": 99.99
}

GET /v1/products/prod-123 HTTP/1.1
```

---

## 核心功能

### 请求参数绑定

支持从多个来源自动绑定参数：

#### JSON Body绑定（POST/PUT/PATCH/DELETE）

```go
type UserReq struct {
    Name  string `json:"name"`
    Email string `json:"email"`
}

func (c *UserController) CreateUser(req *UserReq) (*User, error) {
    // req.Name 和 req.Email 自动从JSON body绑定
    return &User{Name: req.Name}, nil
}
```

#### 查询参数绑定（GET/Action风格）

```go
type SearchReq struct {
    Keyword string `json:"keyword"`
    Page    int    `json:"page"`
}

func (c *UserController) Search(req *SearchReq) (interface{}, error) {
    // 自动从URL查询参数绑定
    // ?keyword=test&page=1
    return nil, nil
}
```

#### 路径参数绑定（RESTful风格）

```go
type GetProductReq struct {
    ProductID string `json:"productId"`
    SubProductID string `json:"subProductId"`
}

func (c *ProductController) GetProduct(req *GetProductReq) (*Product, error) {
    // 自动从路径参数 /v1/products/{productId}/sub/{subProductId} 绑定
    return &Product{ID: req.ProductID}, nil
}
```

#### 请求头绑定

```go
type AuthReq struct {
    Authorization string `header:"Authorization"`
    ContentType   string `header:"Content-Type"`
}

func (c *UserController) GetUser(req *AuthReq) (interface{}, error) {
    // 自动从HTTP请求头绑定
    return nil, nil
}
```

#### 默认值设置

```go
type PageReq struct {
    Page  int `json:"page" default:"1"`
    Size  int `json:"size" default:"10"`
}

func (c *UserController) List(req *PageReq) (interface{}, error) {
    // 如果Page/Size未提供，使用默认值
    return nil, nil
}
```

### 参数校验

框架支持丰富的参数校验标签：

#### 必填校验

```go
type UserReq struct {
    Name string `json:"name" verf:""` // 必填
}
```

#### 枚举值校验

```go
type OrderReq struct {
    Status string `json:"status" inList:"pending,processing,completed"`
}
```

#### 正则表达式校验

```go
type EmailReq struct {
    Email string `json:"email" reg:"^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}$"`
}
```

#### 范围校验

```go
type AgeReq struct {
    Age int `json:"age" between:"0,150"`
}
```

#### 长度校验

```go
type PasswordReq struct {
    Password string `json:"password" len:"8,32"` // 长度8-32
}
```

#### 数组元素长度校验

```go
type TagsReq struct {
    Tags []string `json:"tags" itemLen:"1,20"` // 每个元素长度1-20
}
```

#### 可选字段

```go
type FilterReq struct {
    Category *string `json:"category" verf:"nilable"` // 可选，可以为nil
}
```

### 响应格式

框架自动将方法返回值封装为统一的响应格式：

#### 成功响应

```json
{
    "requestId": "550e8400-e29b-41d4-a716-446655440000",
    "data": {
        "id": 1,
        "name": "Product Name"
    }
}
```

#### 错误响应

```json
{
    "requestId": "550e8400-e29b-41d4-a716-446655440000",
    "error": {
        "code": 400,
        "type": "InvalidArgument",
        "message": "Invalid input parameters"
    }
}
```

### 错误处理

支持多种错误返回方式：

```go
// 方式1：返回 error
func (c *Controller) Method1(req *Req) (*Resp, error) {
    return nil, fmt.Errorf("error message")
}

// 方式2：返回 ApiError
func (c *Controller) Method2(req *Req) (*Resp, e.ApiError) {
    return nil, e.NewApiError(e.InvalidArgument, "Invalid argument", nil)
}

// 预定义错误码
var (
    NotFound        // 404
    NotAcceptable   // 406
    Unknown         // 500
    Internal        // 500
    TooManyRequests // 429
    InvalidArgument // 400
)
```

### 拦截器

实现 `Interceptor` 接口进行请求拦截：

```go
package interceptor

import (
    "github.com/caiflower/common-tools/web"
    "github.com/caiflower/common-tools/web/e"
    "github.com/caiflower/common-tools/web/interceptor"
)

type LoggingInterceptor struct {
}

func (l *LoggingInterceptor) Before(ctx *web.Context) e.ApiError {
    // 业务执行前
    return nil
}

func (l *LoggingInterceptor) After(ctx *web.Context, err e.ApiError) e.ApiError {
    // 业务执行后
    return err
}

func (l *LoggingInterceptor) OnPanic(ctx *web.Context, err interface{}) e.ApiError {
    // 发生panic时执行
    return e.NewApiError(e.Internal, "Internal error", nil)
}

// 注册拦截器
webv1.AddInterceptor(&LoggingInterceptor{}, 1)
```

#### Web Context用法

```go
type MyReq struct {
    Name string `json:"name"`
    web.Context // 嵌入Context获取上下文
}

func (c *Controller) MyAction(req *MyReq) (interface{}, error) {
    // 获取请求信息
    path := req.GetPath()           // 获取请求路径
    params := req.GetParams()       // 获取查询参数
    pathParams := req.GetPathParams() // 获取路径参数
    method := req.GetMethod()       // 获取HTTP方法
    action := req.GetAction()       // 获取Action名称
    version := req.GetVersion()     // 获取API版本
    w, r := req.GetResponseWriterAndRequest() // 获取原始http对象
    
    // 设置自定义属性
    req.Put("key", "value")
    value := req.Get("key")
    
    return nil, nil
}
```

---

## 高级特性

### 限流配置

```go
config := webv1.Config{
    WebLimiter: webv1.WebLimiter{
        Enable: true,
        Qos:    1000, // 每秒最多处理1000个请求
    },
}

server := webv1.InitDefaultHttpServer(config)
server.StartUp()
```

超出限流的请求返回429 TooManyRequests错误。

### 性能监控

框架内置Prometheus指标导出：

```
GET /metrics
```

返回HTTP请求的性能指标。

### 性能分析 (Pprof)

启用Pprof支持分析程序性能：

```go
config := webv1.Config{
    EnablePprof: true,
}

server := webv1.InitDefaultHttpServer(config)
server.StartUp()

// 访问 http://localhost:8080/debug/pprof/
```

### 请求追踪

框架自动为每个请求生成唯一的追踪ID：

```go
config := webv1.Config{
    HeaderTraceID: "X-Request-Id", // 追踪ID请求头名称
}
```

响应中会自动包含请求ID：

```json
{
    "requestId": "550e8400-e29b-41d4-a716-446655440000",
    "data": {}
}
```

### 压缩支持

框架自动支持 gzip 和 brotli 压缩：

- 请求时，通过 `Content-Encoding: gzip` 或 `Content-Encoding: br` 发送
- 响应时，通过 `Accept-Encoding` 请求头自动选择压缩算法

### 自定义前置回调

在请求分发前执行自定义逻辑：

```go
server.SetBeforeDispatchCallBack(func(w http.ResponseWriter, r *http.Request) bool {
    // 返回true中断请求处理
    // 返回false继续处理
    return false
})
```

### 优雅关闭

```go
server.Close()
```

30秒超时内完成优雅关闭，处理完所有已接收的请求。

---

## 完整示例

```go
package main

import (
    "github.com/caiflower/common-tools/web/v1"
    "github.com/caiflower/common-tools/web/e"
)

// 定义请求和响应
type CreateUserReq struct {
    Name  string `json:"name" verf:""`
    Email string `json:"email" verf:""`
}

type User struct {
    ID    int    `json:"id"`
    Name  string `json:"name"`
    Email string `json:"email"`
}

// 定义Controller
type UserController struct {
}

func (c *UserController) CreateUser(req *CreateUserReq) (*User, error) {
    return &User{
        ID:    1,
        Name:  req.Name,
        Email: req.Email,
    }, nil
}

func (c *UserController) GetUser(req *GetUserReq) (*User, error) {
    return &User{
        ID:    req.ID,
        Name:  "John Doe",
        Email: "john@example.com",
    }, nil
}

type GetUserReq struct {
    ID int `json:"id" verf:""`
}

func main() {
    // 初始化配置
    config := webv1.Config{
        Name:     "user-service",
        Port:     8080,
        RootPath: "api",
    }

    // 初始化服务器
    server := webv1.InitDefaultHttpServer(config)

    // 注册Controller
    server.AddController(&UserController{})

    // 注册RESTful路由
    webv1.Register(
        webv1.NewRestFul().
            Version("/v1").
            Method("POST").
            Path("/users").
            Controller("UserController").
            Action("CreateUser"),
    )

    // 启动服务器
    server.StartUp()

    // 阻止程序退出
    select {}
}
```

---

## 配置选项详解

| 配置项 | 类型 | 默认值 | 说明 |
|--------|------|--------|------|
| `Name` | string | "default" | 服务器名称 |
| `Port` | uint | 8080 | 监听端口 |
| `ReadTimeout` | uint | 20 | 读取超时（秒）|
| `WriteTimeout` | uint | 35 | 写入超时（秒）|
| `HandleTimeout` | *uint | 60 | 请求总处理超时（秒）|
| `RootPath` | string | "" | API根路径前缀 |
| `HeaderTraceID` | string | "X-Request-Id" | 追踪ID请求头 |
| `ControllerRootPkgName` | string | "controller" | Controller包根名称 |
| `WebLimiter.Enable` | bool | false | 是否启用限流 |
| `WebLimiter.Qos` | int | 1000 | 限流QoS（每秒请求数）|
| `EnablePprof` | bool | false | 是否启用性能分析 |

---

## 常见问题

### Q: 如何同时支持Action和RESTful风格？
A: 可以同时注册两种风格的路由。框架根据是否提供`action`参数来区分。

### Q: 是否支持WebSocket？
A: 框架设计用于RESTful API，不原生支持WebSocket。可在拦截器中通过`UpgradeWebsocket()`升级连接后自定义处理。


