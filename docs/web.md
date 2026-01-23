# Web框架使用指南

## 概述

Web包是一个高性能的RESTful Web框架，提供HTTP服务器、请求路由、参数校验、拦截器、HTTP客户端、GRPC集成等功能。支持两种请求风格：

- **Action风格**：基于查询参数 `?action=xxx` 的传统风格
- **RESTful风格**：基于HTTP方法和路径的REST API风格

### 核心特性

- 🚀 **高性能**：支持 Netpoll 和 Standard 两种服务器模式
- 🔄 **双路由**：Action 和 RESTful 两种风格并存
- ✅ **参数校验**：内置强大的参数验证系统
- 🎯 **拦截器**：AOP 支持，灵活的请求处理链
- 🌐 **HTTP客户端**：内置高性能 HTTP 客户端
- 🔌 **GRPC集成**：无缝集成 GRPC 服务
- 📊 **监控**：内置 Prometheus 指标导出
- 🚦 **限流**：令牌桶限流机制
- 🗜️ **压缩**：Gzip、Brotli 压缩支持
- 📖 **Swagger**：自动生成 OpenAPI 3.0 文档

---

## 性能基准

在CPU为Intel(R) Xeon(R) Platinum 8338C CPU，2c4g条件下，使用wrk工具压测，结果如下：

```bash
# wrk -t12 -c500 -d60s http://127.0.0.1:30461/v1/hobby/search?query=1\&page_number=1\&hobby=math
Running 1m test @ http://127.0.0.1:30461/v1/hobby/search?query=1&page_number=1&hobby=math
  12 threads and 500 connections
  Thread Stats   Avg      Stdev     Max   +/- Stdev
    Latency    13.99ms   16.47ms 211.61ms   81.17%
    Req/Sec     5.34k   675.51    43.92k    88.50%
  3829089 requests in 1.00m, 752.25MB read
Requests/sec:  63778.36
Transfer/sec:     12.53MB
```

## 快速开始

### 1. 初始化HTTP服务器

使用 `web.Default` 方法并通过 Option 模式进行配置初始化。

```go
import (
    "github.com/caiflower/common-tools/web"
    "github.com/caiflower/common-tools/web/app/server/config"
)

// 初始化服务器
server := web.Default(
    config.WithName("myapp"),
    config.WithAddr(":8080"),
    config.WithMode(config.ServerModeNetpoll), 
    config.WithReadTimeout(20 * time.Second),
    config.WithWriteTimeout(35 * time.Second),
    config.WithEnablePprof(false),
    config.WithEnableSwagger(true),
    config.WithQps(true, 1000),
)

server.Start()
```

### 2. 定义Controller

#### Action风格示例

```go
package controller

import (
    "github.com/caiflower/common-tools/web/common/e"
)

type UserController struct {
}

// 定义请求参数结构体
type GetUserReq struct {
    ID int `json:"id" verf:"required"`
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
func (c *UserController) DeleteUser(req *GetUserReq) (interface{}, e.ApiError) {
    if req.ID <= 0 {
        return nil, e.NewApiError(e.InvalidArgument, "Invalid ID", nil)
    }
    return nil, nil
}
```

**Action风格请求**：

```
POST /api/UserController?Action=GetUser
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
    Name  string  `json:"name" verf:"required"`
    Price float64 `json:"price" verf:"required"`
}

type GetProductReq struct {
    ID string `path:"productId" verf:"required"`
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
// 注册 Controller 到服务器
// 这会自动注册 Action 风格的路由，并返回 *controller.Controller 实例供 RESTful 注册使用
userController := server.AddController(&UserController{})
productController := server.AddController(&ProductController{})
```

### 4. 注册RESTful路由

RESTful 路由需要显式注册，通过 `controller.NewRestFul()` 构建路由规则，并绑定到具体的 Controller 方法上。

