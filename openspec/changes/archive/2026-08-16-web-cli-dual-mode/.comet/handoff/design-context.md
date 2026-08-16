# Comet Design Handoff

- Change: web-cli-dual-mode
- Phase: design
- Mode: compact
- Context hash: cf09055a84ab79cf8d18d8918d8ec68396a68e783cd626ac51823ae9d4b68db2

Generated-by: comet-handoff.sh

OpenSpec remains the canonical capability spec. This handoff is a deterministic, source-traceable context pack, not an agent-authored summary.

## openspec/changes/web-cli-dual-mode/proposal.md

- Source: openspec/changes/web-cli-dual-mode/proposal.md
- Lines: 1-32
- SHA256: ff1db9ea7875ee25361661f8055b3fcac04c1c9538cda3ef096d28844ac4caca

```md
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
```

## openspec/changes/web-cli-dual-mode/design.md

- Source: openspec/changes/web-cli-dual-mode/design.md
- Lines: 1-65
- SHA256: ff9971874017135c9991770aa2d0013120bb2deb6705792b99c52e876e0687d9

```md
## Context

`web/` 框架的路由注册集中在 `RouterGroup`，最终由 `Handler.addRoute` 写入方法树。当前注册过程没有保留可供 CLI 使用的结构化元数据：方法名、参数来源（path/query/header/body）、参数类型和校验规则都存在于 `basic.Method`/`basic.ArgInfo`，但没有统一出口；`/swagger/json` 也只覆盖旧式 `Register` 注册的路由。CLI 需要这些元数据来生成 cobra 命令并构造 HTTP 请求。

目标形态是同一个二进制双模式：`myserver serve` 启动服务；`myserver get users --id=1` 使用本地注册的路由元数据构建命令并通过 HTTP 调用服务；`--server` 指定远端实例时通过 `/cli/routes` 动态发现。

## Goals / Non-Goals

**Goals:**
- 路由注册时自动生成可枚举、可序列化的元数据，不要求用户额外写发现接口。
- 提供 kubectl 风格的动态命令生成：`动词 + 资源`，歧义时可用 `call <operationID>`。
- 本地模式零网络发现；远程模式支持 `/cli/routes` + 本地缓存 + TTL/etag 刷新。
- 保持现有路由注册、分发行为不变。

**Non-Goals:**
- 不做进程内直调 handler，CLI 始终通过 HTTP 调用 server。
- 不做 codegen、交互式 REPL、完整鉴权体系。
- 不做复杂 CRUD 语义推断（如子资源、复数化规范化），只做确定性规则 + 显式覆盖。

## Decisions

### 1. 元数据中立化：`router.RouteInfo`
在 `web/router` 包内定义导出类型 `RouteInfo`/`ParamInfo`，字段只包含 `method/path/operationID/resource/verb/params`。`Handler` 持有线程安全的路由元数据切片，`addRoute` 时从最后一个 handler 提取信息并追加。`web/cli` 只依赖这些导出类型，不触碰路由树内部结构。

备选：复用/补齐 goai OpenAPI。选择独立注册表，是因为 RouterGroup 路由当前未进 goai，且 CLI 需要比 OpenAPI 更直接的参数来源和命令命名，独立 DTO 改动面更小。

### 2. resource/verb 推导规则
优先级：显式覆盖 > 方法名推断 > path 兜底。
- verb：HTTP method 映射 `GET->get`、`POST->create`、`PUT->update`、`PATCH->patch`、`DELETE->delete`，其余为 `call`。
- resource：优先从 handler 方法名（如 `GetUser` -> `user`）取；匿名/HandlerFunc 路由从 path 最后一个静态段取。
- 同名命令冲突时保留资源命令，同时自动生成 `call <operationID>` 兜底。

显式覆盖使用独立注册 API（如 `engine.CLIRoute("GET", "/users/:id", "users", "get")`），避免改变现有 `IRoutes` 链式返回类型。

### 3. 双模式入口：`web/cli`
新增 `web/cli` 包，提供 `cli.New(engine, opts...)` 返回 cobra root command：
- `serve` 子命令把 engine 注册到资源管理器并调用 `Signal()`，不直接 `engine.Start()`。`web/cli` 定义最小 `ResourceManager` 接口（`AddDaemonWithOrder` + `Signal`），默认实现使用 `global.DefaultResourceManger`；用户可通过 option 传入自定义管理器。Engine 已具备 `Name/Start/Close`，天然满足 daemon 资源语义，关闭顺序通过 `AddDaemonWithOrder` 控制。
- 其余子命令由本地 `engine.Routes()` 动态生成。
- `--server` 全局 flag 指定远端地址；未指定时默认使用 engine 配置的监听地址转换出的 `http://127.0.0.1:<port>`。
- `--token`、`--header` 作为透传参数，附加到每个请求。

