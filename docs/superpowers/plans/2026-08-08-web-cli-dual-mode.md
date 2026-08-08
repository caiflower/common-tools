# Web CLI Dual Mode Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为 `web/` 框架提供同一个二进制的双模式 CLI：`serve` 通过全局资源管理器启动服务，CLI 命令从本地路由元数据或远端 `/cli/routes` 生成并通过 HTTP 调用。

**Architecture:** 路由注册时在 `router.Handler` 中记录 `RouteInfo`；`web/cli` 用 cobra 构建 root command，本地模式读取 `engine.Routes()`，远端模式读取 `/cli/routes` 并缓存；请求统一用 `web/app/client` 发送。

**Tech Stack:** Go 1.24、cobra、`web/app/client`、`global.DefaultResourceManger`。

---

change: web-cli-dual-mode
design-doc: docs/superpowers/specs/2026-08-08-web-cli-dual-mode-design.md
base-ref: 3f40e230fd05b8502f6669748f755e29d1448d2a

## Global Constraints

- 不改变现有 `web.Default` 和路由注册 API 行为。
- 新增依赖只允许 cobra。
- 所有路由元数据 DTO 放在 `web/router`，CLI 不依赖路由树内部字段。
- `/cli/routes` 默认关闭。
- 提交粒度：每个任务一个 commit，commit message 使用 `feat:`/`test:` 前缀。
- 每个任务先写测试，再实现，最后跑 `go test ./web/...`。

---

### Task 1: 引入 cobra

**Files:**
- Modify: `go.mod`、`go.sum`

**Interfaces:**
- Consumes: 无
- Produces: cobra 可用

- [ ] **Step 1: 添加依赖**

```bash
go get github.com/spf13/cobra@latest
```

- [ ] **Step 2: 验证编译**

```bash
go build ./web/...
```

Expected: 成功，无新错误。

- [ ] **Step 3: 提交**

```bash
git add go.mod go.sum
git commit -m "chore: add cobra dependency for web cli"
```

---

### Task 2: 路由元数据注册表

**Files:**
- Create: `web/router/route_info.go`
- Create: `web/router/route_info_test.go`
- Modify: `web/router/handler.go`
- Modify: `web/engine.go`

**Interfaces:**
- Consumes: `basic.Method`、`basic.ArgInfo`、`method.Method`
- Produces:
  - `type ParamInfo struct { Name, Source, Type, Verf string; Required bool }`
  - `type RouteInfo struct { Method, Path, OperationID, Resource, Verb string; Params []ParamInfo; Static bool }`
  - `func (h *Handler) Routes() []RouteInfo`
  - `func (h *Handler) CLIRoute(method, path, resource, verb string)`
  - `func (e *Engine) CLIRoute(method, path, resource, verb string)`

- [ ] **Step 1: 定义 DTO 与推导逻辑**

`web/router/route_info.go` 内容：

```go
package router

import (
    "fmt"
    "reflect"
    "strings"

    "github.com/caiflower/common-tools/pkg/basic"
    "github.com/caiflower/common-tools/web/router/method"
)

type ParamInfo struct {
    Name     string `json:"name"`
    Source   string `json:"source"`
    Type     string `json:"type"`
    Required bool   `json:"required,omitempty"`
    Verf     string `json:"verf,omitempty"`
}

type RouteInfo struct {
    Method      string      `json:"method"`
    Path        string      `json:"path"`
    OperationID string      `json:"operationID"`
    Resource    string      `json:"resource"`
    Verb        string      `json:"verb"`
    Params      []ParamInfo `json:"params,omitempty"`
    Static      bool        `json:"static,omitempty"`
}

func deriveVerb(method string) string
func deriveResourceFromName(name string) string
func deriveResourceFromPath(path string) string
func extractParams(m *method.Method, httpMethod string) []ParamInfo
func extractParamsFromArg(arg *basic.ArgInfo, httpMethod string, prefix string) []ParamInfo
func paramType(t reflect.Type) string
func requiredFromVerf(verf string) bool
```

`extractParamsFromArg` 规则：

