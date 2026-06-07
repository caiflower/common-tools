## ADDED Requirements

### Requirement: HandlerFuncTypeOfMethod type
The system SHALL add a new `MethodType` constant `HandlerFuncTypeOfMethod` in the `method` package, and provide `NewHandlerFuncTypeMethod(handlerFunc app.HandlerFunc) *Method` to wrap an `app.HandlerFunc` as a `method.Method`.

#### Scenario: Wrap HandlerFunc as Method
- **WHEN** `method.NewHandlerFuncTypeMethod(handlerFunc)` is called
- **THEN** it SHALL return a `*method.Method` with `GetType() == HandlerFuncTypeOfMethod` and the original `app.HandlerFunc` stored for later invocation

#### Scenario: GetInfo returns HandlerFunc
- **WHEN** `GetInfo()` is called on a `HandlerFuncTypeOfMethod`
- **THEN** it SHALL return `HandlerFuncTypeOfMethod` as the type and provide access to the underlying `app.HandlerFunc`

### Requirement: HandlerFunc dispatch
When `Handler.Dispatch` encounters a route with `MethodType == HandlerFuncTypeOfMethod`, it SHALL directly invoke the `app.HandlerFunc` with `(context.Context, *app.RequestContext)` instead of using reflective argument parsing.

#### Scenario: Dispatch HandlerFunc route
- **WHEN** a request matches a route registered via `engine.GET("/path", handlerFunc)`
- **THEN** `handlerFunc(ctx, reqCtx)` SHALL be called directly without `setArgsOptimized` or `validArgs`

#### Scenario: Dispatch DefaultTypeOfMethod route (existing, unchanged)
- **WHEN** a request matches a route registered via `engine.GET("/path", userController.GetUser)` where the auto-detect wraps it as `DefaultTypeOfMethod`
- **THEN** the existing reflective dispatch with `setArgsOptimized` + `validArgs` SHALL be used unchanged

#### Scenario: Dispatch GrpcTypeOfMethod route (existing, unchanged)
- **WHEN** a request matches a route registered via `engine.GRPC("/path", methodDesc, srv)`
- **THEN** the existing gRPC dispatch via `grpc.MethodDesc.Handler` SHALL be used unchanged (non-reflective)

### Requirement: Middleware execution
Middleware registered via `RouterGroup.Use()` SHALL be executed before the final handler. The middleware SHALL be `app.HandlerFunc` and share the same `*app.RequestContext`.

#### Scenario: Middleware modifies context
- **WHEN** middleware sets a value via `ctx.Set("key", "value")` and the final handler reads `ctx.Get("key")`
- **THEN** the handler SHALL receive the value set by middleware

### Requirement: Compatibility with existing Dispatch
The new `HandlerFuncTypeOfMethod` dispatch SHALL NOT alter the behavior of existing `Handler.Dispatch` for `DefaultTypeOfMethod` and `GrpcTypeOfMethod` routes.

#### Scenario: Existing tests pass unchanged
- **WHEN** existing test cases in `handler_test.go` are executed
- **THEN** all tests SHALL pass without modification
