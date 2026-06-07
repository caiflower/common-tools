## MODIFIED Requirements

### Requirement: Middleware chain execution via Next()
The handler chain dispatch SHALL use `ctx.Next()` to drive execution of the entire handler chain (middleware + final handler), replacing the previous sequential loop execution in `getTargetMethod`.

#### Scenario: Full chain execution via Next()
- **WHEN** a route is matched and handlers chain is found in the tree
- **THEN** the handlers chain SHALL be stored on the RequestContext and `ctx.Next(ctx)` SHALL be called to start execution from the first handler

#### Scenario: Middleware without Next() behaves as before
- **WHEN** a middleware does NOT call `ctx.Next()` and simply returns
- **THEN** subsequent handlers in the chain SHALL NOT execute (same as current behavior where non-Next middleware blocks the chain)

#### Scenario: Middleware with Next() allows post-handler logic
- **WHEN** a middleware calls `ctx.Next()` and then continues executing code after the call
- **THEN** the code after `ctx.Next()` SHALL execute after all subsequent handlers complete

## ADDED Requirements

### Requirement: getTargetMethod no longer executes middleware
`getTargetMethod` SHALL only locate and store the handler chain on the RequestContext without executing any middleware. Execution is driven by `Next()` from the dispatch layer.

#### Scenario: getTargetMethod stores chain only
- **WHEN** `getTargetMethod` finds a matching route with handlers
- **THEN** it SHALL store the full handlers chain on the RequestContext and set handler index to 0, without executing any middleware
