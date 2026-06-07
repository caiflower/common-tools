## Why

当前框架的 `RouterGroup`（`web/router/routergroup.go`）是从 Hertz 直接复制过来的，但存在多个编译和架构兼容性问题：`engine` 字段引用了不存在的 `*Engine` 类型、`rConsts.AbortIndex` 引用缺失、`app.FS` 类型不存在等。同时，当前框架的路由注册方式仅支持通过 `Handler.Register()` + `controller.RestfulController` 的高级 API，缺少 Hertz 风格的 `Group().GET()/POST()` 链式路由注册能力。需要重新实现 RouterGroup，使其与当前框架架构匹配，同时保持旧版 API 完全兼容。

## What Changes

- 定义 `RouteRegistrar` 接口，替代 `RouterGroup` 对具体 `*Engine` 类型的依赖，实现 `router` 包与 `web` 包的解耦
- 重写 `routergroup.go`：将 `engine` 字段类型改为 `RouteRegistrar`，移除 `StaticFile/Static/StaticFS` 方法（当前框架无 `app.FS` 和 `ctx.File()`），移除 `rConsts` 引用并内联定义 `abortIndex` 常量，调整 `returnObj()` 逻辑
- 在 `Handler` 上实现 `addRoute` 方法，直接操作 `trees MethodTrees`，支持 `app.HandlerFunc` 链式注册
- 适配 `Handler.Dispatch` 逻辑，确保通过 RouterGroup 注册的 `HandlersChain`（`[]app.HandlerFunc`）能被正确分发执行
- 在 `web.Engine` 上暴露 RouterGroup 能力，持有 `RouterGroup` 实例，对外提供 `GET/POST/Group/Use` 等便捷方法
- 保留现有 `Handler.Register()` + `controller.RestfulController` API 不变，确保旧版兼容

## Capabilities

### New Capabilities
- `route-registrar`: 路由注册接口定义，解耦 RouterGroup 与具体 Engine 的依赖关系
- `router-group`: RouterGroup 核心功能实现，包括 Group/Use/GET/POST/PUT/DELETE/PATCH/OPTIONS/HEAD/Any/Handle 及 EX 变体方法
- `handler-chain-dispatch`: HandlerFunc 链式分发执行能力，使通过 RouterGroup 注册的中间件链能被正确执行
- `engine-router-group`: Engine 层暴露 RouterGroup 便捷方法，使用户可像 Hertz 一样使用链式路由注册

### Modified Capabilities
<!-- 无现有 spec 需要修改 -->

## Impact

- **代码文件**：`web/router/routergroup.go`（重写）、`web/router/handler.go`（新增 addRoute 方法、适配 Dispatch）、`web/engine.go`（新增 RouterGroup 暴露）
- **API 变更**：移除 `IRoutes` 接口中的 `StaticFile/Static/StaticFS`（当前不可用），新增 `RouteRegistrar` 接口
- **兼容性**：现有 `Handler.Register()` + `controller.RestfulController` API 完全不变，旧版测试用例无需修改
- **依赖**：无新增外部依赖