- `Tags["path"]` 非空 -> source `path`，名称取 tag 值。
- 否则 `Tags["query"]` -> `query`。
- 否则 `Tags["header"]` -> `header`。
- 否则按 HTTP method：GET/HEAD 时 `json` tag 或字段名映射 `query`，其他方法映射 `body`。
- `Verf` 取 `Tags["verf"]`；包含 `required` 时 `Required=true`。

- [ ] **Step 2: 注册表与 `addRoute` 钩子**

在 `Handler` 增加：

```go
type Handler struct {
    // 已有字段
    routeInfos []RouteInfo
    routeMu    sync.RWMutex
}

func (h *Handler) recordRoute(httpMethod, path string, handlers HandlersChain)
func (h *Handler) Routes() []RouteInfo
func (h *Handler) CLIRoute(method, path, resource, verb string)
```

`addRoute` 末尾调用 `recordRoute(httpMethod, path, handlers)`。`recordRoute` 只取 `handlers[len(handlers)-1]` 作为目标 handler，提取 operationID 和参数；推导 resource/verb；写入 `routeInfos`。

`CLIRoute` 按 `method+path` 找到已有 `RouteInfo` 并覆盖 `Resource/Verb`。

`Engine` 增加转发：

```go
func (e *Engine) CLIRoute(method, path, resource, verb string) {
    e.handler.CLIRoute(method, path, resource, verb)
}
```

- [ ] **Step 3: 测试**

`route_info_test.go` 覆盖：

- `TestDeriveVerb`：GET/POST/PUT/PATCH/DELETE/自定义方法。
- `TestDeriveResourceFromName`：`GetUser` -> `user`。
- `TestDeriveResourceFromPath`：`/api/v1/users/:id` -> `users`。
- `TestExtractParams`：`path`/`query`/`header`/`body` 字段、GET json 映射 query、verf required。
- `TestRoutesAndOverride`：注册后 `Routes()` 非空，`CLIRoute` 覆盖生效。

运行：

```bash
go test ./web/router/... ./pkg/basic/...
```

Expected: 全部通过。

- [ ] **Step 4: 提交**

```bash
git add web/router/route_info.go web/router/route_info_test.go web/router/handler.go web/engine.go
git commit -m "feat: add route metadata registry and cli resource derivation"
```

---

### Task 3: `/cli/routes` 发现接口

**Files:**
- Modify: `web/app/server/config/options.go`
- Modify: `web/router/handler.go`
- Create: `web/router/cli_routes_test.go`

**Interfaces:**
- Consumes: `Handler.Routes()`
- Produces:
  - `config.WithEnableCLI(bool) Option`
  - `config.WithCLIRoutesPath(string) Option`
  - `Options.EnableCLI bool`
  - `Options.CLIRoutesPath string`

- [ ] **Step 1: 配置项**

在 `Options` 增加 `EnableCLI bool`、`CLIRoutesPath string`（默认 `/cli/routes`），新增 `WithEnableCLI` 和 `WithCLIRoutesPath`。

- [ ] **Step 2: 特殊请求处理**

在 `getHandlerCfg` 传入 `EnableCLI` 和 `CLIRoutesPath`；`HandlerCfg` 增加同名字段。在 `specialRequest` 中处理：

```go
if h.config.EnableCLI && h.config.CLIRoutesPath != "" && path == h.config.CLIRoutesPath {
    payload := cliMetadata{
        Name:      h.config.Name,
        Version:   versionOfRoutes(h.Routes()),
        Resources: resourceList(h.Routes()),
        Routes:    h.Routes(),
    }
    body, _ := hjson.Marshal(payload)
    if etagMatch(ctx, versionOfRoutes(h.Routes())) {
        ctx.SetStatusCode(http.StatusNotModified)
        return true
    }
    ctx.SetHeader("ETag", `"v1-`+versionOfRoutes(h.Routes())+`"`)
    ctx.Write(body)
    return true
}
```

`versionOfRoutes` 使用 SHA256 摘要；`etagMatch` 比较 `If-None-Match`。

- [ ] **Step 3: 测试**

`cli_routes_test.go`：

- 未启用时请求路径返回 404。
- 启用后返回 JSON，包含 `name/version/resources/routes`。
- 携带匹配 ETag 返回 304。

运行：

```bash
go test ./web/router/... -run TestCLIRoutes
```

