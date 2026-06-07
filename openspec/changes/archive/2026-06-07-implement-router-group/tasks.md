## 1. RouteRegistrar 接口定义

- [ ] 1.1 在 `web/router/routergroup.go` 中定义 `RouteRegistrar` 接口，包含 `addRoute(httpMethod string, path string, handlers HandlersChain)` 方法
- [ ] 1.2 在 `web/router/handler.go` 中为 `Handler` 实现 `addRoute` 方法：检查 `trees` 中是否存在对应 HTTP 方法的 `router`，不存在则创建，然后调用 `router.addRoute(path, handlers)`

## 2. 新增 HandlerFuncTypeOfMethod

- [ ] 2.1 在 `web/router/method/method.go` 中新增 `HandlerFuncTypeOfMethod` 常量
- [ ] 2.2 新增 `NewHandlerFuncTypeMethod(handlerFunc app.HandlerFunc) *Method` 构造函数，将 `app.HandlerFunc` 包装为 `method.Method`
- [ ] 2.3 确保 `Method.GetInfo()` 能返回 `HandlerFuncTypeOfMethod` 类型和底层 `app.HandlerFunc`

## 3. 重写 RouterGroup

- [ ] 3.1 修改 `RouterGroup` 结构体：将 `engine *Engine` 改为 `engine RouteRegistrar`
- [ ] 3.2 修改 `IRoutes` 接口：移除 `StaticFile/Static/StaticFS` 方法声明
- [ ] 3.3 移除 `RouterGroup` 上的 `StaticFile/Static/StaticFS` 方法实现
- [ ] 3.4 移除 `rConsts` 导入，在 `routergroup.go` 中定义 `const abortIndex int8 = math.MaxInt8 / 2`，替换 `combineHandlers` 中的 `rConsts.AbortIndex` 引用
- [ ] 3.5 修改 `returnObj()` 方法：无论 `root` 是否为 true，均返回 `RouterGroup` 自身
- [ ] 3.6 修改 `GET/POST/PUT/DELETE/PATCH/OPTIONS/HEAD` 方法签名：将 `handlers ...app.HandlerFunc` 改为 `handler interface{}`，实现自动检测逻辑：
  - `app.HandlerFunc` → `NewHandlerFuncTypeMethod`
  - `method.Method` → 直接使用
  - 其他函数 → `basic.NewMethod(nil, handler)` → `DefaultTypeOfMethod`
  - 其他 → panic
- [ ] 3.7 修改 `RouterGroup.handle()` 方法：适配新的 handler 参数类型，将检测结果转为 `HandlersChain` 后调用 `engine.addRoute()`
- [ ] 3.8 新增 `GRPC(httpMethod string, path string, handler func(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error), srv interface{})` 方法，接受 HTTP 方法名 + protoc 生成的 handler + srv，内部通过 `runtime.FuncForPC` 提取方法名，从 srv 反射获取 targetMethod，构造 `method.NewGrpcTypeMethod` 注册 gRPC 路由
- [ ] 3.9 验证 `routergroup.go` 可编译通过

## 4. 适配 Handler.Dispatch

- [ ] 4.1 修改 `Handler.Dispatch`：在 `getTargetMethod` 返回后，根据 `MethodType` 区分三种分发路径
  - `DefaultTypeOfMethod`：现有反射调用 + 自动参数解析（不变）
  - `GrpcTypeOfMethod`：现有 gRPC 分发（不变）
  - `HandlerFuncTypeOfMethod`：直接调用 `app.HandlerFunc(ctx, reqCtx)`
- [ ] 4.2 处理中间件链：`Use()` 注册的 `app.HandlerFunc` 中间件在最终 handler 前依次执行

## 5. Engine 暴露 RouterGroup

- [ ] 5.1 在 `web/engine.go` 中为 `Engine` 添加 `routerGroup *router.RouterGroup` 字段
- [ ] 5.2 修改 `web.Default()` 初始化逻辑：创建 `Handler` 后，用 `Handler` 作为 `RouteRegistrar` 创建根 `RouterGroup` 并赋值给 `Engine.routerGroup`
- [ ] 5.3 在 `Engine` 上添加委托方法：`Group/GET/POST/PUT/DELETE/PATCH/OPTIONS/HEAD/Any/Use`（接受 `interface{}`，自动检测签名）
- [ ] 5.4 在 `Engine` 上添加 `GRPC(path, methodDesc, srv)` 委托方法
- [ ] 5.5 确保 `Engine` 的 `Handler` 可被外部访问（如需通过 `Engine` 获取 `Handler` 来调用 `Register` 等现有方法）

## 6. 兼容性验证

- [ ] 6.1 运行 `web/test/handler_test.go` 全部测试用例，确保现有功能不受影响
- [ ] 6.2 运行 `web/test/netx_server_test.go` 全部测试用例，确保服务器启动和关闭正常
- [ ] 6.3 确认 `Handler.Register()` + `controller.RestfulController` 注册的路由仍可正常分发

## 7. 新增 RouterGroup 功能测试

- [ ] 7.1 编写测试：通过 `Engine.GET()` 注册 HandlerFunc 签名的路由
- [ ] 7.2 编写测试：通过 `Engine.GET()` 注册结构体方法，验证自动参数解析
- [ ] 7.3 编写测试：通过 `Engine.GRPC()` 注册 gRPC 路由，验证非反射分发
- [ ] 7.4 编写测试：通过 `Engine.Use()` 添加中间件，验证中间件在 handler 前执行
- [ ] 7.5 编写测试：嵌套 Group 路径拼接正确性
- [ ] 7.6 编写测试：RouterGroup 注册的路由与 RestfulController 注册的路由共存
