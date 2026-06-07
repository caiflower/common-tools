## Context

当前 RouterGroup 的中间件执行机制是在 `handler.go` 的 `getTargetMethod` 中顺序遍历 `HandlersChain`，逐个调用 `InvokeHandlerFunc`，最后返回最后一个 handler 作为 target method。这种方式不支持 `ctx.Next()` 模式——中间件无法暂停执行、让后续 handler 运行后再恢复。

现有代码结构：
- `app.RequestContext` 有 `Abort()` 方法（设置 `special = -1`），但没有 `Next()` 方法
- `HandlersChain` 是 `[]method.Method`，存储在 tree node 中
- 中间件和 handler 在 `routergroup.go` 的 `handle()` 方法中合并为一个 `HandlersChain`
- `getTargetMethod` 中顺序执行 `res.handlers[0..n-2]`，返回 `res.handlers[n-1]`

## Goals / Non-Goals

**Goals:**
- 支持 `ctx.Next(ctx)` 调用，使中间件可以在 handler 执行前后都有逻辑
- 保持向后兼容：不调用 `Next()` 的中间件行为不变
- 与 Hertz/Gin 的 `ctx.Next()` 语义一致

**Non-Goals:**
- 不修改 `method.Method` 的类型系统（HandlerFuncTypeOfMethod/DefaultTypeOfMethod/GrpcTypeOfMethod 保持不变）
- 不改变 RouterGroup 的 `Use()`/`Group()` API
- 不支持 goroutine 中调用 `Next()`（与 Hertz/Gin 一致）

## Decisions

### Decision 1: 在 RequestContext 上新增 Next() 方法

在 `app.RequestContext` 上新增 `handlers` 和 `handlerIndex` 字段，以及 `Next(ctx context.Context)` 方法。

`Next()` 的实现逻辑：
```
func (ctx *RequestContext) Next(c context.Context) {
    ctx.handlerIndex++
    for ctx.handlerIndex < len(ctx.handlers) {
        if ctx.IsAbort() {
            return
        }
        handler := ctx.handlers[ctx.handlerIndex]
        handler(c, ctx)
        ctx.handlerIndex++
    }
}
```

**替代方案**：将 handler chain 执行逻辑放在 handler.go 中，RequestContext 只存储 index。但这样 Next() 需要访问 Handler 实例，耦合度高。将执行逻辑放在 RequestContext 上更符合 Hertz/Gin 的设计。

### Decision 2: 修改 getTargetMethod 为设置 handlers chain 而非立即执行

当前 `getTargetMethod` 中立即执行中间件链。改为：
1. 将 `res.handlers`（全部 handlers，包括中间件和最终 handler）存入 `RequestContext`
2. 设置 `handlerIndex = 0`
3. 调用第一个 handler（或通过 `Next()` 启动链）

但这里有一个问题：`HandlersChain` 是 `[]method.Method`，而 `Next()` 需要调用的是 `app.HandlerFunc`。对于 `HandlerFuncTypeOfMethod` 可以直接调用，但 `DefaultTypeOfMethod` 和 `GrpcTypeOfMethod` 需要不同的调用方式。

**决策**：在 `method.Method` 上新增 `Invoke(ctx context.Context, reqCtx *app.RequestContext)` 统一方法，内部根据 `MethodType` 分发：
- `HandlerFuncTypeOfMethod`：直接调用 `handlerFunc`
- `DefaultTypeOfMethod`：走原有的参数解析 + 反射调用逻辑
- `GrpcTypeOfMethod`：走原有的 gRPC 调用逻辑

这样 `Next()` 只需调用 `handler.Invoke(ctx, reqCtx)` 即可，无需关心具体类型。

### Decision 3: 执行入口调整

修改 `handler.go` 中的 `doTargetMethod` 逻辑：
- 移除 `getTargetMethod` 中的中间件顺序执行代码
- 在 `Dispatch` 流程中，找到路由后，将 handlers chain 存入 RequestContext，然后调用 `ctx.Next(ctx)` 启动整个链
- 最后一个 handler 执行完毕后，`Next()` 自然返回，中间件的后置逻辑继续执行

### Decision 4: Abort 语义保持

`ctx.Abort()` 设置 `special = -1`，`Next()` 循环中检查 `IsAbort()` 来决定是否中断。这与现有 Abort 行为一致。

## Risks / Trade-offs

- **[性能]** `Next()` 递归调用比顺序遍历多一层函数调用开销 → 可忽略不计
- **[兼容性]** 现有 `getTargetMethod` 中直接执行中间件的逻辑需要移除，改为 `Next()` 驱动 → 需要仔细测试所有路由类型（HandlerFunc/Default/GRPC）
- **[DefaultTypeOfMethod 的 Next 支持]** DefaultTypeOfMethod 的 handler 不是 `app.HandlerFunc`，需要通过 `Invoke` 方法包装 → 在 `Invoke` 内部走完整的参数解析+校验+调用流程
