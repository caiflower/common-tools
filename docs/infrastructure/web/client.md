# Web HTTP 客户端

`web/app/client` 提供高性能 HTTP 客户端，支持连接池、超时控制、重试、代理和链路追踪。

> 导入路径：
>
> - `github.com/caiflower/common-tools/web/app/client`
> - `github.com/caiflower/common-tools/web/protocol`

## 快速开始

```go
package main

import (
    "context"
    "fmt"
    "time"

    "github.com/caiflower/common-tools/web/app/client"
)

func main() {
    c, err := client.NewClient(
        client.WithDialTimeout(5*time.Second),
        client.WithMaxConnsPerHost(100),
        client.WithClientReadTimeout(10*time.Second),
    )
    if err != nil {
        panic(err)
    }

    // GET：dst 为 nil 时返回新分配的响应体
    statusCode, body, err := c.Get(context.Background(), nil, "http://example.com/api/data")
    if err != nil {
        panic(err)
    }
    fmt.Println(statusCode, string(body))
}
```

## 发送请求

### GET

```go
statusCode, body, err := c.Get(ctx, nil, "http://example.com/api/data")
```

`dst` 可以传入复用的切片，避免频繁分配：

```go
buf := make([]byte, 0, 4096)
buf, err = c.Get(ctx, buf[:0], url)
```

### POST 与自定义请求

需要设置 Header、Body 或使用非 GET 方法时，通过 `protocol.NewRequest` 构造请求：

```go
import (
    "strings"

    "github.com/caiflower/common-tools/web/protocol"
)

req := protocol.NewRequest("POST", "http://example.com/api/create", strings.NewReader(`{"name":"test"}`))
req.Header.SetContentType("application/json")

resp := protocol.AcquireResponse()
defer protocol.ReleaseResponse(resp)

if err := c.Do(ctx, req, resp); err != nil {
    panic(err)
}

fmt.Println(resp.StatusCode(), string(resp.Body()))
```

> [!TIP]
> `protocol.AcquireResponse()` 从对象池获取响应，使用完必须调用
> `protocol.ReleaseResponse(resp)` 归还，避免内存分配和对象泄漏。

## 超时控制

| 方式 | 说明 |
| --- | --- |
| `client.WithDialTimeout` | 建立连接的超时时间 |
| `client.WithClientReadTimeout` | 读取响应的超时时间 |
| `client.WithWriteTimeout` | 写出请求的超时时间 |
| `c.GetTimeout(ctx, dst, url, timeout)` | 单次 GET 的超时 |
| `c.GetDeadline(ctx, dst, url, deadline)` | 单次 GET 的截止时间 |
| `c.DoTimeout(ctx, req, resp, timeout)` | 单次请求的超时 |
| `c.DoDeadline(ctx, req, resp, deadline)` | 单次请求的截止时间 |

## 代理

```go
c.SetProxy(func(req *protocol.Request) (*protocol.URI, error) {
    return protocol.ParseURI("http://proxy.example.com:8080"), nil
})
```

## 重试

```go
import (
    "github.com/caiflower/common-tools/web/app/client/retry"
)

c, _ := client.NewClient(
    client.WithRetryConfig(
        retry.WithMaxAttemptTimes(3),
        retry.WithInitDelay(100*time.Millisecond),
        retry.WithMaxDelay(2*time.Second),
    ),
)

c.SetRetryIfFunc(func(req *protocol.Request, resp *protocol.Response, err error) bool {
    return err != nil || resp.StatusCode() >= 500
})
```

`SetRetryIfFunc` 决定一次失败是否重试；`WithRetryConfig` 决定重试次数和退避策略。
两者需要配合使用。

## 配置选项

`client.NewClient` 接受 `config.ClientOption`：

| Option 函数 | 参数 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `WithDialTimeout` | `time.Duration` | 1s | 建立连接超时 |
| `WithMaxConnsPerHost` | `int` | 0（不限） | 每个 Host 的最大连接数 |
| `WithMaxIdleConnDuration` | `time.Duration` | 10s | 空闲连接保留时长 |
| `WithMaxConnDuration` | `time.Duration` | 0（不限） | 连接最大存活时长 |
| `WithMaxConnWaitTimeout` | `time.Duration` | 0（不等待） | 等待可用连接的超时 |
| `WithKeepAlive` | `bool` | true | 是否启用 Keep-Alive |
| `WithClientReadTimeout` | `time.Duration` | 0（不限） | 读取响应超时 |
| `WithWriteTimeout` | `time.Duration` | 0（不限） | 写出请求超时 |
| `WithTLSConfig` | `*tls.Config` | nil | TLS 配置 |
| `WithDialer` | `network.Dialer` | 默认 Dialer | 自定义拨号器 |
| `WithResponseBodyStream` | `bool` | false | 流式读取响应体 |
| `WithDisableHeaderNamesNormalizing` | `bool` | false | 禁用 Header 名规范化 |
| `WithDisablePathNormalizing` | `bool` | false | 禁用路径规范化 |
| `WithName` | `string` | "" | 客户端名称（用于 User-Agent） |
| `WithNoDefaultUserAgentHeader` | `bool` | false | 禁用默认 User-Agent |
| `WithRetryConfig` | `...retry.Option` | nil | 重试配置 |
| `WithConnStateObserve` | `HostClientStateFunc, interval` | nil, 5s | 连接池状态观察 |
| `WithDialFunc` | `DialFunc, ...Dialer` | nil | 自定义拨号函数 |
| `WithHostClientConfigHook` | `func(interface{}) error` | nil | 重新配置 HostClient 的回调 |

> [!NOTE]
> 默认值以 `web/common/config/client_option.go` 和
> `web/protocol/consts/default.go` 为准；版本升级后如有差异，以源码为准。

## 相关文档

- [Web 框架概览](README.md)
- [服务端文档](server.md)
- 源码：[web/app/client](../../../web/app/client)
- 协议层：[web/protocol](../../../web/protocol)