```go
import (
    "github.com/caiflower/common-tools/web/router/controller"
)

// 创建路由组
group := controller.NewRestFul().Group("/v1/products")

// 注册 POST /v1/products
server.Register(group.
    Method("POST").
    RegisterMethod(productController.GetMethod("CreateProduct")),
)

// 注册 GET /v1/products/:productId
server.Register(group.
    Method("GET").
    Path("/:productId").
    RegisterMethod(productController.GetMethod("GetProductByID")),
)
```

**RESTful风格请求**：

```
POST /api/v1/products HTTP/1.1
Content-Type: application/json

{
    "name": "Product Name",
    "price": 99.99
}

GET /api/v1/products/prod-123 HTTP/1.1
```

### 5. 注册GRPC服务

框架支持将 GRPC 服务注册为 HTTP 接口：

```go
import (
    "google.golang.org/grpc"
    pb "path/to/your/proto"
)

// 定义 GRPC 服务实现
type HelloServiceImpl struct {
    pb.UnimplementedHelloServiceServer
}

func (s *HelloServiceImpl) SayHello(ctx context.Context, req *pb.HelloRequest) (*pb.HelloReply, error) {
    return &pb.HelloReply{Message: "Hello " + req.Name}, nil
}

// 注册 GRPC 服务
helloService := &HelloServiceImpl{}
helloController := server.RegisterGRPCService(&pb.HelloService_ServiceDesc, helloService)

// 将 GRPC 方法注册为 RESTful 路由
group := controller.NewRestFul().Group("/v1")
server.Register(group.
    Method("POST").
    Path("/hello").
    RegisterGrpcMethod(helloController.GetGrpcMethodDesc("SayHello")),
)
```

### 6. 使用HTTP客户端

框架内置高性能 HTTP 客户端：

```go
import (
    "context"
    "github.com/caiflower/common-tools/web/app/client"
    "github.com/caiflower/common-tools/web/protocol"
)

// 创建客户端
c, _ := client.NewClient(
    client.WithDialTimeout(5 * time.Second),
    client.WithMaxConnsPerHost(100),
    client.WithClientReadTimeout(10 * time.Second),
)

// GET 请求
statusCode, body, err := c.Get(context.Background(), nil, "http://example.com/api/data")

// POST 请求
req := protocol.NewRequest("POST", "http://example.com/api/create", strings.NewReader(`{"name":"test"}`))
req.Header.SetContentType("application/json")
resp := protocol.AcquireResponse()
err = c.Do(context.Background(), req, resp)

// 设置代理
c.SetProxy(func(req *protocol.Request) (*protocol.URI, error) {
    return protocol.ParseRequestURI("http://proxy.example.com:8080")
})

// 设置重试策略
c.SetRetryIfFunc(func(req *protocol.Request, resp *protocol.Response, err error) bool {
    return err != nil || resp.StatusCode() >= 500
})
```

---

## 核心接口

### ICore 接口

```go
type Core interface {
    Name() string
    Start() error
    Close()

    AddController(v interface{}) *controller.Controller
    RegisterGRPCService(serviceDesc *grpc.ServiceDesc, srv interface{}) *controller.Controller
    Register(ctl *controller.RestfulController)

    AddInterceptor(i interceptor.Interceptor, order int)

    SetBeforeDispatchCallBack(callbackFunc router.CallbackFunc)
    SetAfterDispatchCallBack(callbackFunc router.CallbackFunc)

    AddProtocol(protocol string, core protocol.Server)
}
```

### Interceptor 接口

```go
type Interceptor interface {
    Before(ctx *app.Context) e.ApiError                   // 执行业务前执行
    After(ctx *app.Context, err e.ApiError) e.ApiError    // 执行业务后执行，参数err为业务返回的ApiErr信息
    OnPanic(ctx *app.Context, err interface{}) e.ApiError // 发生panic时执行
}
```

---

## 请求参数绑定

支持从多个来源自动绑定参数：

#### JSON Body绑定（POST/PUT/PATCH/DELETE）

```go
type UserReq struct {
    Name  string `json:"name"`
    Email string `json:"email"`
}
```

#### 查询参数绑定（GET/Action风格）

使用 `json` tag 或 `query` tag 绑定查询参数。

