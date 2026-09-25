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
