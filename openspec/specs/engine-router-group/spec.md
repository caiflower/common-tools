## ADDED Requirements

### Requirement: Engine holds RouterGroup instance
`web.Engine` SHALL hold a `*router.RouterGroup` instance that is initialized during engine creation with the `Handler` as the `RouteRegistrar`.

#### Scenario: Engine initialization creates RouterGroup
- **WHEN** `web.Default()` is called
- **THEN** the returned `Engine` SHALL have a non-nil `RouterGroup` with the `Handler` as its `RouteRegistrar`

### Requirement: Engine exposes Group method
`Engine` SHALL expose a `Group(relativePath string, handlers ...app.HandlerFunc) *router.RouterGroup` method that delegates to its `RouterGroup.Group()`.

#### Scenario: Create route group from Engine
- **WHEN** `engine.Group("/api")` is called
- **THEN** a new `RouterGroup` with basePath "/api" SHALL be returned

### Requirement: Engine exposes HTTP method shortcuts
`Engine` SHALL expose `GET`, `POST`, `PUT`, `DELETE`, `PATCH`, `OPTIONS`, `HEAD` methods that delegate to its `RouterGroup`.

#### Scenario: Register route directly on Engine
- **WHEN** `engine.GET("/ping", handler)` is called
- **THEN** a GET route for "/ping" SHALL be registered

### Requirement: Engine exposes Use method
`Engine` SHALL expose a `Use(middleware ...app.HandlerFunc) router.IRoutes` method that delegates to its `RouterGroup.Use()`.

#### Scenario: Add middleware via Engine
- **WHEN** `engine.Use(middleware)` is called
- **THEN** the middleware SHALL be added to the root RouterGroup and apply to all subsequent routes

### Requirement: Engine RouterGroup uses Handler as RouteRegistrar
The `RouterGroup` held by `Engine` SHALL use the `Handler` instance as its `RouteRegistrar`. This ensures routes registered via `RouterGroup` are stored in the same `trees` used by `Handler.Dispatch`.

#### Scenario: Routes registered via RouterGroup are dispatched by Handler
- **WHEN** a route is registered via `engine.GET("/test", handler)` and a request to "/test" is received
- **THEN** `Handler.Dispatch` SHALL find and execute the handler