### 4. 远程发现与缓存
`/cli/routes` 返回 `{name, version, resources, routes}`，响应带 ETag；CLI 发送 `If-None-Match`，304 时复用缓存并更新时间戳。缓存写入用户缓存目录，以 server 地址 hash 命名；TTL 默认 5 分钟，支持 `--refresh` 强制刷新。网络失败且有缓存时降级使用旧缓存并输出警告。

### 5. 参数绑定与输出
path/query/header 字段生成独立 flag；body 参数用 `--data '{"json":...}'` 或 `-f file.json`。GET/HEAD 下 `json` tag 字段视为 query，其他方法视为 body，与现有 `setArgsOptimized` 语义一致。输出默认 `table`，支持 `--output json|yaml`。

## Risks / Trade-offs

- 匿名 handler 的方法名无法推断资源 → 使用 path 兜底，并始终提供 `call <operationID>` 命令。
- 同一个进程双模式要求用户在 CLI 模式前先启动 `serve`，否则 HTTP 连接失败 → 错误信息明确提示“请先运行 serve”。
- 自动推导可能不符合用户语义 → 提供显式覆盖 API，规则文档化。
- 远程发现默认关闭可避免泄露接口元数据 → `WithEnableCLI` 默认 false，允许配置路径/鉴权。
- 动态命令帮助依赖本地元数据或缓存；远程模式无缓存且服务不可达时无法生成帮助 → 给出明确错误和 `--refresh`/缓存目录说明。

## Migration Plan

- 纯新增能力，不影响现有 `web.Default`/路由注册调用方式。
- `web/cli` 作为可选集成，不改变已有 server 启动路径。
- 按依赖顺序提交：路由元数据 → `/cli/routes` → `web/cli` 本地模式 → 远程发现/缓存 → 文档。

## Open Questions

- 显式覆盖 API 的具体签名（独立 `CLIRoute` vs 注册参数）在 build 阶段实现时再固化，spec 只约束能力。
- `table` 输出的字段抽取规则（对象取 `data` 展开）在实现时以最小可用版本为准。
```

## openspec/changes/web-cli-dual-mode/tasks.md

- Source: openspec/changes/web-cli-dual-mode/tasks.md
- Lines: 1-38
- SHA256: 5052f140dd092f5d42e25791723a499c63328d2f01b697a61036d584babd42db

```md
## 1. 路由元数据基础

- [ ] 1.1 在 `web/router` 定义导出的 `RouteInfo`/`ParamInfo` 类型，包含 method/path/operationID/resource/verb/params 与参数来源、类型、校验信息
- [ ] 1.2 为 `Handler` 增加线程安全的元数据注册表，并在 `addRoute` 时从最后一个 handler 提取 operationID、参数结构并写入
- [ ] 1.3 实现 resource/verb 推导：方法名优先、path 兜底、HTTP method 映射，冲突时生成 `call <operationID>` 兜底信息
- [ ] 1.4 为 `Handler` 增加并发安全的 `Routes()` 只读枚举 API
- [ ] 1.5 增加显式覆盖 API，允许为指定路由设置 resource/verb
- [ ] 1.6 为元数据注册与推导补齐单元测试（普通函数、结构体方法、gRPC、匿名 HandlerFunc、显式覆盖）

## 2. CLI 发现接口

- [ ] 2.1 在 `web/app/server/config` 增加 CLI 元数据开关与路径配置项（默认关闭）
- [ ] 2.2 在 `specialRequest` 中实现 `GET /cli/routes`，返回 name/version/resources/routes JSON
- [ ] 2.3 实现 ETag/`If-None-Match` 支持，匹配时返回 304
- [ ] 2.4 补充 `/cli/routes` 开启/关闭、内容正确性、304 条件请求测试

## 3. CLI 包本地模式

- [ ] 3.1 引入 cobra 依赖，创建 `web/cli` 包和 root command，定义最小 `ResourceManager` 接口
- [ ] 3.2 实现 `serve` 子命令：将 engine 注册为 daemon 并调用 `Signal()`，默认使用 `global.DefaultResourceManger`，支持自定义管理器 option
- [ ] 3.3 从 `engine.Routes()` 生成本地动态命令树（`动词 + 资源` + `call <operationID>` 兜底）
- [ ] 3.4 为 path/query/header 参数生成 flag，`--data`/`-f` 处理 body，GET/HEAD 的 json 字段映射为 query
- [ ] 3.5 使用 `web/app/client` 发起 HTTP 请求，支持 `--server` 默认地址解析、`--token`、`--header`
- [ ] 3.6 实现 `table/json/yaml` 输出
- [ ] 3.7 补充本地模式命令生成、请求构造、输出格式、资源管理器集成测试

