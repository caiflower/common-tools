# Web 框架

`web` 是 common-tools 提供的高性能 RESTful Web 框架，覆盖 HTTP 服务端、路由、参数绑定与校验、中间件、HTTP 客户端、gRPC 集成和命令行双模式。

> 导入路径：`github.com/caiflower/common-tools/web`
>
> 当前文档对应 `release-v1.0.0`，包路径不带版本后缀。

## 特性

| 能力 | 说明 |
| --- | --- |
| 双服务器模式 | Netpoll（默认，事件驱动）与标准 `net/http` |
| 路由 | 路由组、嵌套分组、组级中间件、RESTful 路径参数 |
| 参数处理 | JSON / query / path / header 自动绑定，`verf` 标签校验 |
| 中间件 | 基于 `ctx.Next()` 的链式模型，与 Gin/Hertz 一致 |
| gRPC over HTTP | 直接注册 protobuf Handler，减少一次反射 |
| HTTP 客户端 | 连接池、超时、重试、代理、链路追踪 |
| CLI 双模式 | 同一二进制同时提供 HTTP 服务与命令行调用 |
| 可观测性 | Prometheus 指标、pprof、Swagger、Trace ID |
| 流量治理 | 令牌桶限流、请求大小限制、连接管理 |

## 性能基准

在 Intel Xeon Platinum 8338C（2c4g）上使用 wrk 压测：

```bash
wrk -t12 -c500 -d60s 'http://127.0.0.1:30461/v1/hobby/search?query=1&page_number=1&hobby=math'
```

```text
Running 1m test @ http://127.0.0.1:30461/v1/hobby/search?query=1&page_number=1&hobby=math
  12 threads and 500 connections
  Thread Stats   Avg      Stdev     Max   +/- Stdev
    Latency    13.99ms   16.47ms 211.61ms   81.17%
    Req/Sec     5.34k   675.51    43.92k    88.50%
  3829089 requests in 1.00m, 752.25MB read
Requests/sec:  63778.36
Transfer/sec:     12.53MB
```

> [!NOTE]
> 压测结果依赖机器规格、网络环境和业务逻辑，仅用于量级参考，不代表所有场景下的性能。

## 快速开始

### 安装

```bash
go get github.com/caiflower/common-tools
```

### 最小 HTTP 服务

```go
package main

import (
    "context"

    "github.com/caiflower/common-tools/global"
    "github.com/caiflower/common-tools/web"
    "github.com/caiflower/common-tools/web/app/server/config"
)

type HelloReq struct {
    Name string `query:"name" verf:"required"`
}

func Hello(ctx context.Context, req *HelloReq) (interface{}, error) {
    return map[string]string{"message": "hello, " + req.Name}, nil
}

func main() {
    engine := web.Default(
        config.WithName("myapp"),
        config.WithAddr(":8080"),
        config.WithMode(config.ServerModeStandard),
    )
    engine.GET("/hello", Hello)

    // Start() 本身是非阻塞的；交给资源管理器托管启动与优雅退出
    global.DefaultResourceManger.AddDaemonWithOrder(engine, global.OrderHTTPServer)
    global.DefaultResourceManger.Signal()
}
```

启动后验证：

```bash
curl 'http://127.0.0.1:8080/hello?name=world'
# {"requestId":"...","data":{"message":"hello, world"}}
```

> [!TIP]
> `engine.Start()` 只负责启动监听，不会阻塞主 goroutine。生产环境推荐使用
> `global.DefaultResourceManger` 统一管理启动顺序和优雅退出（global 文档待补充）。

## 服务器模式

| 模式 | 常量 | 说明 | 适用场景 |
| --- | --- | --- | --- |
| Netpoll | `config.ServerModeNetpoll` | 默认。基于 CloudWeGo netpoll，事件驱动 | 高并发、低延迟服务 |
| Standard | `config.ServerModeStandard` | 基于标准库 `net/http` | 需要标准库生态或既有中间件 |

## 文档导航

| 文档 | 内容 |
| --- | --- |
| [server.md](server.md) | 服务端完整参考：Handler、路由、gRPC、参数绑定、校验、中间件、高级特性、配置 |
| [client.md](client.md) | HTTP 客户端用法与配置 |
| [cli.md](cli.md) | 命令行双模式：`serve`、`routes`、动态命令与远端元数据 |

## 相关链接

- 源码：[web/engine.go](../../../web/engine.go)
- 服务端实现：[web/app/server](../../../web/app/server)
- 文档索引：[docs/README.md](../../README.md)
- 根 README：[README.md](../../../README.md)
