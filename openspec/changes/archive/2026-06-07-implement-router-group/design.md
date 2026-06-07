## Context

当前框架 `web/router/routergroup.go` 是从 Hertz 直接复制而来，但框架架构与 Hertz 有本质差异：

- **Hertz**：`Engine` 在 `route` 包内，内嵌 `RouterGroup`，直接持有 `trees MethodTrees`，有 `addRoute` 方法
- **当前框架**：`Engine` 在 `web` 包，内嵌 `server.Core`；路由注册逻辑在 `router.Handler` 中，通过 `Handler.Register()` + `controller.RestfulController` 高级 API 完成

当前 `RouterGroup` 存在的编译问题：
1. `engine *Engine` — `router` 包内无 `Engine` 类型
2. `rConsts.AbortIndex` — 无 `route/consts` 包
3. `*app.FS` / `ctx.File()` — 无此类型和方法
4. `IRoutes` 接口与实现不一致（Static 方法注释掉但仍有实现）

## Goals / Non-Goals

**Goals:**
- 使 `RouterGroup` 可编译、可运行，与当前框架架构匹配
- 提供 Hertz 风格的链式路由注册能力：`engine.Group("/api").GET("/users", handler)`
- 支持中间件链：`engine.Use(middleware1).GET("/path", handler)`
- 保持现有 `Handler.Register()` + `controller.RestfulController` API 完全兼容
- `router` 包不反向依赖 `web` 包

**Non-Goals:**
- 不实现 StaticFile/Static/StaticFS（当前框架无 `app.FS` 和 `ctx.File()` 支持）
- 不重构 `Handler` 的现有 `Dispatch` 核心逻辑（仅扩展，不修改已有行为）
- 不实现 Hertz 的 `Engine` 内嵌 `RouterGroup` 模式（架构不同）
- 不实现 NoRoute/NoMethod 处理器

## Decisions

### Decision 1: 用 `RouteRegistrar` 接口替代 `*Engine` 依赖

**选择**：在 `router` 包内定义 `RouteRegistrar` 接口，`RouterGroup.engine` 字段类型改为 `RouteRegistrar`

**理由**：
- `router` 包不能反向依赖 `web` 包（会造成循环依赖）
- `Handler` 已有 `trees MethodTrees`，只需新增 `addRoute` 方法即可实现接口
- 接口解耦更灵活，未来其他类型也可实现 `RouteRegistrar`

**备选方案**：
- A) 在 `router` 包新建 `Engine` 结构体（类似 Hertz）→ 改动过大，与现有架构冲突
- B) 将 `RouterGroup` 移到 `web` 包 → 改变包结构，与 Hertz 组织方式不同

### Decision 2: 内联定义 `abortIndex` 常量

**选择**：在 `routergroup.go` 中直接定义 `const abortIndex int8 = math.MaxInt8 / 2`

**理由**：仅一处使用，无需新建 `consts` 包

### Decision 3: 移除 Static 相关方法

**选择**：从 `IRoutes` 接口和 `RouterGroup` 实现中均移除 `StaticFile/Static/StaticFS`

**理由**：当前框架无 `app.FS` 类型和 `ctx.File()` 方法，这些方法无法编译也无法运行。未来需要时可重新添加。

### Decision 4: 自动检测签名 + 统一分发方案

**选择**：RouterGroup 的 `GET/POST/PUT/DELETE/PATCH/OPTIONS/HEAD` 方法接受 `interface{}` 参数，注册时通过反射自动检测函数签名，内部统一存储为 `method.Method`，在 `Handler.Dispatch` 中根据 `MethodType` 区分分发路径。

**三种 MethodType 分发路径**：
- `DefaultTypeOfMethod`：反射调用，自动参数解析（`setArgsOptimized` + `validArgs`）— 现有逻辑不变
- `GrpcTypeOfMethod`：gRPC 分发，走 `grpc.MethodDesc.Handler`，非反射，性能更好 — 现有逻辑不变
- **新增 `HandlerFuncTypeOfMethod`**：直接调用 `app.HandlerFunc`，不解析参数

