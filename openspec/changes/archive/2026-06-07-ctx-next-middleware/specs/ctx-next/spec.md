## ADDED Requirements

### Requirement: RequestContext.Next() method
RequestContext SHALL provide a `Next(ctx context.Context)` method that increments the handler index and executes the next handler in the chain. When `Next()` is called, the current middleware pauses, the subsequent handlers execute, and control returns to the middleware after `Next()` completes.

#### Scenario: Middleware calls Next() to execute subsequent handlers
- **WHEN** a middleware calls `ctx.Next(ctx)` during its execution
- **THEN** the next handler in the chain SHALL execute, and after it completes, execution SHALL resume in the middleware after the `Next()` call

#### Scenario: Next() called when no more handlers remain
- **WHEN** `ctx.Next(ctx)` is called and the handler index is already at or past the end of the chain
- **THEN** `Next()` SHALL return immediately without executing any handler

#### Scenario: Next() respects Abort
- **WHEN** `ctx.Abort()` has been called (setting abort state)
- **THEN** `Next()` SHALL stop executing further handlers and return immediately

### Requirement: RequestContext stores handler chain state
RequestContext SHALL store the handler chain and current execution index to support `Next()` traversal.

#### Scenario: Handler chain is set before execution begins
- **WHEN** a route is matched and handlers are found
- **THEN** the full handler chain SHALL be stored on the RequestContext with handler index starting at 0

#### Scenario: Handler index advances after each handler
- **WHEN** a handler in the chain completes execution
- **THEN** the handler index SHALL advance to the next position

### Requirement: method.Method.Invoke unified method
`method.Method` SHALL provide an `Invoke(ctx context.Context, reqCtx *app.RequestContext)` method that dispatches to the appropriate execution path based on MethodType.

#### Scenario: Invoke for HandlerFuncTypeOfMethod
- **WHEN** `Invoke()` is called on a Method with type HandlerFuncTypeOfMethod
- **THEN** it SHALL directly call the stored `handlerFunc(ctx, reqCtx)`

#### Scenario: Invoke for DefaultTypeOfMethod
- **WHEN** `Invoke()` is called on a Method with type DefaultTypeOfMethod
- **THEN** it SHALL execute parameter parsing, validation, and reflection-based method invocation

#### Scenario: Invoke for GrpcTypeOfMethod
- **WHEN** `Invoke()` is called on a Method with type GrpcTypeOfMethod
- **THEN** it SHALL execute the gRPC method descriptor handler
