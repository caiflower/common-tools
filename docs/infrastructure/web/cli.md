# 命令行双模式

同一个二进制可以同时作为 HTTP 服务和命令行工具使用：

- `serve` 子命令把 engine 注册到资源管理器并启动 HTTP 服务；
- `routes` 列出已注册接口；
- 其余子命令按「动词 + 资源」动态生成，通过 HTTP 调用运行中的服务。

CLI 只通过 HTTP 调用服务，不直接执行进程内的 handler。

> 导入路径：`github.com/caiflower/common-tools/web/cli`

## 启用方式

```go
package main

import (
    "github.com/caiflower/common-tools/web"
    "github.com/caiflower/common-tools/web/app/server/config"
    "github.com/caiflower/common-tools/web/cli"
)

func main() {
    engine := web.Default(
        config.WithName("myapp"),
        config.WithAddr(":8080"),
        config.WithEnableCLI(true),
    )

    engine.GET("/users/:userId", uc.GetUser)
    engine.POST("/users", uc.CreateUser)

    // 可选：手动覆盖 CLI 的资源名和动词
    engine.CLIRoute("GET", "/users/:userId", "user", "get")

    if err := cli.Run(engine); err != nil {
        panic(err)
    }
}
```

`WithEnableCLI(true)` 同时会暴露 `/cli/routes` 元数据接口，供远端 CLI 发现路由。

## 内置命令

```bash
# 启动服务（默认使用 global.DefaultResourceManger 托管）
myapp serve

# 列出已注册接口
myapp routes
```

## 动态命令

未显式声明的路由会按以下规则生成命令：

| HTTP 方法 | CLI 动词 |
| --- | --- |
| `GET` | `get` |
| `POST` | `create` |
| `PUT` | `update` |
| `PATCH` | `patch` |
| `DELETE` | `delete` |

资源名优先取 handler 方法名，取不到时使用路径最后一段。每条路由同时保留
`call <operationID>` 作为兜底命令。

```bash
# 调用本地服务
myapp get users --userId=42
myapp create users --data '{"name":"alice"}'

# 调用远端服务，并指定认证与请求头
myapp get users --userId=42 --server http://10.0.0.8:8080 --token abc --header X-Env=prod

# 输出 JSON 或 YAML
myapp get users --userId=42 --output json
myapp get users --userId=42 --output yaml
```

### 参数生成规则

| 来源 | 规则 |
| --- | --- |
| `path` / `query` / `header` tag | 生成对应的独立 flag |
| 非 GET/HEAD 的 `json` 字段 | 通过 `--data` 传 JSON，或通过 `-f file.json` 从文件读取 |
| GET/HEAD 的 `json` 字段 | 不生成 flag；服务端绑定同样只认 `query` tag |
| `verf:"required"` | 标记为必填 flag |

### 全局 flag

```text
--server <url>       远端服务地址；缺省使用 engine 配置的监听地址
--token <value>      附加 Authorization: Bearer <value>
--header k=v         可重复，附加请求头
--output table|json|yaml
--refresh            强制刷新远端元数据缓存
--cache-ttl 5m       元数据缓存有效期
--cache-dir <path>   元数据缓存目录
```

## 远端路由发现

`/cli/routes` 默认关闭。开启后，`myapp routes --server http://host:port` 会拉取远端接口元数据并写入本地缓存：

- 默认 TTL 为 5 分钟；
- 缓存文件按 server 地址哈希存放在用户缓存目录；
- 刷新时携带 `If-None-Match`，命中 `304` 时复用缓存；
- 服务不可达但存在缓存时，降级使用旧缓存并输出警告；
- 远端拉取失败且无缓存时，CLI 输出警告并降级为本进程本地元数据。

```go
config.WithEnableCLI(true)
config.WithCLIRoutesPath("/cli/routes")
```

> [!WARNING]
> `/cli/routes` 会暴露接口元数据。生产环境仅在需要远端发现时开启，并配合认证和网络隔离使用。

## 相关文档

- [Web 框架概览](README.md)
- [服务端文档](server.md)
- 源码：[web/cli](../../../web/cli)