```go
type SearchReq struct {
    Keyword string `query:"keyword"` // 推荐使用 query tag
    Page    int    `json:"page"`     // 兼容 json tag
}

func (c *UserController) Search(req *SearchReq) (interface{}, error) {
    // ?keyword=test&page=1
    return nil, nil
}
```

#### 路径参数绑定（RESTful风格）

使用 `path` tag 绑定 URL 路径参数。

```go
type GetProductReq struct {
    ProductID    string `path:"productId"`
    SubProductID string `path:"subProductId"`
}

func (c *ProductController) GetProduct(req *GetProductReq) (*Product, error) {
    // 对应路由路径：/products/:productId/sub/:subProductId
    return &Product{ID: req.ProductID}, nil
}
```

#### 请求头绑定

```go
type AuthReq struct {
    Authorization string `header:"Authorization"`
    ContentType   string `header:"Content-Type"`
}
```

---

## 参数校验

框架支持丰富的参数校验标签 `verf`：

#### 必填校验

```go
type UserReq struct {
    Name string `json:"name" verf:"required"` // 必填
}
```

#### 枚举值校验

```go
type OrderReq struct {
    Status string `json:"status" verf:"inList:pending,processing,completed"`
}
```

#### 正则表达式校验

```go
type EmailReq struct {
    Email string `json:"email" verf:"reg:^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}$"`
}
```

#### 范围校验

```go
type AgeReq struct {
    Age int `json:"age" verf:"between:0,150"`
}
```

#### 长度校验

```go
type PasswordReq struct {
    Password string `json:"password" verf:"len:8,32"` // 长度8-32
}
```

#### 数组元素长度校验

```go
type TagsReq struct {
    Tags []string `json:"tags" verf:"itemLen:1,20"` // 每个元素长度1-20
}
```

#### 可选字段

```go
type FilterReq struct {
    Category *string `json:"category" verf:"nilable"` // 可选，可以为nil
}
```

---

## 响应格式

框架自动将方法返回值封装为统一的响应格式：

#### 成功响应

```json
{
    "requestId": "550e8400-e29b-41d4-a716-4466554400000",
    "data": {
        "id": 1,
        "name": "Product Name"
    }
}
```

#### 错误响应

```json
{
    "requestId": "550e8400-e29b-41d4-a716-4466554400000",
    "error": {
        "code": 400,
        "type": "InvalidArgument",
        "message": "Invalid input parameters"
    }
}
```

---

## 错误处理

支持多种错误返回方式，推荐使用 `web/common/e` 包中的错误类型：

```go
import "github.com/caiflower/common-tools/web/common/e"

// 方式1：返回 error
func (c *Controller) Method1(req *Req) (*Resp, error) {
    return nil, fmt.Errorf("error message")
}

// 方式2：返回 ApiError
func (c *Controller) Method2(req *Req) (*Resp, e.ApiError) {
    return nil, e.NewApiError(e.InvalidArgument, "Invalid argument", nil)
}

// 方式3：返回 (data, error)
func (c *Controller) Method3(req *Req) (*Resp, e.ApiError) {
    if req.ID <= 0 {
        return nil, e.NewApiError(e.InvalidArgument, "ID must be positive", nil)
    }
    return &Resp{ID: req.ID}, nil
}
```

### 预定义错误类型

框架提供了丰富的预定义错误类型：

| 错误类型               | HTTP状态码 | 说明         |
| ---------------------- | ---------- | ------------ |
| `e.NotFound`           | 404        | 资源未找到   |
| `e.NotAcceptable`      | 406        | 不可接受     |
| `e.Unauthorized`       | 401        | 未授权       |
| `e.Forbidden`          | 403        | 禁止访问     |
| `e.InvalidArgument`    | 400        | 参数错误     |
| `e.TooManyRequests`    | 429        | 请求过多     |
| `e.Internal`           | 500        | 内部错误     |
| `e.Unavailable`        | 503        | 服务不可用   |
| `e.Timeout`            | 408        | 请求超时     |
| `e.Conflict`           | 400        | 冲突         |
| `e.FailedPrecondition` | 412        | 前置条件失败 |
| `e.OutOfRange`         | 400        | 超出范围     |
| `e.Unimplemented`      | 501        | 未实现       |
| `e.Aborted`            | 500        | 已中止       |
| `e.DataLoss`           | 500        | 数据丢失     |
| `e.Unknown`            | 500        | 未知错误     |