**自动检测逻辑**（在 `RouterGroup.handle` 中）：
```
handler 参数类型判断：
├── app.HandlerFunc → HandlerFuncTypeOfMethod（直接调用）
├── method.Method   → 直接使用（保留原始 MethodType）
├── 其他函数类型     → basic.NewMethod(nil, handler) → DefaultTypeOfMethod（自动参数解析）
└── 其他类型        → panic("unsupported handler type")
```

**gRPC 注册**：提供 `GRPC(httpMethod, path, handler, srv)` 方法，接受 HTTP 方法名、protoc 生成的 handler 函数和服务实例。内部通过 `runtime.FuncForPC` 从 handler 函数名提取方法名，再从 `srv` 反射获取 `targetMethod`，构造 `method.NewGrpcTypeMethod`。

**用户使用示例**：
```go
// 1. HandlerFunc 签名 - 自动识别，直接调用
engine.GET("/ping", func(ctx context.Context, reqCtx *app.RequestContext) {
    reqCtx.SetData("pong")
})

// 2. 任意函数签名 - 自动识别，自动参数解析
engine.GET("/users/:id", userController.GetUser)
engine.GET("/users", func(req *GetUsersReq) (*GetUsersResp, error) { ... })

// 3. gRPC - 传 HTTP 方法 + 生成的 handler + srv，非反射
engine.GRPC("POST", "/search", _IService_Search_Handler, &HelloImpl{})

// 4. 中间件 - 仅支持 app.HandlerFunc
engine.Use(func(ctx context.Context, reqCtx *app.RequestContext) { ... })
```

**gRPC 内部推导逻辑**：
```
1. runtime.FuncForPC(handler.Pointer()).Name() → "_IService_Search_Handler"
2. 正则提取方法名 → "Search"
3. basic.NewMethod(srv的Class, srv.Search方法) → targetMethod
4. grpc.MethodDesc{MethodName: "Search", Handler: handler}
5. method.NewGrpcTypeMethod(methodDesc, srv, targetMethod)
6. engine.addRoute(httpMethod, path, HandlersChain{grpcMethod})
```

**理由**：
- 用户只需 `engine.GET(path, handler)`，无需关心底层类型，API 最简洁
- 自动参数解析是框架核心优势，不应因使用 RouterGroup 而丢失
- gRPC 走 `MethodDesc.Handler` 非反射路径，性能不受影响
- 统一存储为 `method.Method`，无需修改 `tree.go`，`HandlersChain` 类型不变
- `Handler.Dispatch` 仅需在 `getTargetMethod` 返回后增加一个 `HandlerFuncTypeOfMethod` 分支

**备选方案**：
- A) 只支持 `app.HandlerFunc` → 失去自动参数解析能力，不符合框架设计理念
- B) 双签名分离（GET + GETMethod）→ 用户需要理解两种 API，增加认知负担
- C) 统一为 `app.HandlerFunc` → 需要大幅重构现有 `method.Method` 体系

### Decision 5: Engine 暴露 RouterGroup 的方式

**选择**：`Engine` 持有 `*router.RouterGroup` 实例，通过委托方法暴露 `GET/POST/Group/Use` 等

**理由**：
- 不内嵌 `RouterGroup`（避免暴露过多方法）
- 委托方式更清晰，可选择性暴露
- `RouterGroup` 的创建需要 `Handler` 作为 `RouteRegistrar`，在 `Engine` 初始化时完成

## Risks / Trade-offs

- **[风险] HandlerFunc 包装为 method.Method 的兼容性** → 包装后的 Method 在反射调用时需要特殊处理，需确保不影响现有 Dispatch 逻辑。通过在 `method.Method` 接口新增类型标记来区分。
- **[风险] RouterGroup 注册的路由与 RestfulController 注册的路由冲突** → 两者共用 `trees`，路径冲突时 `tree.addRoute` 会 panic，行为与 Hertz 一致，可接受。
- **[取舍] 移除 Static 方法** → 未来如需静态文件服务，需重新实现 `app.FS` 和 `ctx.File()`，但当前不是优先级。
