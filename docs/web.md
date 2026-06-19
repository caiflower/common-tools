# Web框架使用指南

## 概述

Web包是一个高性能的RESTful Web框架，提供HTTP服务器、请求路由、参数校验、中间件、HTTP客户端、GRPC集成等功能。

### 核心特性

- 🚀 **高性能**：支持 Netpoll 和 Standard 两种服务器模式
- 🎯 **中间件**：基于 `ctx.Next()` 的中间件链，灵活的请求处理
- 🛣️ **RouterGroup**：支持路由分组、嵌套、组级别中间件
- ✅ **参数校验**：内置强大的参数验证系统
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

### 2. 定义Handler

框架支持多种 handler 签名，通过 `engine.GET`/`engine.POST` 等方法自动识别：

```go
package handler

import (
    "github.com/caiflower/common-tools/web/common/e"
)

// 定义请求参数结构体
type CreateUserReq struct {
    Name  string `json:"name" verf:"required"`
    Email string `json:"email" verf:"required"`
}

type GetUserReq struct {
    ID string `path:"userId" verf:"required"`
}

// 定义响应结构体
type User struct {
    ID    int    `json:"id"`
    Name  string `json:"name"`
    Email string `json:"email"`
}

// 方式1：返回 (data, error)
func CreateUser(ctx context.Context, req *CreateUserReq) (*User, error) {
    return &User{ID: 1, Name: req.Name, Email: req.Email}, nil
}

// 方式2：返回 ApiError
func DeleteUser(ctx context.Context, req *GetUserReq) (interface{}, e.ApiError) {
    if req.ID == "" {
        return nil, e.NewApiError(e.InvalidArgument, "Invalid ID", nil)
    }
    return nil, nil
}

// 方式3：使用 app.HandlerFunc 中间件风格
func ListUsers(ctx context.Context, reqCtx *app.RequestContext) {
    reqCtx.JSON(200, map[string]interface{}{
        "users": []string{"Alice", "Bob"},
    })
}
```

### 3. 注册路由

使用 `engine.GET`/`engine.POST` 等方法注册路由，handler 类型自动识别：

```go
engine := web.Default(
    config.WithAddr(":8080"),
    config.WithName("myapp"),
)

// 方式1：app.HandlerFunc 中间件风格（ctx 控制链执行）
engine.GET("/ping", func(ctx context.Context, reqCtx *app.RequestContext) {
    reqCtx.JSON(200, map[string]string{"msg": "pong"})
})

// 方式2：普通函数风格（自动绑定参数、校验、序列化响应）
engine.POST("/users", func(req *CreateUserReq) (*User, error) {
    return &User{ID: 1, Name: req.Name}, nil
})

// 方式3：结构体方法值（推荐，避免反射）
uc := &UserController{}
engine.POST("/users", uc.CreateUser)
engine.GET("/users/:userId", uc.GetUser)

// 使用路由组
api := engine.Group("/api/v1")
api.POST("/users", uc.CreateUser)
api.GET("/users/:userId", uc.GetUser)

// 使用 Handle 指定自定义 HTTP 方法
engine.Handle("GET", "/custom", func(ctx context.Context, reqCtx *app.RequestContext) {
    reqCtx.JSON(200, map[string]string{"custom": "true"})
})
```

### 4. 注册 GRPC 服务（推荐）

> **推荐使用 GRPC 注册方式**：相比普通函数注册，GRPC 方式直接使用 proto 生成的 Handler，减少一次反射调用，性能更优。

框架支持将 GRPC 服务直接注册为 HTTP 路由：

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

// 注册 GRPC 路由（支持 GET/POST/PUT/PATCH 等 HTTP 方法）
helloService := &HelloServiceImpl{}
engine.GRPC("POST", "/v1/hello", pb.HelloService_SayHello_Handler, helloService)
engine.GRPC("GET", "/v1/search", pb.HelloService_Search_Handler, helloService)
```

### 5. 使用HTTP客户端

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

    SetBeforeDispatchCallBack(callbackFunc router.CallbackFunc)
    SetAfterDispatchCallBack(callbackFunc router.CallbackFunc)

    AddProtocol(protocol string, core protocol.Server)
}
```

### Engine 路由与中间件方法

`Engine` 提供路由注册和中间件方法：

```go
// 注册全局中间件
engine.Use(middleware ...app.HandlerFunc) IRoutes

// 创建路由组
engine.Group(relativePath string, handlers ...app.HandlerFunc) *RouterGroup

// 路由注册（handler 类型自动识别：函数/结构体指针/HandlerFunc）
engine.GET(relativePath string, handlers ...interface{}) IRoutes
engine.POST(relativePath string, handlers ...interface{}) IRoutes
engine.PUT(relativePath string, handlers ...interface{}) IRoutes
engine.DELETE(relativePath string, handlers ...interface{}) IRoutes
engine.PATCH(relativePath string, handlers ...interface{}) IRoutes
engine.OPTIONS(relativePath string, handlers ...interface{}) IRoutes
engine.HEAD(relativePath string, handlers ...interface{}) IRoutes
engine.Any(relativePath string, handlers ...interface{}) IRoutes
engine.Handle(httpMethod, relativePath string, handlers ...interface{}) IRoutes

// GRPC 路由注册（推荐：直接使用 proto Handler，减少一次反射）
engine.GRPC(httpMethod, relativePath string, handler grpc.MethodHandler, srv interface{}) IRoutes

// 获取根路由组
engine.RouterGroup() *RouterGroup
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

#### 查询参数绑定（GET请求）

使用 `json` tag 或 `query` tag 绑定查询参数。

```go
type SearchReq struct {
    Keyword string `query:"keyword"` // 推荐使用 query tag
    Page    int    `json:"page"`     // 兼容 json tag
}