## 4. 远程发现与缓存

- [ ] 4.1 实现远端 `/cli/routes` 拉取与元数据反序列化
- [ ] 4.2 实现缓存文件读写、TTL（默认 5 分钟）、server 地址 hash 命名
- [ ] 4.3 实现 `If-None-Match` 条件刷新，304 时保留缓存并更新时间戳
- [ ] 4.4 实现 `routes` 列表命令与 `--refresh` 强制刷新，网络失败有缓存时降级并警告
- [ ] 4.5 补充远程发现、缓存命中、强制刷新、304、降级场景测试

## 5. 文档与收尾

- [ ] 5.1 更新 `docs/web.md`：双模式用法、命令示例、`/cli/routes` 配置与安全说明
- [ ] 5.2 运行 `go test ./web/...` 和相关静态检查，确认无回归
```

## openspec/changes/web-cli-dual-mode/specs/route-metadata/spec.md

- Source: openspec/changes/web-cli-dual-mode/specs/route-metadata/spec.md
- Lines: 1-56
- SHA256: fce3a3ee03fc9fcd64fbb7ff85df395f762c1440593f9d8566b86d5dc39cd53f

```md
## ADDED Requirements

### Requirement: Route metadata registry
The system SHALL record structured metadata for every route registered through `RouterGroup` (including `GET/POST/PUT/PATCH/DELETE/OPTIONS/HEAD/Any/Handle/GRPC/Static*`). Each record SHALL contain HTTP method, absolute path, operationID, resource, verb, and parameter list. Parameter list SHALL distinguish `path`, `query`, `header`, and `body` sources, and SHALL include field name, type, and validation tag when available.

#### Scenario: Record normal route
- **WHEN** `engine.GET("/users/:id", uc.GetUser)` is registered
- **THEN** the metadata registry SHALL contain a record with method `GET`, path `/users/:id`, and parameters derived from `GetUser`'s request struct

#### Scenario: Record GRPC route
- **WHEN** `engine.GRPC("POST", "/v1/search", handler, srv)` is registered
- **THEN** the metadata registry SHALL contain a record with method `POST`, path `/v1/search`, and operationID from the gRPC method name

### Requirement: Route enumeration API
The `router.Handler` SHALL expose a read-only `Routes()` API returning all registered route metadata. It SHALL be safe for concurrent access after registration.

#### Scenario: Enumerate registered routes
- **WHEN** a handler has multiple routes registered
- **THEN** `Routes()` SHALL return metadata for all of them without modifying the route tree

### Requirement: Resource and verb derivation
The system SHALL derive `resource` and `verb` with priority: explicit override, then handler method name, then path fallback. HTTP method mapping SHALL be `GET->get`, `POST->create`, `PUT->update`, `PATCH->patch`, `DELETE->delete`, and any other method to `call`.

#### Scenario: Derive from method name
- **WHEN** a handler method is named `GetUser`
- **THEN** its metadata SHALL use verb `get` and resource `user`

#### Scenario: Fallback to path
- **WHEN** a route uses an anonymous handler and no explicit override
- **THEN** its metadata SHALL use the last static path segment as resource and the HTTP-method-derived verb

#### Scenario: Explicit override wins
- **WHEN** an explicit override assigns resource `users` and verb `get` to a route
- **THEN** the metadata SHALL use `users`/`get` regardless of method name or path

### Requirement: CLI discovery endpoint
The system SHALL expose a `GET /cli/routes` endpoint when CLI metadata is enabled via configuration. The endpoint SHALL return JSON containing server name, metadata version, resource list, and route list. The endpoint SHALL support ETag and return `304 Not Modified` when `If-None-Match` matches the current version.

#### Scenario: Endpoint disabled by default
- **WHEN** CLI metadata is not enabled
- **THEN** `GET /cli/routes` SHALL NOT be served by the framework

#### Scenario: Endpoint enabled
- **WHEN** CLI metadata is enabled and a request hits `GET /cli/routes`
- **THEN** the response SHALL be JSON with non-empty route list reflecting registered routes

#### Scenario: Conditional request
- **WHEN** a request includes `If-None-Match` equal to the current ETag
- **THEN** the server SHALL respond with status `304` and no body

### Requirement: Explicit metadata override
The system SHALL provide an explicit registration API to assign `resource` and `verb` to a registered route without changing existing route registration methods.