### gRPC 错误码转换

框架支持将 gRPC 错误码转换为自定义错误类型：

```go
// 将 gRPC 错误转换为自定义错误
err := e.ConvertErrorToErrorCode(grpcErr)

// 将自定义错误转换为 gRPC Status
status := apiError.GRPCStatus()
```

---

## 拦截器

实现 `Interceptor` 接口进行请求拦截：

```go
package interceptor

import (
    "github.com/caiflower/common-tools/web/common/e"
    "github.com/caiflower/common-tools/web/app"
    "github.com/caiflower/common-tools/web/common/interceptor"
)

type LoggingInterceptor struct {
}

func (l *LoggingInterceptor) Before(ctx *app.Context) e.ApiError {
    // 业务执行前
    return nil
}

func (l *LoggingInterceptor) After(ctx *app.Context, err e.ApiError) e.ApiError {
    // 业务执行后
    return err
}

func (l *LoggingInterceptor) OnPanic(ctx *app.Context, err interface{}) e.ApiError {
    // 发生panic时执行
    return e.NewApiError(e.Internal, "Internal error", nil)
}

// 注册拦截器
server.AddInterceptor(&LoggingInterceptor{}, 1)
```

### 中断请求

在拦截器中可以中断请求：

```go
func (l *AuthInterceptor) Before(ctx *app.Context) e.ApiError {
    token := ctx.Get("Authorization")
    if token == nil {
        ctx.Abort()  // 中断请求
        return e.NewApiError(e.Unauthorized, "Unauthorized", nil)
    }
    return nil
}
```

### Web Context用法

通过在 Request 结构体中嵌入 `app.Context` 来获取上下文：

```go
import "github.com/caiflower/common-tools/web/app"

type MyReq struct {
    Name string `json:"name"`
    app.Context // 嵌入Context获取上下文
}

func (c *Controller) MyAction(req *MyReq) (interface{}, error) {
    // 获取请求信息
    path := req.GetPath()           // 获取请求路径
    params := req.GetParams()       // 获取查询参数
    method := req.GetMethod()       // 获取HTTP方法
    
    // 获取原始http对象
    w, r := req.GetResponseWriterAndRequest() 
    
    // 设置自定义属性
    req.Put("key", "value")
    value := req.Get("key")
    
    // 中断请求
    req.Abort()
    
    return nil, nil
}
```

---

## 高级特性

### 限流配置

```go
server := web.Default(
    config.WithQps(true, 1000), // 开启限流，每秒1000请求
)
```

超出限流的请求返回429 TooManyRequests错误。

### 性能监控

框架内置Prometheus指标导出，可通过 `EnableMetrics` 选项开启。

```go
server := web.Default(
    config.WithEnableMetrics(true),
)
```

访问 `GET /metrics` 获取指标数据。

### Swagger/OpenAPI 文档

框架支持自动生成 OpenAPI 3.0 文档：

```go
server := web.Default(
    config.WithEnableSwagger(true),
)
```

访问 `GET /swagger/json` 获取 Swagger json。

### 性能分析 (Pprof)

启用Pprof支持分析程序性能：

```go
server := web.Default(
    config.WithEnablePprof(true),
)
```

访问以下端点进行性能分析：

- `http://localhost:8080/debug/pprof/` - 概览
- `http://localhost:8080/debug/pprof/profile` - CPU profile
- `http://localhost:8080/debug/pprof/heap` - 内存堆
- `http://localhost:8080/debug/pprof/goroutine` - Goroutine
- `http://localhost:8080/debug/pprof/trace` - 执行追踪

### 请求追踪

从Header key = X-Request-Id获取traceID

```go
server := web.Default(
    config.WithHeaderTraceID("X-Request-Id"),
)
```

### 自定义回调

