## ADDED Requirements

### Requirement: RouterGroup struct definition
The `RouterGroup` struct SHALL have fields: `Handlers app.HandlersChain`, `basePath string`, `engine RouteRegistrar`, `root bool`. It SHALL NOT reference any concrete Engine type.

#### Scenario: RouterGroup creation
- **WHEN** a new `RouterGroup` is created via `Group()` method
- **THEN** it SHALL inherit handlers from the parent group, calculate the absolute path, and reference the same `RouteRegistrar`

### Requirement: IRouter and IRoutes interfaces
`IRouter` SHALL extend `IRoutes` and add the `Group(string, ...app.HandlerFunc) *RouterGroup` method. `IRoutes` SHALL define: `Use`, `Handle`, `Any`, `GET`, `POST`, `DELETE`, `PATCH`, `PUT`, `OPTIONS`, `HEAD`. `IRoutes` SHALL NOT include `StaticFile`, `Static`, or `StaticFS`.

#### Scenario: IRoutes interface compliance
- **WHEN** a type implements all methods in `IRoutes`
- **THEN** it SHALL be assignable to `IRoutes` without implementing `StaticFile`, `Static`, or `StaticFS`

### Requirement: Group method
`RouterGroup.Group()` SHALL create a new `RouterGroup` with combined handlers and calculated absolute path. The new group SHALL reference the same `RouteRegistrar`.

#### Scenario: Create nested group
- **WHEN** `router.Group("/api").Group("/v1")` is called
- **THEN** the resulting group SHALL have basePath "/api/v1" and inherit all middleware from parent groups

### Requirement: Use method
`RouterGroup.Use()` SHALL append middleware handlers to the group's handler chain and return `IRoutes`.

#### Scenario: Add middleware to group
- **WHEN** `group.Use(middleware1, middleware2)` is called
- **THEN** both middleware SHALL be appended to `group.Handlers` and all subsequent routes in this group SHALL include these middleware

### Requirement: Auto-detect handler signature
`GET/POST/PUT/DELETE/PATCH/OPTIONS/HEAD` methods SHALL accept `interface{}` as the handler parameter. At registration time, the system SHALL auto-detect the handler type:
- If `app.HandlerFunc` → wrap as `HandlerFuncTypeOfMethod` (direct call, no parameter parsing)
- If `method.Method` → use directly (preserving original `MethodType`)
- If other function type → wrap via `basic.NewMethod(nil, handler)` as `DefaultTypeOfMethod` (auto parameter parsing)
- Otherwise → panic with "unsupported handler type"

#### Scenario: Register HandlerFunc
- **WHEN** `group.GET("/ping", func(ctx context.Context, reqCtx *app.RequestContext) { ... })` is called
- **THEN** the handler SHALL be wrapped as `HandlerFuncTypeOfMethod` and dispatched via direct call

#### Scenario: Register struct method with auto parameter parsing
- **WHEN** `group.GET("/users/:id", userController.GetUser)` is called where `GetUser` has typed parameters
- **THEN** the handler SHALL be wrapped via `basic.NewMethod` as `DefaultTypeOfMethod` and dispatched with `setArgsOptimized` + `validArgs`

#### Scenario: Register standalone function with auto parameter parsing
- **WHEN** `group.GET("/users", func(req *GetUsersReq) (*GetUsersResp, error) { ... })` is called
- **THEN** the handler SHALL be wrapped via `basic.NewMethod` as `DefaultTypeOfMethod` and dispatched with auto parameter parsing

#### Scenario: Register method.Method directly
- **WHEN** `group.GET("/path", methodDesc)` is called where `methodDesc` is already a `method.Method`
- **THEN** it SHALL be used directly, preserving its original `MethodType`

### Requirement: GRPC method
`RouterGroup` SHALL provide a `GRPC(httpMethod string, path string, handler func(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error), srv interface{})` method that accepts an HTTP method name, a protoc-generated handler function, and a service instance. Internally it SHALL:
1. Extract the method name from the handler function name via `runtime.FuncForPC`
2. Derive `targetMethod` from `srv` via reflection
3. Construct `grpc.MethodDesc{MethodName, Handler}` and `method.NewGrpcTypeMethod(methodDesc, srv, targetMethod)`
4. Register the route with the specified HTTP method

The gRPC dispatch SHALL use `grpc.MethodDesc.Handler`, NOT reflection.

#### Scenario: Register gRPC route
- **WHEN** `group.GRPC("POST", "/search", _IService_Search_Handler, &HelloImpl{})` is called
- **THEN** the route SHALL be registered for POST method as `GrpcTypeOfMethod` and dispatched via `grpc.MethodDesc.Handler` (non-reflective)

### Requirement: Any method
`RouterGroup.Any()` SHALL register the route for all standard HTTP methods: GET, POST, PUT, PATCH, HEAD, OPTIONS, DELETE, CONNECT, TRACE.

#### Scenario: Register route for all methods
- **WHEN** `group.Any("/path", handler)` is called
- **THEN** the route SHALL be registered for all 9 HTTP methods with the same auto-detection logic

### Requirement: Handle method
`RouterGroup.Handle()` SHALL register a route with a custom HTTP method. It SHALL validate that the method name consists of uppercase letters only, panicking otherwise.

#### Scenario: Invalid method name
- **WHEN** `group.Handle("custom", "/path", handler)` is called
- **THEN** it SHALL panic with "http method custom is not valid"

### Requirement: EX method variants
`RouterGroup` SHALL provide `GETEX`, `POSTEX`, `PUTEX`, `DELETEEX`, `HEADEX`, `AnyEX`, `HandleEX` methods that accept an additional `handlerName` parameter and call `app.SetHandlerName` before registering the route.

### Requirement: BasePath method
`RouterGroup.BasePath()` SHALL return the base path of the router group.

### Requirement: abortIndex constant
The `combineHandlers` method SHALL define an `abortIndex` constant with value `math.MaxInt8 / 2`. If the combined handler count exceeds this value, it SHALL panic with "too many handlers".

### Requirement: returnObj method
`RouterGroup.returnObj()` SHALL return the `RouterGroup` itself regardless of the `root` flag, since `RouteRegistrar` does not implement `IRoutes`.