#### Scenario: Override a route
- **WHEN** a route is registered and then assigned resource `orders` with verb `list`
- **THEN** generated metadata and `/cli/routes` SHALL use `orders`/`list`
```

## openspec/changes/web-cli-dual-mode/specs/web-cli/spec.md

- Source: openspec/changes/web-cli-dual-mode/specs/web-cli/spec.md
- Lines: 1-94
- SHA256: 87d460032e16d91607085aba02fc37fc30475aa63e3e681aa29012f4fefe6d80

[TRUNCATED]

```md
## ADDED Requirements

### Requirement: Dual-mode command entry
The CLI package SHALL provide a cobra root command that supports a `serve` subcommand and dynamically generated API commands. `serve` SHALL start the same engine's HTTP server; API commands SHALL send HTTP requests to a running server.

#### Scenario: Start server
- **WHEN** user runs `<binary> serve`
- **THEN** the engine SHALL be registered as a daemon resource on the resource manager and `Signal()` SHALL start it with its configured address

#### Scenario: Serve through resource manager
- **WHEN** a resource manager is configured on the CLI
- **THEN** `serve` SHALL use that manager for start/stop instead of calling `engine.Start()` directly

#### Scenario: Invoke local API
- **WHEN** user runs `<binary> get users --id=1` without `--server`
- **THEN** the CLI SHALL build the command from local route metadata and send HTTP request to the engine's configured local address

#### Scenario: Local mode without running server
- **WHEN** user runs a local CLI command without `--server` and the local server is not reachable
- **THEN** the CLI SHALL print a clear error explaining that the server must be started with `serve` first

### Requirement: Dynamic command generation from local metadata
The CLI SHALL generate commands from `engine.Routes()` at startup. Each route SHALL become a `verb resource` command with flags for path/query/header parameters and body input flags.

#### Scenario: Generate resource command
- **WHEN** local metadata contains route `GET /users/:id` with verb `get` and resource `users`
- **THEN** the CLI SHALL expose command `get users` with an `id` flag

#### Scenario: Fallback call command
- **WHEN** a route has conflicting or ambiguous resource/verb derivation
- **THEN** the CLI SHALL still expose a `call <operationID>` command for that route

### Requirement: Remote discovery
When `--server` is specified, the CLI SHALL fetch route metadata from `GET /cli/routes` on that server and build commands from the fetched metadata instead of local metadata.

#### Scenario: Discover remote routes
- **WHEN** user runs `<binary> --server http://example.com get users`
- **THEN** the CLI SHALL fetch `/cli/routes` from that server and generate the `get users` command from the response

### Requirement: Metadata cache and refresh
The CLI SHALL cache remote metadata locally. Cache SHALL be keyed by server address, have a TTL (default 5 minutes), support `--refresh` force refresh, and send `If-None-Match` using the cached version. When remote fetch fails and a cache exists, the CLI SHALL use the stale cache and print a warning.

#### Scenario: Cache hit
- **WHEN** remote metadata was fetched within TTL
- **THEN** the CLI SHALL reuse the cache without requesting `/cli/routes`

#### Scenario: Forced refresh
- **WHEN** user runs `routes --refresh`
- **THEN** the CLI SHALL ignore TTL and re-fetch remote metadata

#### Scenario: Conditional refresh
- **WHEN** cache is stale and the server returns `304`
- **THEN** the CLI SHALL keep the cached metadata and refresh the cache timestamp

#### Scenario: Stale fallback
- **WHEN** remote fetch fails and a cache file exists
- **THEN** the CLI SHALL execute using the cached metadata and output a warning

### Requirement: Request parameter binding
The CLI SHALL bind path parameters into the URL template, query parameters into the query string, header parameters into request headers, and body parameters from `--data` or `-f`. For GET/HEAD routes, `json` tag fields SHALL be treated as query parameters; for other methods they SHALL be treated as body.

#### Scenario: Path and query parameters
- **WHEN** command `get users --id=1 --status=active` maps to `GET /users/:id`
- **THEN** the request URL SHALL be `/users/1?status=active`

#### Scenario: Header parameter
- **WHEN** a parameter has header source
- **THEN** its flag value SHALL be set as a request header

#### Scenario: Body from file
- **WHEN** user passes `-f request.json`
- **THEN** the file content SHALL be sent as JSON request body

### Requirement: Output formats
The CLI SHALL support `table`, `json`, and `yaml` output. Default SHALL be `table`, selected by `--output`.

#### Scenario: JSON output
- **WHEN** user runs command with `--output json`
- **THEN** the CLI SHALL print the raw unified JSON response

```

Full source: openspec/changes/web-cli-dual-mode/specs/web-cli/spec.md

