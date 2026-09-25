## ADDED Requirements

### Requirement: Route metadata registry
The system SHALL record structured metadata for every route registered through `RouterGroup` (including `GET/POST/PUT/PATCH/DELETE/OPTIONS/HEAD/Any/Handle/GRPC/Static*`). Each record SHALL contain HTTP method, absolute path, operationID, resource, verb, and parameter list. Parameter list SHALL distinguish `path`, `query`, `header`, and `body` sources, and SHALL include field name, type, and validation tag when available.

#### Scenario: Record normal route
- **WHEN** `engine.GET("/users/:id", uc.GetUser)` is registered
- **THEN** the metadata registry SHALL contain a record with method `GET`, path `/users/:id`, and parameters derived from `GetUser`'s request struct

#### Scenario: Record GRPC route
- **WHEN** `engine.GRPC("POST", "/v1/search", handler, srv)` is registered
- **THEN** the metadata registry SHALL contain a record with method `POST`, path `/v1/search`, and operationID from the gRPC method name

### Requirement: Route enumeration API
The `router.Handler` SHALL expose a read-only `Routes()` API returning all registered route metadata. It SHALL be safe for concurrent access after registration.

#### Scenario: Enumerate registered routes
- **WHEN** a handler has multiple routes registered
- **THEN** `Routes()` SHALL return metadata for all of them without modifying the route tree

### Requirement: Resource and verb derivation
The system SHALL derive `resource` and `verb` with priority: explicit override, then handler method name, then path fallback. HTTP method mapping SHALL be `GET->get`, `POST->create`, `PUT->update`, `PATCH->patch`, `DELETE->delete`, and any other method to `call`.

#### Scenario: Derive from method name
- **WHEN** a handler method is named `GetUser`
- **THEN** its metadata SHALL use verb `get` and resource `user`

#### Scenario: Fallback to path
- **WHEN** a route uses an anonymous handler and no explicit override
- **THEN** its metadata SHALL use the last static path segment as resource and the HTTP-method-derived verb

#### Scenario: Explicit override wins
- **WHEN** an explicit override assigns resource `users` and verb `get` to a route
- **THEN** the metadata SHALL use `users`/`get` regardless of method name or path

### Requirement: CLI discovery endpoint
The system SHALL expose a `GET /cli/routes` endpoint when CLI metadata is enabled via configuration. The endpoint SHALL return JSON containing server name, metadata version, resource list, and route list. The endpoint SHALL support ETag and return `304 Not Modified` when `If-None-Match` matches the current version.

#### Scenario: Endpoint disabled by default
- **WHEN** CLI metadata is not enabled
- **THEN** `GET /cli/routes` SHALL NOT be served by the framework

#### Scenario: Endpoint enabled
- **WHEN** CLI metadata is enabled and a request hits `GET /cli/routes`
- **THEN** the response SHALL be JSON with non-empty route list reflecting registered routes

#### Scenario: Conditional request
- **WHEN** a request includes `If-None-Match` equal to the current ETag
- **THEN** the server SHALL respond with status `304` and no body

### Requirement: Explicit metadata override
The system SHALL provide an explicit registration API to assign `resource` and `verb` to a registered route without changing existing route registration methods.

#### Scenario: Override a route
- **WHEN** a route is registered and then assigned resource `orders` with verb `list`
- **THEN** generated metadata and `/cli/routes` SHALL use `orders`/`list`
