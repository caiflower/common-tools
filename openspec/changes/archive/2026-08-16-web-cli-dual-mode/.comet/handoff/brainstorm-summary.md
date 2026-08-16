# Brainstorm Summary

- Change: web-cli-dual-mode
- Date: 2026-08-08

## Confirmed Technical Approach

- Same binary dual mode: `serve` starts the HTTP server through a resource manager; API commands use cobra dynamic commands and call a running server over HTTP.
- `serve` registers the engine as a daemon on a minimal `ResourceManager` interface and calls `Signal()`. Default manager is `global.DefaultResourceManger`; custom manager supported via option.
- Route metadata: new exported `router.RouteInfo`/`ParamInfo`, automatically recorded by `Handler.addRoute`; `Handler.Routes()` provides read-only enumeration.
- resource/verb derivation: explicit override (`engine.CLIRoute`) > handler method name > path fallback; HTTP method mapping to get/create/update/patch/delete/call; ambiguous routes also get `call <operationID>`.
- `/cli/routes`: enabled via `WithEnableCLI` (default off), returns name/version/resources/routes, ETag/304 support.
- `web/cli` package: cobra root command with `serve`, `routes`, and dynamically generated commands; `--server` switches to remote discovery; local mode uses `engine.Routes()`.
- Cache: server-address-hash keyed file, default TTL 5 minutes, `If-None-Match` refresh, `--refresh` force refresh, stale fallback with warning.
- Params/output: path/query/header flags; body via `--data`/`-f`; output table/json/yaml; GET/HEAD json-tag fields map to query.

## Key Trade-offs and Risks

- Anonymous handlers cannot infer resource from method name; path fallback plus `call` command keeps every route reachable.
- Local CLI mode requires a running server; clear error when unreachable.
- Automatic derivation may not match business semantics; explicit override API fixes it.
- `/cli/routes` exposes metadata; default off, can be network/auth restricted.
- Dynamic commands make help depend on local metadata or cache; no cache plus unreachable remote gives a clear error.

## Testing Strategy

- Unit tests for metadata extraction and derivation (normal func, struct method, gRPC, anonymous HandlerFunc, explicit override).
- Endpoint tests for `/cli/routes` enable/disable, content, 304.
- CLI tests for command generation, request construction, output formats, resource manager integration, and local-server-unreachable error.
- Remote cache tests: cache hit, force refresh, 304, stale fallback.
- Final regression: `go test ./web/...`.

## Spec Patches

- Added `serve through resource manager` scenario to web-cli spec.
- Added `local mode without running server` scenario to web-cli spec.