- [ ] **Step 4: 提交**

```bash
git add web/app/server/config/options.go web/router/handler.go web/router/cli_routes_test.go
git commit -m "feat: expose cli route metadata endpoint with etag"
```

---

### Task 4: `web/cli` root 与 `serve`

**Files:**
- Create: `web/cli/options.go`
- Create: `web/cli/root.go`
- Create: `web/cli/serve.go`
- Create: `web/cli/root_test.go`

**Interfaces:**
- Consumes: `*web.Engine`、`global.DefaultResourceManger`
- Produces:
  - `type Option func(*options)`
  - `func WithName(string) Option`
  - `func WithResourceManager(ResourceManager) Option`
  - `type ResourceManager interface { AddDaemonWithOrder(global.DaemonResource, int); Signal() }`
  - `func New(engine *web.Engine, opts ...Option) *cobra.Command`
  - `func Run(engine *web.Engine, opts ...Option) error`

- [ ] **Step 1: options 与接口**

`options.go` 定义 `ResourceManager`、`options` 默认值（name 取 engine.Name()，manager 取 `global.DefaultResourceManger`，serve order 500）。

- [ ] **Step 2: root 与 serve**

`root.go`：

```go
func New(engine *web.Engine, opts ...Option) *cobra.Command {
    cfg := defaultOptions(engine, opts...)
    root := &cobra.Command{Use: cfg.name}
    root.AddCommand(serveCommand(engine, cfg))
    root.AddCommand(routesCommand(engine, cfg))
    addDynamicCommands(root, engine.Routes())
    addGlobalFlags(root, cfg)
    return root
}

func Run(engine *web.Engine, opts ...Option) error {
    return New(engine, opts...).Execute()
}
```

`serve.go`：

```go
func serveCommand(engine *web.Engine, cfg *options) *cobra.Command {
    return &cobra.Command{
        Use: "serve",
        RunE: func(cmd *cobra.Command, args []string) error {
            cfg.manager.AddDaemonWithOrder(engine, cfg.serveOrder)
            cfg.manager.Signal()
            return nil
        },
    }
}
```

- [ ] **Step 3: 测试**

`root_test.go` 用 fake `ResourceManager` 验证 `serve` 调用了 `AddDaemonWithOrder` 和 `Signal`；验证 `Run` 返回 root command。

运行：

```bash
go test ./web/cli/...
```

- [ ] **Step 4: 提交**

```bash
git add web/cli
git commit -m "feat: add web cli root command and serve via resource manager"
```

---

### Task 5: 本地动态命令生成

**Files:**
- Create: `web/cli/command.go`
- Create: `web/cli/command_test.go`

**Interfaces:**
- Consumes: `router.RouteInfo`
- Produces: `func buildVerbCommand(route router.RouteInfo, client *Client) *cobra.Command`
- Client 方法：
  - `func (c *Client) Execute(ctx context.Context, route router.RouteInfo, values map[string]string, body []byte) (*protocol.Response, error)`

- [ ] **Step 1: 命令生成**

`command.go`：

- 按 `verb resource` 生成命令；resource 为空时跳过资源命令。
- 每个 `ParamInfo` 生成 flag：path/query/header 用 `--<name>`，body 参数统一使用 `--data`/`-f`。
- 冲突资源命令通过 `routesCommand` 下的 `call <operationID>` 保留；`call` 命令接受 `--route` 定位具体路由。
- RunE 中收集 flag 值，构造 `map[string]string` 和 body，调用 `Client.Execute`。

- [ ] **Step 2: 测试**

`command_test.go` 覆盖：

- `get users` 生成 `id` flag。
- `call GetUser` 兜底命令可执行。
- 必填参数缺失时报错。

运行：

```bash
go test ./web/cli/... -run TestBuildCommand
```

- [ ] **Step 3: 提交**

```bash
git add web/cli/command.go web/cli/command_test.go
git commit -m "feat: generate dynamic cli commands from route metadata"
```

---

### Task 6: HTTP 执行与输出

**Files:**
- Create: `web/cli/request.go`
- Create: `web/cli/output.go`
- Create: `web/cli/request_test.go`

