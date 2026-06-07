## ADDED Requirements

### Requirement: RouteRegistrar interface definition
The system SHALL define a `RouteRegistrar` interface in the `router` package with a method `addRoute(httpMethod string, path string, handlers app.HandlersChain)`. This interface SHALL decouple `RouterGroup` from any concrete Engine type.

#### Scenario: Handler implements RouteRegistrar
- **WHEN** `Handler` is used as the `RouteRegistrar` for a `RouterGroup`
- **THEN** `Handler` SHALL implement the `addRoute` method that registers handlers into `trees MethodTrees`

#### Scenario: RouterGroup references RouteRegistrar
- **WHEN** a `RouterGroup` is created
- **THEN** its `engine` field SHALL be of type `RouteRegistrar`, not a concrete struct type

### Requirement: Handler.addRoute implementation
The `Handler` SHALL implement the `RouteRegistrar` interface. The `addRoute` method SHALL create a new `router` in `trees` for the given HTTP method if one does not exist, then call `router.addRoute` to register the path and handlers.

#### Scenario: Add route for new HTTP method
- **WHEN** `addRoute` is called with an HTTP method not yet in `trees`
- **THEN** a new `router` entry SHALL be created in `trees` and the route SHALL be registered

#### Scenario: Add route for existing HTTP method
- **WHEN** `addRoute` is called with an HTTP method already in `trees`
- **THEN** the route SHALL be registered in the existing `router` entry
