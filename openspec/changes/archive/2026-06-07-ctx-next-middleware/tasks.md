## 1. RequestContext 扩展

- [x] 1.1 在 `app.RequestContext` 上新增 `handlers`（`app.HandlersChain`）和 `handlerIndex`（`int`）字段
- [x] 1.2 实现 `Next(ctx context.Context)` 方法：递增 handlerIndex，循环执行后续 handler，检查 Abort 状态
- [x] 1.3 实现 `SetHandlers(handlers app.HandlersChain)` 和 `GetHandlers()` 方法
- [x] 1.4 在 `Reset()` 方法中重置 handlers 和 handlerIndex

## 2. method.Method 统一 Invoke 方法

- [x] 2.1 在 `method.Method` 上新增 `ToHandlerFunc(invokeFunc)` 方法
- [x] 2.2 HandlerFuncTypeOfMethod 的 ToHandlerFunc：直接返回 handlerFunc
- [x] 2.3 DefaultTypeOfMethod 的 ToHandlerFunc：包装 invokeFunc 调用
- [x] 2.4 GrpcTypeOfMethod 的 ToHandlerFunc：包装 invokeFunc 调用

## 3. Handler 调度逻辑重构

- [x] 3.1 修改 `getTargetMethod`：移除中间件顺序执行代码，改为将 handlers chain 存入 RequestContext
- [x] 3.2 修改 `Dispatch` 流程：路由匹配后调用 `ctx.Next(ctx)` 启动 handler chain 执行
- [x] 3.3 新增 `invokeMethod`/`invokeDefaultMethod`/`invokeGrpcMethod` 方法处理不同类型 handler
- [x] 3.4 确保 Abort 语义正确：Next() 中检查 IsAbort()，Abort() 后不再执行后续 handler

## 4. 测试

- [x] 4.1 测试中间件调用 ctx.Next() 后置逻辑执行
- [x] 4.2 测试多个中间件链式调用 Next()
- [x] 4.3 测试不调用 Next() 的中间件行为（兼容性）
- [x] 4.4 测试 Abort() 中断后续 handler 执行
- [x] 4.5 测试 DefaultTypeOfMethod 路由与 Next() 的配合
- [x] 4.6 测试路由组级别中间件与 Next() 的配合
- [x] 4.7 运行全部现有测试确保向后兼容

## 5. Interceptor 废弃标记

- [x] 5.1 标记 Interceptor/ItemSort/Item 接口为 Deprecated
- [x] 5.2 标记 AddInterceptor/SortInterceptors 方法为 Deprecated
- [x] 5.3 标记 server.go 接口中 AddInterceptor 为 Deprecated
- [x] 5.4 标记 LoggerInterceptor 为 Deprecated，提供 Use 中间件替代示例
- [x] 5.5 更新 docs/web.md 文档