**Interfaces:**
- Consumes: `web/app/client`、`router.RouteInfo`
- Produces:
  - `type Client struct { Server string; Token string; Headers []string }`
  - `func (c *Client) Execute(ctx context.Context, route router.RouteInfo, values map[string]string, body []byte) ([]byte, error)`
  - `func PrintOutput(w io.Writer, format string, body []byte) error`

- [ ] **Step 1: 请求构造**

`request.go`：

- path 参数替换 `:name`/`*name`。
- query 参数用 `url.Values` 追加。
- header 参数和 `--header`/`--token` 写入请求头。
- body 写入 `--data` 或 `-f` 内容。
- 使用 `client.NewClient(client.WithClientReadTimeout(10*time.Second))`，`Do` 后返回 body。

- [ ] **Step 2: 输出**

`output.go`：

- `json`：原样输出。
- `yaml`：`yaml.Marshal` 后输出。
- `table`：解析 `resp.Result`，单对象键值对，数组按首元素字段输出列。

- [ ] **Step 3: 测试**

`request_test.go` 用 `httptest.Server` 验证 path/query/header/body 正确；`output_test.go` 验证三种格式。

运行：

```bash
go test ./web/cli/... -run 'TestExecute|TestPrintOutput'
```

- [ ] **Step 4: 提交**

```bash
git add web/cli/request.go web/cli/output.go web/cli/request_test.go
git commit -m "feat: execute cli requests and render output formats"
```

---

### Task 7: 远程发现与缓存

**Files:**
- Create: `web/cli/discovery.go`
- Create: `web/cli/cache.go`
- Create: `web/cli/discovery_test.go`
- Modify: `web/cli/root.go`

**Interfaces:**
- Consumes: `/cli/routes`、`router.RouteInfo`
- Produces:
  - `type Metadata struct { Name string; Version string; Resources []string; Routes []router.RouteInfo }`
  - `func Discover(ctx context.Context, server string, opts CacheOptions) (Metadata, error)`
  - `func (c *Client) Server(server string)`

- [ ] **Step 1: 缓存**

`cache.go`：

- 缓存路径默认 `os.UserCacheDir()/web-cli/<sha256(server)>.json`。
- 文件包含 `Metadata` + `FetchedAt`。
- `--cache-ttl` 默认 5 分钟；`--refresh` 忽略 TTL。
- 命中 TTL 直接返回；过期请求携带 `If-None-Match: v1-<version>`。
- 200 更新缓存；304 保留缓存并更新 `FetchedAt`；网络失败有缓存则降级返回并记录警告。

- [ ] **Step 2: 发现**

`discovery.go` 请求 `server + /cli/routes`，反序列化 `Metadata`，返回路由列表。

- [ ] **Step 3: root 接入**

`root.go` 中 `--server` 非空时用 `Discover` 结果替代 `engine.Routes()` 生成命令；`routes` 命令支持 `--refresh`。

- [ ] **Step 4: 测试**

`discovery_test.go` 覆盖缓存命中、过期刷新、304、强制刷新、降级警告；用 `httptest.Server` 模拟 `/cli/routes`。

运行：

```bash
go test ./web/cli/... -run 'TestDiscover|TestCache'
```

- [ ] **Step 5: 提交**

```bash
git add web/cli/discovery.go web/cli/cache.go web/cli/discovery_test.go web/cli/root.go
git commit -m "feat: add remote route discovery with local metadata cache"
```

---

### Task 8: 文档与回归

**Files:**
- Modify: `docs/web.md`
- Modify: `openspec/changes/web-cli-dual-mode/tasks.md`

- [ ] **Step 1: 更新文档**

`docs/web.md` 增加“命令行双模式”章节，包含 `serve`、本地命令、`--server` 远程模式、`/cli/routes` 配置与安全说明。

- [ ] **Step 2: 全量测试**

```bash
go test ./web/...
```

Expected: 全部通过。

- [ ] **Step 3: 勾选 tasks.md**

将 `openspec/changes/web-cli-dual-mode/tasks.md` 中对应任务全部改为 `- [x]`。

- [ ] **Step 4: 提交**

```bash
git add docs/web.md openspec/changes/web-cli-dual-mode/tasks.md
git commit -m "docs: document web cli dual mode and mark tasks complete"
```