func Search(ctx context.Context, req *SearchReq) (interface{}, error) {
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

func GetProduct(ctx context.Context, req *GetProductReq) (*Product, error) {
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
func Method1(ctx context.Context, req *Req) (*Resp, error) {
    return nil, fmt.Errorf("error message")
}

// 方式2：返回 ApiError
func Method2(ctx context.Context, req *Req) (*Resp, e.ApiError) {
    return nil, e.NewApiError(e.InvalidArgument, "Invalid argument", nil)
}

// 方式3：返回 (data, error)
func Method3(ctx context.Context, req *Req) (*Resp, e.ApiError) {
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

## 中间件

框架通过 `Use()` 注册中间件，中间件通过 `ctx.Next()` 控制链执行，与 Hertz/Gin 风格一致。

### 注册中间件

```go
engine := web.Default(
    config.WithAddr(":8080"),
    config.WithName("myapp"),
    config.WithRootPath(""),
)

// 全局中间件
engine.Use(func(ctx context.Context, reqCtx *app.RequestContext) {
    start := time.Now()
    reqCtx.Next(ctx) // 调用后续 handler
    latency := time.Since(start)
    logger.Info("| %3d | %13v | %15s | %7s %s",
        reqCtx.GetStatusCode(), latency, reqCtx.ClientIP(), reqCtx.GetMethod(), reqCtx.GetPath())
})
```

### ctx.Next() 执行模型

中间件通过 `ctx.Next()` 驱动后续 handler 执行。`Next()` 之前的代码在 handler 前执行，`Next()` 之后的代码在 handler 后执行：

```go
engine.Use(func(ctx context.Context, reqCtx *app.RequestContext) {
    // Before: handler 执行前
    fmt.Println("middleware before")

    reqCtx.Next(ctx) // 执行后续中间件和 handler

    // After: handler 执行后
    fmt.Println("middleware after")
})
```

多个中间件按注册顺序形成链式调用：

```
mw1-before → mw2-before → handler → mw2-after → mw1-after
```

### 中断链执行

中间件不调用 `ctx.Next()` 时，后续中间件和 handler 都不会执行：

```go
// 鉴权中间件：验证失败则中断
engine.Use(func(ctx context.Context, reqCtx *app.RequestContext) {
    token := reqCtx.GetHeader("Authorization")
    if token == "" {
        reqCtx.AbortWithMsg("unauthorized", 401)
        return // 不调用 Next()，链中断
    }
    reqCtx.Next(ctx)
})
```

使用 `Abort()` 也可中断链执行：

```go
engine.Use(func(ctx context.Context, reqCtx *app.RequestContext) {
    reqCtx.AbortWithMsg("forbidden", 403)
    reqCtx.Next(ctx) // Abort 后 Next() 不会执行后续 handler
})
```

### 路由组中间件

`Group()` 创建路由组，支持组级别中间件：

```go
api := engine.Group("/api")

// 路由组级别中间件
api.Use(func(ctx context.Context, reqCtx *app.RequestContext) {
    reqCtx.SetHeader("X-API-Version", "v1")
    reqCtx.Next(ctx)
})

// 路由组内的路由会继承组中间件
api.GET("/users", userHandler)
api.POST("/users", createUserHandler)

// 嵌套路由组
v1 := api.Group("/v1")
v1.Use(authMiddleware) // v1 组专属中间件
v1.GET("/profile", profileHandler)
```

### Panic 恢复中间件

通过 `defer recover()` 实现 `OnPanic` 的等效功能：

```go
engine.Use(func(ctx context.Context, reqCtx *app.RequestContext) {
    defer func() {
        if err := recover(); err != nil {
            logger.Error("panic recovered: %v", err)
            reqCtx.AbortWithMsg("internal server error", 500)
        }
    }()
    reqCtx.Next(ctx)
})
```

---

### Web Context用法

通过在 Request 结构体中嵌入 `app.Context` 来获取上下文：

```go
import "github.com/caiflower/common-tools/web/app"

type MyReq struct {
    Name string `json:"name"`
    app.Context // 嵌入Context获取上下文
}

func MyHandler(ctx context.Context, req *MyReq) (interface{}, error) {
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

| Option函数          | 参数   | 默认值         | 说明          |
| ------------------- | ------ | -------------- | ------------- |
| `WithRootPath`      | string | ""             | API根路径前缀 |
| `WithHeaderTraceID` | string | "X-Request-Id" | 追踪ID请求头  |

### 功能开关

| Option函数                   | 参数 | 默认值 | 说明                       |
| ---------------------------- | ---- | ------ | -------------------------- |
| `WithEnablePprof`            | bool | false  | 是否启用性能分析           |
| `WithEnableMetrics`          | bool | false  | 是否启用 Prometheus 指标   |
| `WithEnableSwagger`          | bool | false  | 是否启用 Swagger 文档        |
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