## Why

当前 RouterGroup 的 `Use()` 注册的中间件只能在 handler 之前执行，不支持"后置中间件"模式。用户无法在中间件中调用 `ctx.Next()` 让后续 handler 执行后再继续处理逻辑（如日志记录、响应修改、耗时统计等）。这是 Hertz/Gin 等主流框架的标准特性，缺少它会导致中间件表达能力不足。

## What Changes

- 在 `app.RequestContext` 上新增 `Next(ctx context.Context)` 方法，调用后暂停当前中间件，执行后续中间件和 handler，返回后继续执行当前中间件的剩余逻辑
- 修改 RouterGroup 的中间件执行机制：从顺序遍历改为递归调用链，支持 `ctx.Next()` 暂停/恢复
- 中间件和 handler 统一为 `app.HandlerFunc` 签名，通过 `ctx.Next()` 串联执行
- 保持向后兼容：不调用 `ctx.Next()` 的中间件行为与现有逻辑一致（前置执行）

## Capabilities

### New Capabilities
- `ctx-next`: RequestContext 上的 Next() 方法，支持中间件暂停当前执行、调用后续 handler 后恢复

### Modified Capabilities
- `handler-chain-dispatch`: 修改中间件链的执行方式，从顺序遍历改为递归调用链以支持 ctx.Next()
- `router-group`: RouterGroup 中 Use() 注册的中间件与 handler 统一为 HandlersChain，通过 Next() 串联

## Impact

- `web/app/context.go`：新增 Next() 方法及相关状态字段（handler index）
- `web/router/handler.go`：修改 getTargetMethod 中中间件执行逻辑，改为递归调用链
- `web/router/routergroup.go`：中间件与 handler 合并逻辑可能需要调整
- 现有不调用 Next() 的中间件完全兼容，无需修改
