## MODIFIED Requirements

### Requirement: Middleware and handler unified in HandlersChain
RouterGroup's `handle()` method SHALL combine group middleware and route handlers into a single `HandlersChain` where all items are `method.Method` instances. The middleware are placed before the route handler, and the entire chain is executed via `ctx.Next()`.

#### Scenario: Middleware executes before handler via Next()
- **WHEN** a route is registered with group middleware via `Use()` and a handler
- **THEN** the middleware SHALL execute first, and if it calls `ctx.Next()`, the handler SHALL execute next

#### Scenario: Multiple middleware in chain
- **WHEN** multiple middleware are registered via `Use()` calls
- **THEN** each middleware SHALL execute in registration order, and each can call `ctx.Next()` to proceed to the next handler in the chain