在请求分发前后执行自定义逻辑：

```go
// 分发前回调
server.SetBeforeDispatchCallBack(func(w http.ResponseWriter, r *http.Request) bool {
    // 返回true中断请求处理
    // 返回false继续处理
    return false
})

// 分发后回调（仅 Netpoll 模式支持）
server.SetAfterDispatchCallBack(func(w http.ResponseWriter, r *http.Request) bool {
    // 自定义响应格式
    return false
})
```

### TLS/HTTPS 配置

```go
import "crypto/tls"

server := web.Default(
    config.WithALPN(true, &tls.Config{
        Certificates: []tls.Certificate{
            // 加载证书
        },
    }),
)
```

### HTTP/2 配置

```go
server := web.Default(
    config.WithH2C(true), // 启用 HTTP/2 Cleartext
    config.WithMaxConcurrentStreams(1000),
    config.WithMaxUploadBufferPerConnection(1 << 20),
    config.WithMaxUploadBufferPerStream(1 << 18),
)
```

### 连接管理

```go
server := web.Default(
    config.WithDisableKeepalive(false), // 启用 Keep-Alive
    config.WithIdleTimeout(60 * time.Second), // 空闲超时
    config.WithKeepAliveTimeout(180 * time.Second), // Keep-Alive 超时
)
```

### 自定义连接回调

```go
server := web.Default(
    config.WithOnAccept(func(conn net.Conn) context.Context {
        // 连接接受时回调
        return context.Background()
    }),
    config.WithOnConnect(func(ctx context.Context, conn network.Conn) context.Context {
        // 连接建立时回调
        return ctx
    }),
)
```

### 请求大小限制

```go
server := web.Default(
    config.WithMaxHeaderBytes(1 << 20), // 最大请求头大小 1MB
    config.WithMaxRequestBodySize(10 << 20), // 最大请求体大小 10MB
)
```

### 服务器模式选择

```go
// 标准模式（基于 net/http）
server := web.Default(
    config.WithMode(config.ServerModeStandard),
)

// Netpoll 模式（高性能，基于 CloudWeGo）
server := web.Default(
    config.WithMode(config.ServerModeNetpoll),
)
```

---

## 配置选项详解

`web/app/server/config` 包提供了多种 Option 函数：

### 服务器基础配置

| Option函数    | 参数       | 默认值    | 说明                          |
| ------------- | ---------- | --------- | ----------------------------- |
| `WithName`    | string     | "default" | 服务器名称                    |
| `WithAddr`    | string     | ":8080"   | 监听地址                      |
| `WithMode`    | ServerMode | "netpoll"  | 服务器模式 (Standard/Netpoll) |
| `WithNetwork` | string     | "tcp"     | 网络类型                      |

### 超时配置

| Option函数             | 参数     | 默认值 | 说明            |
| ---------------------- | -------- | ------ | --------------- |
| `WithReadTimeout`      | duration | 20s    | 读取超时        |
| `WithWriteTimeout`     | duration | 35s    | 写入超时        |
| `WithHandleTimeout`    | duration | 60s    | 请求总处理超时  |
| `WithIdleTimeout`      | duration | 60s    | 空闲连接超时    |
| `WithKeepAliveTimeout` | duration | 180s   | Keep-Alive 超时 |

### 路由配置

| Option函数                  | 参数   | 默认值         | 说明               |
| --------------------------- | ------ | -------------- | ------------------ |
| `WithRootPath`              | string | ""             | API根路径前缀      |
| `WithHeaderTraceID`         | string | "X-Request-Id" | 追踪ID请求头       |
| `WithControllerRootPkgName` | string | "controller"   | Controller包根名称 |

### 功能开关

| Option函数                   | 参数 | 默认值 | 说明                       |
| ---------------------------- | ---- | ------ | -------------------------- |
| `WithEnablePprof`            | bool | false  | 是否启用性能分析           |
| `WithEnableMetrics`          | bool | false  | 是否启用 Prometheus 指标   |
| `WithEnableSwagger`          | bool | false  | 是否启用 Swagger 文档        |
| `WithEnableActionController` | bool | false   | 是否启用 Action 风格控制器 |
| `WithDisableOptimization`    | bool | false  | 是否禁用性能优化           |
| `WithDisableKeepalive`       | bool | false  | 是否禁用 Keep-Alive        |

