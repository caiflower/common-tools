---
comet_change: web-cli-dual-mode
role: technical-design
canonical_spec: openspec
---

# Web CLI Dual Mode — Technical Design

## Architecture

同一个二进制提供两种模式：`serve` 模式启动 HTTP 服务，CLI 模式通过 HTTP 调用已注册接口。路由只注册一次，元数据由框架在注册时自动收集。

```text
main.go
  └─ web.Default(...) + 路由注册
       └─ web/cli.New(engine).Execute()
            ├─ serve ──> ResourceManager.AddDaemonWithOrder(engine) + Signal()
            ├─ routes ──> 本地元数据 / 远端发现
            └─ get/create/... 动态命令
                 ├─ 本地: engine.Routes()
                 └─ 远端: GET /cli/routes + 本地缓存
                      └─ web/app/client 发送 HTTP 请求
```

## Component Details

### 1. 路由元数据（`web/router`）

新增导出类型：

```go
type ParamInfo struct {
    Name     string `json:"name"`
    Source   string `json:"source"` // path|query|header|body
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
```

`Handler` 新增线程安全注册表和 `Routes()`：

```go
type Handler struct {
    // ...
    routeInfos []RouteInfo
    routeMu    sync.RWMutex
}

func (h *Handler) Routes() []RouteInfo
```

`addRoute` 在写入方法树后，从 `HandlersChain` 最后一个 handler 提取元数据：

- `method.Method` / gRPC 路由：`GetAction()` 得到 operationID，`GetTargetMethod()`/`ArgInfo` 得到参数。
- `HandlerFuncTypeOfMethod`：无法得到稳定方法名时，用 method + path 生成 operationID，参数为空。
- 参数来源映射：`path`/`query`/`header` tag 对应各自来源；GET/HEAD 下 `json` tag 映射 query，其他方法映射 body，与 `setArgsOptimized` 一致。

### 2. resource/verb 推导与显式覆盖

推导优先级：显式覆盖 > 方法名 > path。

- 方法名规则：`GetUser` -> verb `get`、resource `user`；`CreateOrder` -> `create`/`order`。
- path 兜底：匿名 handler 取 path 最后一个静态段作为 resource。
- verb 映射：`GET->get`、`POST->create`、`PUT->update`、`PATCH->patch`、`DELETE->delete`，其余 `call`。
- 冲突处理：同名 `verb resource` 命令冲突时保留自动推导命令，同时生成 `call <operationID>`。

显式覆盖 API：

```go
func (e *Engine) CLIRoute(method, path, resource, verb string)
```

在路由注册前后均可调用，按 `method+path` 覆盖元数据中的 resource/verb，不改现有 `IRoutes` 链式 API。

### 3. `/cli/routes` 发现接口

配置：

```go
config.WithEnableCLI(true)
config.WithCLIRoutesPath("/cli/routes")
```

默认关闭。开启后在 `Handler.specialRequest` 中处理 `GET /cli/routes`：

```json
{
  "name": "myapp",
  "version": "sha256-of-metadata",
  "resources": ["user", "users"],
  "routes": []
}
```

- `version` 使用元数据 JSON 的 SHA256，作为 `ETag: "v1-<sha256>"`。
- 请求带匹配的 `If-None-Match` 时返回 304，无 body。
- 元数据变化会导致 ETag 变化，CLI 据此判断是否更新缓存。

### 4. `web/cli` 双模式入口

```go
type ResourceManager interface {
    AddDaemonWithOrder(global.DaemonResource, int)
    Signal()
}

func New(engine *web.Engine, opts ...Option) *cobra.Command
func Run(engine *web.Engine, opts ...Option) error
```

`New` 构建 root command：

- `serve`：`manager.AddDaemonWithOrder(engine, order)` + `manager.Signal()`，默认 manager 为 `global.DefaultResourceManger`，默认 order 500。
- `routes`：列出接口；支持 `--refresh` 强制刷新远端缓存。
- 动态命令：遍历 `engine.Routes()` 生成 `verb resource`；参数 flag 来自 `ParamInfo`。

全局 flag：

```text
--server <url>      远端服务地址；缺省用 engine 监听地址
--token <value>     附加 Authorization: Bearer <value>
--header k=v        可重复，附加请求头
--output table|json|yaml
--refresh           强制刷新远端元数据
--cache-ttl 5m      缓存有效期
--cache-dir <path>  缓存目录
```

### 5. 请求构造与输出

请求构造：

- path 参数替换路径模板中的 `:name`/`*name`。
- query 参数拼接到 URL。
- header 参数设置请求头。
- body 参数从 `--data` JSON 或 `-f file` 读取；未提供时发送空 body。
- 使用 `web/app/client` 执行请求，统一解析 `resp.Result`。

输出：

- `json`：原样输出统一响应 JSON。
- `yaml`：将响应结构转 YAML。
- `table`：单对象输出键值对；数组输出首元素字段作为列名，保持最小可用实现。

### 6. 远程发现与缓存

远程模式流程：

1. 以 `--server` 地址 hash 计算缓存文件名。
2. 缓存命中且未超过 TTL：直接使用。
3. 缓存过期：携带缓存 version 的 `If-None-Match` 请求 `/cli/routes`。
4. 200：更新缓存和 version；304：保留缓存并刷新时间戳。
5. 网络失败但有缓存：使用旧缓存并输出警告；无缓存：返回明确错误。

## Data Flow

```text
用户执行 myapp get users --id=1
  → cobra 解析到动态命令 get users
  → 本地模式: engine.Routes() 获取元数据
  → 远端模式: 缓存/ /cli/routes 获取元数据
  → 构造 GET /users/1
  → web/app/client 发送到 http://127.0.0.1:8080 或 --server
  → 输出 table/json/yaml
```

## Error Handling

- 本地模式且 server 未启动：连接失败，提示“请先运行 `<binary> serve`”。
- 远端模式且无缓存、服务不可达：明确提示无法获取元数据，建议 `--refresh` 或检查地址。
- 参数校验失败：CLI 侧先做必填检查，再由服务端校验兜底。
- 动态命令生成失败：保留 `call <operationID>` 命令，保证每个接口仍可调用。

## Testing Strategy

- 元数据提取与推导：普通函数、结构体方法、gRPC、匿名 HandlerFunc、显式覆盖。
- `/cli/routes`：默认关闭、开启后内容、ETag/304。
- 命令生成：`verb resource`、`call` 兜底、flag 生成。
- 请求构造：path/query/header/body 组合。
- 资源管理器：`serve` 注册 engine 并调用 `Signal()`。
- 缓存：命中、TTL 过期、304、`--refresh`、降级警告。
- 回归：`go test ./web/...`。

## Migration Plan

- 纯新增能力，`web.Default` 和现有路由注册方式不变。
- `web/cli` 为可选集成，不改变已有 server 启动路径。
- 实现顺序：路由元数据 -> `/cli/routes` -> CLI 本地模式 -> 远程发现/缓存 -> 文档。
