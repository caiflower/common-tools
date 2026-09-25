## Why

`web/` 框架目前只有 HTTP 调用方式，接口调试和线上排查需要手拼 URL、请求头和 JSON body。希望同一个二进制既能启动 HTTP 服务，也能用 kubectl 风格的命令直接调用已注册接口，减少调试成本并统一联调体验。

## What Changes

- 新增路由元数据注册表：路由注册时自动记录 `method/path/operationID/resource/verb` 以及 path/query/header/body 参数信息。
- 新增可选 `/cli/routes` 发现接口：由 `WithEnableCLI` 控制，默认关闭；开启后返回路由元数据，供远程 CLI 发现接口。
- 新增 `web/cli` 包：基于 cobra 构建命令行，支持同一个二进制双模式（`serve` 模式启动服务，CLI 模式调用服务）。
- `serve` 模式接入 `global.DefaultResourceManger`：把 engine 注册为 daemon 资源，由统一的资源管理器负责启停和优雅退出。
- CLI 默认使用本进程已注册的路由元数据构建命令；指定 `--server` 时通过远端 `/cli/routes` 动态发现，并支持本地缓存、TTL/etag 刷新和 `routes --refresh` 强制刷新。
- 命令模型采用 kubectl 风格 `动词 + 资源`；resource/verb 自动推断（方法名优先、path 兜底），支持显式覆盖，歧义时生成 `call <operationID>` 兜底命令。
- 参数传递：path/query/header 使用独立 flag，body 使用 `--data`/`-f`；输出支持 `table/json/yaml`。

## Capabilities

### New Capabilities
- `route-metadata`: 服务端路由元数据注册表与 `/cli/routes` 发现接口。
- `web-cli`: cobra 命令行、动态命令生成、HTTP 调用与元数据缓存。

### Modified Capabilities
- 无：现有路由注册、分发行为不变，新增能力独立承载。

## Impact

- `web/router`：`Handler`/`RouterGroup` 在注册时写入元数据，新增路由枚举/元数据访问能力。
- `web/app/server/config`：新增 `WithEnableCLI` 等配置项。
- `web/` 新增 `cli` 包：cobra 根命令、动态命令生成、HTTP 客户端封装、缓存。
- `global`：`serve` 默认使用 `global.DefaultResourceManger` 启停 engine。
- `go.mod`：引入 cobra 依赖。
- 文档：更新 `docs/web.md` 和 OpenSpec spec。
- 测试：新增路由元数据、`/cli/routes`、命令生成、缓存刷新、HTTP 调用测试。