### 限流配置

| Option函数 | 参数      | 默认值   | 说明                   |
| ---------- | --------- | -------- | ---------------------- |
| `WithQps`  | bool, int | false, 0 | 限流配置 (enable, qps) |

### HTTP/2 配置

| Option函数                         | 参数              | 默认值     | 说明                      |
| ---------------------------------- | ----------------- | ---------- | ------------------------- |
| `WithH2C`                          | bool              | false      | 是否启用 HTTP/2 Cleartext |
| `WithALPN`                         | bool, *tls.Config | false, nil | 是否启用 ALPN (TLS)       |
| `WithMaxConcurrentStreams`         | uint32            | 100        | 最大并发流数              |
| `WithMaxUploadBufferPerConnection` | int32             | 1<<20      | 每连接上传缓冲区大小      |
| `WithMaxUploadBufferPerStream`     | int32             | 1<<18      | 每流上传缓冲区大小        |
| `WithMaxReadFrameSize`             | uint32            | 0          | 最大读取帧大小            |
| `WithPermitProhibitedCipherSuites` | bool              | false      | 是否允许禁止的密码套件    |

### 连接管理

| Option函数                     | 参数                                                | 默认值 | 说明               |
| ------------------------------ | --------------------------------------------------- | ------ | ------------------ |
| `WithSenseClientDisconnection` | bool                                                | false  | 是否检测客户端断开 |
| `WithOnAccept`                 | func(net.Conn) context.Context                      | nil    | 连接接受回调       |
| `WithOnConnect`                | func(context.Context, network.Conn) context.Context | nil    | 连接建立回调       |
| `WithListenConfig`             | *net.ListenConfig                                   | nil    | 监听配置           |

### 请求限制

| Option函数               | 参数 | 默认值        | 说明           |
| ------------------------ | ---- | ------------- | -------------- |
| `WithMaxHeaderBytes`     | int  | 1<<20 (1MB)   | 最大请求头大小 |
| `WithMaxRequestBodySize` | int  | 10<<20 (10MB) | 最大请求体大小 |

---

## HTTP客户端配置选项

| Option函数                | 参数        | 默认值 | 说明                |
| ------------------------- | ----------- | ------ | ------------------- |
| `WithDialTimeout`       | duration    | 3s     | 连接超时            |
| `WithMaxConnsPerHost`   | int         | 512    | 每个主机最大连接数    |
| `WithMaxIdleConnDuration`| duration    | 10s    | 最大空闲连接时长      |
| `WithMaxConnDuration`    | duration    | 60s    | 最大连接时长         |
| `WithMaxConnWaitTimeout`| duration    | 5s     | 等待连接超时        |
| `WithKeepAlive`         | bool        | true    | 是否使用 Keep-Alive    |
| `WithClientReadTimeout`  | duration    | 0      | 读取超时            |
| `WithWriteTimeout`      | duration    | 0      | 写入超时            |
| `WithTLSConfig`         | *tls.Config | nil    | TLS 配置            |
| `WithDialer`           | Dialer      | nil    | 自定义拨号器         |
| `WithResponseBodyStream`| bool        | false  | 是否流式读取响应体   |
| `WithDisableHeaderNamesNormalizing`| bool| false  | 是否禁用请求头规范化  |
| `WithName`             | string      | ""      | 客户端名称          |
| `WithNoDefaultUserAgentHeader`| bool| false  | 是否禁用默认 User-Agent |
| `WithDisablePathNormalizing`| bool| false  | 是否禁用路径规范化    |
| `WithRetryConfig`      | ...retry.Option | -   | 重试配置            |
| `WithConnStateObserve` | HostClientStateFunc, interval | - | 连接状态观察        |
| `WithDialFunc`        | DialFunc, Dialer | - | 自定义拨号函数      |