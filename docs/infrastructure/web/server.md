# Web 服务端

本文档覆盖 `web` 包的服务端能力：服务器初始化、Handler 定义、路由注册、gRPC 注册、参数绑定与校验、响应与错误、中间件、高级特性和配置选项。

> 导入路径：
>
> - `github.com/caiflower/common-tools/web`
> - `github.com/caiflower/common-tools/web/app/server/config`
>
> 客户端用法见 [client.md](client.md)，命令行模式见 [cli.md](cli.md)。

## 目录

- [服务器初始化](#服务器初始化)
- [Handler 定义](#handler-定义)
- [路由注册](#路由注册)
- [gRPC 服务注册](#grpc-服务注册)
- [核心接口](#核心接口)
- [请求参数绑定](#请求参数绑定)
- [参数校验](#参数校验)
- [响应格式](#响应格式)
- [错误处理](#错误处理)
- [中间件](#中间件)
- [请求上下文](#请求上下文)
- [高级特性](#高级特性)
- [配置选项](#配置选项)
- [相关文档](#相关文档)

## 服务器初始化

使用 `web.Default` 并通过 Option 模式配置：

```go
import (
    "time"

    "github.com/caiflower/common-tools/web"
    "github.com/caiflower/common-tools/web/app/server/config"
)

server := web.Default(
    config.WithName("myapp"),
    config.WithAddr(":8080"),
    config.WithMode(config.ServerModeNetpoll),
    config.WithReadTimeout(20*time.Second),
    config.WithWriteTimeout(35*time.Second),
    config.WithEnablePprof(false),
    config.WithEnableSwagger(true),
    config.WithQps(true, 1000),
)

if err := server.Start(); err != nil {
    panic(err)
}
```

> [!NOTE]
> `Start()` 是非阻塞的：它启动监听后立即返回。生产环境建议把 engine 交给
> `global.DefaultResourceManger` 托管，由资源管理器统一启动和优雅退出：
>
> ```go
> global.DefaultResourceManger.AddDaemonWithOrder(server, global.OrderHTTPServer)
> global.DefaultResourceManger.Signal()
> ```

## Handler 定义

框架通过反射自动识别 handler 签名，并在调用前完成参数绑定、校验和响应序列化。

### 支持的签名

| 签名 | 说明 |
| --- | --- |
| `func(ctx context.Context, req *Req) (*Resp, error)` | 最常用；自动绑定 `req`，`error` 转为错误响应 |
| `func(ctx context.Context, req *Req) (interface{}, e.ApiError)` | 返回业务错误码，便于区分 HTTP 状态 |
| `func(req *Req) (*Resp, error)` | 省略 `ctx` 的简化写法 |
| `func(ctx context.Context, reqCtx *app.RequestContext)` | 中间件风格；直接操作请求上下文，不走参数绑定 |
| 结构体方法值 `uc.CreateUser` | 推荐；避免匿名函数带来的反射开销 |

### 请求与响应结构体

```go
type CreateUserReq struct {
    Name  string `json:"name" verf:"required"`
    Email string `json:"email" verf:"required"`
}

type GetUserReq struct {
    ID string `path:"userId" verf:"required"`
}

type User struct {
    ID    int    `json:"id"`
    Name  string `json:"name"`
    Email string `json:"email"`
}
```

### 三种写法

```go
import (
    "context"

    "github.com/caiflower/common-tools/web/app"
    "github.com/caiflower/common-tools/web/common/e"
)

// 方式 1：返回 (data, error)
func CreateUser(ctx context.Context, req *CreateUserReq) (*User, error) {
    return &User{ID: 1, Name: req.Name, Email: req.Email}, nil
}

// 方式 2：返回 ApiError
func DeleteUser(ctx context.Context, req *GetUserReq) (interface{}, e.ApiError) {
    if req.ID == "" {
        return nil, e.NewApiError(e.InvalidArgument, "Invalid ID", nil)
    }
    return nil, nil
}

// 方式 3：HandlerFunc 风格
func ListUsers(ctx context.Context, reqCtx *app.RequestContext) {
    reqCtx.JSON(200, map[string]interface{}{
        "users": []string{"Alice", "Bob"},
    })
}
```

## 路由注册

### 注册方式

```go
engine := web.Default(
    config.WithAddr(":8080"),
    config.WithName("myapp"),
)

// HandlerFunc 风格
engine.GET("/ping", func(ctx context.Context, reqCtx *app.RequestContext) {
    reqCtx.JSON(200, map[string]string{"msg": "pong"})
})

// 普通函数风格：自动绑定参数、校验、序列化响应
engine.POST("/users", func(req *CreateUserReq) (*User, error) {
    return &User{ID: 1, Name: req.Name}, nil
})

// 结构体方法值（推荐）
uc := &UserController{}
engine.POST("/users", uc.CreateUser)
engine.GET("/users/:userId", uc.GetUser)

// 自定义 HTTP 方法
engine.Handle("GET", "/custom", func(ctx context.Context, reqCtx *app.RequestContext) {
    reqCtx.JSON(200, map[string]string{"custom": "true"})
})
```

### 路由方法

| 方法 | 说明 |
| --- | --- |
| `GET` / `POST` / `PUT` / `DELETE` / `PATCH` / `OPTIONS` / `HEAD` | 注册对应 HTTP 方法的路由 |
| `Any` | 注册所有方法 |
| `Handle` | 指定任意合法 HTTP 方法 |
| `StaticFile` / `Static` / `StaticFS` | 注册静态文件或目录 |
| `GRPC` | 注册 gRPC Handler，见下节 |

### 路由组

```go
api := engine.Group("/api/v1")
api.Use(authMiddleware)

api.POST("/users", uc.CreateUser)
api.GET("/users/:userId", uc.GetUser)

// 嵌套路由组
internal := api.Group("/internal")
internal.Use(adminMiddleware)
```

路由组会继承父组的中间件；组内路由按注册顺序形成中间件链。

## gRPC 服务注册

相比普通函数注册，gRPC 方式直接使用 protobuf 生成的 Handler，减少一次反射调用，性能更优。

```go
import (
    "google.golang.org/grpc"
    pb "path/to/your/proto"
)

type HelloServiceImpl struct {
    pb.UnimplementedHelloServiceServer
}

func (s *HelloServiceImpl) SayHello(ctx context.Context, req *pb.HelloRequest) (*pb.HelloReply, error) {
    return &pb.HelloReply{Message: "Hello " + req.Name}, nil
}

helloService := &HelloServiceImpl{}
engine.GRPC("POST", "/v1/hello", pb.HelloService_SayHello_Handler, helloService)
engine.GRPC("GET", "/v1/search", pb.HelloService_Search_Handler, helloService)
```

### internal 包的 Wrapper

当 proto 生成的代码位于 `internal` 包中时，`_XxxService_Yyy_Handler` 无法直接导出。此时创建导出 wrapper，框架会按 `{ServiceName}Service{MethodName}Handler` 命名规则自动解析方法名：

```go
// internal/proto/handlers.go

// Wrapper 命名规则：{ServiceName}Service{MethodName}Handler
func ExecutionServiceGetHandler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
    return _ExecutionService_Get_Handler(srv, ctx, dec, interceptor)
}

func FlowServiceCreateHandler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
    return _FlowService_Create_Handler(srv, ctx, dec, interceptor)
}
```

注册时使用 wrapper 函数即可：

```go
import "your-project/internal/proto"

engine.GRPC("GET", "/v1/executions/:id", proto.ExecutionServiceGetHandler, &executionServiceImpl{})
engine.GRPC("POST", "/v1/flows", proto.FlowServiceCreateHandler, &flowServiceImpl{})
```

## 核心接口

### server.Core

`Engine` 内嵌 `server.Core`，提供服务器生命周期、控制器注册和协议扩展能力：

```go
type Core interface {
    Name() string
    Start() error
    Close()

    // 优雅退出顺序：HTTP 服务器总是最先关闭
    Order() int

    AddController(v interface{}) *controller.Controller
    RegisterGRPCService(serviceDesc *grpc.ServiceDesc, srv interface{}) *controller.Controller
    Register(ctl *controller.RestfulController)

    // Deprecated: 使用 RouterGroup.Use() + ctx.Next() 中间件
    AddInterceptor(i interceptor.Interceptor, order int)

    SetBeforeDispatchCallBack(callbackFunc router.CallbackFunc)
    // 仅 Netpoll 模式支持
    SetAfterDispatchCallBack(callbackFunc router.CallbackFunc)

    AddProtocol(protocol string, core protocol.Server)
}
```

### Engine 路由方法

```go
engine.Use(middleware ...app.HandlerFunc) router.IRoutes
engine.Group(relativePath string, handlers ...app.HandlerFunc) *router.RouterGroup

engine.GET(relativePath string, handlers ...interface{}) router.IRoutes
engine.POST(relativePath string, handlers ...interface{}) router.IRoutes
engine.PUT(relativePath string, handlers ...interface{}) router.IRoutes
engine.DELETE(relativePath string, handlers ...interface{}) router.IRoutes
engine.PATCH(relativePath string, handlers ...interface{}) router.IRoutes
engine.OPTIONS(relativePath string, handlers ...interface{}) router.IRoutes
engine.HEAD(relativePath string, handlers ...interface{}) router.IRoutes
engine.Any(relativePath string, handlers ...interface{}) router.IRoutes
engine.Handle(httpMethod, relativePath string, handlers ...interface{}) router.IRoutes

engine.GRPC(
    httpMethod, relativePath string,
    handler func(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error),
    srv interface{},
) router.IRoutes

engine.RouterGroup() *router.RouterGroup
```

## 请求参数绑定

框架根据 tag 从不同来源自动绑定参数：

| 来源 | Tag | 适用方法 |
| --- | --- | --- |
| JSON Body | `json` | POST / PUT / PATCH / DELETE |
| 查询参数 | `query` | GET / HEAD |
| 路径参数 | `path` | 所有方法 |
| 请求头 | `header` | 所有方法 |

### JSON Body

```go
type UserReq struct {
    Name  string `json:"name"`
    Email string `json:"email"`
}
```

### 查询参数

只有显式声明 `query` tag 的字段才会参与查询参数绑定；只写 `json` tag 的字段不会绑定。

```go
type SearchReq struct {
    Keyword string `query:"keyword"`
    Page    int    `query:"page"`
}

func Search(ctx context.Context, req *SearchReq) (interface{}, error) {
    // ?keyword=test&page=1
    return nil, nil
}
```

### 路径参数

```go
type GetProductReq struct {
    ProductID    string `path:"productId"`
    SubProductID string `path:"subProductId"`
}

func GetProduct(ctx context.Context, req *GetProductReq) (*Product, error) {
    // 对应路由：/products/:productId/sub/:subProductId
    return &Product{ID: req.ProductID}, nil
}
```

### 请求头

```go
type AuthReq struct {
    Authorization string `header:"Authorization"`
    ContentType   string `header:"Content-Type"`
}
```

## 参数校验

框架通过 `verf` tag 对绑定后的参数做校验。

| 规则 | 示例 | 说明 |
| --- | --- | --- |
| `required` | `verf:"required"` | 必填 |
| `inList` | `verf:"inList:pending,processing,completed"` | 枚举值 |
| `reg` | `verf:"reg:^[a-z]+$"` | 正则匹配 |
| `between` | `verf:"between:0,150"` | 数值范围 |
| `len` | `verf:"len:8,32"` | 字符串长度范围 |
| `itemLen` | `verf:"itemLen:1,20"` | 数组元素长度范围 |
| `nilable` | `verf:"nilable"` | 允许为 nil |

多条规则用 `|` 分隔：

```go
type CreateOrderReq struct {
    Status string   `json:"status" verf:"required|inList:pending,processing,completed"`
    Email  string   `json:"email" verf:"required|reg:^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}$"`
    Age    int      `json:"age" verf:"between:0,150"`
    Tags   []string `json:"tags" verf:"itemLen:1,20"`
}
```

校验失败会返回 `400 InvalidArgument`，错误信息包含具体字段。

## 响应格式

框架自动把 handler 返回值封装为统一结构。

### 成功响应

```json
{
    "requestId": "550e8400-e29b-41d4-a716-4466554400000",
    "data": {
        "id": 1,
        "name": "Product Name"
    }
}
```

### 错误响应

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

## 错误处理

推荐使用 `web/common/e` 包中的错误类型：

```go
import "github.com/caiflower/common-tools/web/common/e"

// 返回普通 error
func Method1(ctx context.Context, req *Req) (*Resp, error) {
    return nil, fmt.Errorf("error message")
}

// 返回 ApiError，精确控制状态码与错误类型
func Method2(ctx context.Context, req *Req) (*Resp, e.ApiError) {
    return nil, e.NewApiError(e.InvalidArgument, "Invalid argument", nil)
}
```

### 预定义错误类型

| 错误类型 | HTTP 状态码 | 说明 |
| --- | --- | --- |
| `e.NotFound` | 404 | 资源未找到 |
| `e.NotAcceptable` | 406 | 不可接受 |
| `e.Unauthorized` | 401 | 未授权 |
| `e.Forbidden` | 403 | 禁止访问 |
| `e.InvalidArgument` | 400 | 参数错误 |
| `e.TooManyRequests` | 429 | 请求过多 |
| `e.Internal` | 500 | 内部错误 |
| `e.Unavailable` | 503 | 服务不可用 |
| `e.Timeout` | 408 | 请求超时 |
| `e.Conflict` | 400 | 冲突 |
| `e.FailedPrecondition` | 412 | 前置条件失败 |
| `e.OutOfRange` | 400 | 超出范围 |
| `e.Unimplemented` | 501 | 未实现 |
| `e.Aborted` | 500 | 已中止 |
| `e.DataLoss` | 500 | 数据丢失 |
| `e.Unknown` | 500 | 未知错误 |

### gRPC 错误码转换

```go
// gRPC error -> e.ApiError
apiErr := e.ConvertErrorToErrorCode(grpcErr)

// e.ApiError -> gRPC status
status := apiErr.GRPCStatus()
```

## 中间件

中间件通过 `Use()` 注册，通过 `ctx.Next()` 驱动后续 handler，与 Gin/Hertz 风格一致。

### 注册中间件

```go
engine := web.Default(
    config.WithAddr(":8080"),
    config.WithName("myapp"),
)

engine.Use(func(ctx context.Context, reqCtx *app.RequestContext) {
    start := time.Now()
    reqCtx.Next(ctx)
    logger.Info("| %3d | %13v | %15s | %7s %s",
        reqCtx.GetStatusCode(),
        time.Since(start),
        reqCtx.ClientIP(),
        reqCtx.GetMethod(),
        reqCtx.GetPath(),
    )
})
```

### 执行模型

`Next()` 之前的代码在 handler 前执行，之后的代码在 handler 后执行：

```go
engine.Use(func(ctx context.Context, reqCtx *app.RequestContext) {
    fmt.Println("middleware before")
    reqCtx.Next(ctx)
    fmt.Println("middleware after")
})
```

多个中间件按注册顺序形成链：

```text
mw1-before → mw2-before → handler → mw2-after → mw1-after
```

### 中断链

不调用 `Next()` 即中断后续中间件和 handler：

```go
engine.Use(func(ctx context.Context, reqCtx *app.RequestContext) {
    token := reqCtx.HeaderGet("Authorization")
    if token == "" {
        reqCtx.AbortWithMsg("unauthorized", 401)
        return
    }
    reqCtx.Next(ctx)
})
```

调用 `Abort()` 后即使继续调用 `Next()`，后续 handler 也不会执行：

```go
engine.Use(func(ctx context.Context, reqCtx *app.RequestContext) {
    reqCtx.AbortWithMsg("forbidden", 403)
    reqCtx.Next(ctx)
})
```

### 路由组中间件

```go
api := engine.Group("/api")

api.Use(func(ctx context.Context, reqCtx *app.RequestContext) {
    reqCtx.SetHeader("X-API-Version", "v1")
    reqCtx.Next(ctx)
})

api.GET("/users", userHandler)
api.POST("/users", createUserHandler)

v1 := api.Group("/v1")
v1.Use(authMiddleware)
v1.GET("/profile", profileHandler)
```

### Panic 恢复

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

## 请求上下文

需要访问原始请求信息时，使用 HandlerFunc 风格：

```go
import "github.com/caiflower/common-tools/web/app"

func MyHandler(ctx context.Context, reqCtx *app.RequestContext) {
    path := reqCtx.GetPath()
    params := reqCtx.GetParams()
    method := reqCtx.GetMethod()

    // 获取原始 http 对象
    w, r := reqCtx.GetResponseWriterAndRequest()

    // 设置自定义属性
    reqCtx.Set("key", "value")
    value, _ := reqCtx.Get("key")

    reqCtx.JSON(200, map[string]interface{}{
        "path":   path,
        "params": params,
        "method": method,
        "value":  value,
    })
}
```

常用方法：

| 分类 | 方法 |
| --- | --- |
| 请求信息 | `GetPath`、`GetParams`、`GetMethod`、`GetRouteTemplate`、`ClientIP`、`Host`、`URI` |
| Header | `HeaderGet`、`SetHeader` |
| 上下文数据 | `Set`、`Get`、`SetData`、`GetData` |
| 响应 | `JSON`、`WriteString`、`SetStatusCode`、`GetStatusCode`、`SetContentType` |
| 中断与链 | `Abort`、`AbortWithMsg`、`Next`、`IsAbort` |
| 原始对象 | `GetResponseWriterAndRequest` |

## 高级特性

### 限流

```go
server := web.Default(
    config.WithQps(true, 1000), // 每秒 1000 请求
)
```

超出限流的请求返回 `429 TooManyRequests`。

### Prometheus 指标

```go
server := web.Default(
    config.WithMetrics(true),
)
```

访问 `GET /metrics` 获取指标。内置指标：

| 指标 | 类型 | 标签 |
| --- | --- | --- |
| `http_request_total` | Counter | `web`、`status`、`method`、`handler`、`ip`，可选 `namespace`、`app` |
| `http_request_duration_seconds` | Histogram | `handler`、`ip`，可选 `namespace`、`app` |

> [!NOTE]
> `namespace` 和 `app` 来自 `global/env`，只有在 Kubernetes 等设置了对应环境变量的环境中才会出现。

### Swagger / OpenAPI

```go
server := web.Default(
    config.WithEnableSwagger(true),
)
```

访问 `GET /swagger/json` 获取 OpenAPI 3.0 文档。

### Pprof

```go
server := web.Default(
    config.WithEnablePprof(true),
)
```

可用端点：

- `GET /debug/pprof/`：概览
- `GET /debug/pprof/profile`：CPU profile
- `GET /debug/pprof/heap`：堆内存
- `GET /debug/pprof/goroutine`：Goroutine
- `GET /debug/pprof/trace`：执行追踪

### 请求追踪

从指定 Header 提取 Trace ID：

```go
server := web.Default(
    config.WithHeaderTraceID("X-Request-Id"),
)
```

### 自定义回调

```go
// 分发前回调；返回 true 中断请求
server.SetBeforeDispatchCallBack(func(reqCtx *app.RequestContext) bool {
    return false
})

// 分发后回调；仅 Netpoll 模式支持
server.SetAfterDispatchCallBack(func(reqCtx *app.RequestContext) bool {
    return false
})
```

### TLS / HTTPS

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

### HTTP/2

```go
server := web.Default(
    config.WithH2C(true), // 启用 HTTP/2 Cleartext
    config.WithMaxConcurrentStreams(1000),
    config.WithMaxUploadBufferPerConnection(1<<20),
    config.WithMaxUploadBufferPerStream(1<<18),
)
```

### 连接管理

```go
server := web.Default(
    config.WithDisableKeepalive(false),
    config.WithSenseClientDisconnection(true),
    config.WithOnAccept(func(conn net.Conn) context.Context {
        return context.Background()
    }),
    config.WithOnConnect(func(ctx context.Context, conn network.Conn) context.Context {
        return ctx
    }),
)
```

### 请求大小限制

`MaxHeaderBytes` 和 `MaxRequestBodySize` 没有内置 Option 函数，可以通过自定义
`config.Option` 设置（仅 Netpoll 模式生效）：

```go
import "github.com/caiflower/common-tools/web/app/server/config"

func withRequestLimits(maxHeaderBytes, maxRequestBodySize int) config.Option {
    return func(opts *config.Options) *config.Options {
        opts.MaxHeaderBytes = maxHeaderBytes
        opts.MaxRequestBodySize = maxRequestBodySize
        return opts
    }
}

server := web.Default(
    withRequestLimits(1<<20, 10<<20), // 1MB Header，10MB Body
)
```

### 服务器模式

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

## 配置选项

### 内置 Option 函数

#### 基础配置

| Option 函数 | 参数 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `WithName` | `string` | `"default"` | 服务器名称 |
| `WithAddr` | `string` | `":8080"` | 监听地址 |
| `WithMode` | `ServerMode` | `"netpoll"` | `standard` 或 `netpoll` |
| `WithRootPath` | `string` | `""` | API 根路径前缀 |
| `WithHeaderTraceID` | `string` | `"X-Request-Id"` | Trace ID 请求头 |
| `WithControllerRootPkgName` | `string` | `"controller"` | Controller 扫描的根包名 |

#### 超时配置

| Option 函数 | 参数 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `WithReadTimeout` | `time.Duration` | 20s | 读取超时 |
| `WithWriteTimeout` | `time.Duration` | 35s | 写入超时 |
| `WithHandleTimeout` | `time.Duration` | 60s | 请求处理超时 |
| `WithExitWaitTime` | `time.Duration` | 5s | 优雅退出等待时间 |

#### 功能开关

| Option 函数 | 参数 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `WithEnablePprof` | `bool` | false | 启用 pprof |
| `WithEnableSwagger` | `bool` | false | 启用 Swagger |
| `WithEnableCLI` | `bool` | false | 启用 `/cli/routes` 元数据接口 |
| `WithCLIRoutesPath` | `string` | `"/cli/routes"` | CLI 元数据接口路径 |
| `WithEnableActionController` | `bool` | false | 启用 Action Controller |
| `WithMetrics` | `bool` | false | 启用 Prometheus 指标 |
| `WithDisableOptimization` | `bool` | false | 禁用性能优化 |
| `WithDisableKeepalive` | `bool` | false | 禁用 Keep-Alive |
| `WithSenseClientDisconnection` | `bool` | false | 检测客户端断开 |
| `WithDisableHeaderNamesNormalizing` | `bool` | false | 禁用 Header 名规范化 |

#### 限流

| Option 函数 | 参数 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `WithQps` | `bool, int` | false, 0 | 是否启用限流及 QPS 上限 |

#### HTTP/2

| Option 函数 | 参数 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `WithH2C` | `bool` | false | 启用 HTTP/2 Cleartext |
| `WithALPN` | `bool, *tls.Config` | false, nil | 启用 ALPN（TLS） |
| `WithMaxConcurrentStreams` | `uint32` | 0（协议默认） | 最大并发流数 |
| `WithMaxUploadBufferPerConnection` | `int32` | 0（协议默认） | 每连接上传缓冲区 |
| `WithMaxUploadBufferPerStream` | `int32` | 0（协议默认） | 每流上传缓冲区 |
| `WithMaxReadFrameSize` | `uint32` | 0（协议默认） | 最大读取帧大小 |
| `WithPermitProhibitedCipherSuites` | `bool` | false | 允许 HTTP/2 禁用的密码套件 |

#### 连接管理

| Option 函数 | 参数 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `WithOnAccept` | `func(net.Conn) context.Context` | nil | 连接接受回调 |
| `WithOnConnect` | `func(context.Context, network.Conn) context.Context` | nil | 连接建立回调 |
| `WithListenConfig` | `*net.ListenConfig` | nil | 监听配置 |

### 未提供内置 Option 的字段

以下字段在 `config.Options` 中存在，但没有对应的 `With*` 函数，需要自定义 `config.Option`：

| 字段 | 类型 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `IdleTimeout` | `time.Duration` | 60s | 空闲连接超时 |
| `KeepAliveTimeout` | `time.Duration` | 180s | Keep-Alive 超时 |
| `MaxHeaderBytes` | `int` | 1MB | 最大请求头大小 |
| `MaxRequestBodySize` | `int` | 10MB | 最大请求体大小 |
| `Network` | `string` | `"tcp"` | 监听网络类型 |
| `EnableTrace` | `bool` | false | 启用链路追踪 |

> [!NOTE]
> 这些字段目前主要由 Netpoll 模式读取。Standard 模式基于 `net/http`，只使用
> `Addr`、`ReadTimeout`、`WriteTimeout`、`HandleTimeout` 等基础字段。

```go
func withIdleTimeouts(idle, keepAlive time.Duration) config.Option {
    return func(opts *config.Options) *config.Options {
        opts.IdleTimeout = idle
        opts.KeepAliveTimeout = keepAlive
        return opts
    }
}
```

> [!IMPORTANT]
> 旧版本文档中出现的 `WithIdleTimeout`、`WithKeepAliveTimeout`、`WithMaxHeaderBytes`、
> `WithMaxRequestBodySize`、`WithNetwork`、`WithEnableMetrics` 在 `release-v1.0.0`
> 中并不存在。请使用上表的自定义 Option 方式，指标开关使用 `WithMetrics`。

## 相关文档

- [Web 框架概览](README.md)
- [HTTP 客户端](client.md)
- [命令行双模式](cli.md)
- 源码：[web/engine.go](../../../web/engine.go)
- 服务端配置：[web/app/server/config](../../../web/app/server/config)
- 错误类型：[web/common/e](../../../web/common/e)
