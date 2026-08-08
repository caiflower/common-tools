## ADDED Requirements

### Requirement: Dual-mode command entry
The CLI package SHALL provide a cobra root command that supports a `serve` subcommand and dynamically generated API commands. `serve` SHALL start the same engine's HTTP server; API commands SHALL send HTTP requests to a running server.

#### Scenario: Start server
- **WHEN** user runs `<binary> serve`
- **THEN** the engine SHALL be registered as a daemon resource on the resource manager and `Signal()` SHALL start it with its configured address

#### Scenario: Serve through resource manager
- **WHEN** a resource manager is configured on the CLI
- **THEN** `serve` SHALL use that manager for start/stop instead of calling `engine.Start()` directly

#### Scenario: Invoke local API
- **WHEN** user runs `<binary> get users --id=1` without `--server`
- **THEN** the CLI SHALL build the command from local route metadata and send HTTP request to the engine's configured local address

#### Scenario: Local mode without running server
- **WHEN** user runs a local CLI command without `--server` and the local server is not reachable
- **THEN** the CLI SHALL print a clear error explaining that the server must be started with `serve` first

### Requirement: Dynamic command generation from local metadata
The CLI SHALL generate commands from `engine.Routes()` at startup. Each route SHALL become a `verb resource` command with flags for path/query/header parameters and body input flags.

#### Scenario: Generate resource command
- **WHEN** local metadata contains route `GET /users/:id` with verb `get` and resource `users`
- **THEN** the CLI SHALL expose command `get users` with an `id` flag

#### Scenario: Fallback call command
- **WHEN** a route has conflicting or ambiguous resource/verb derivation
- **THEN** the CLI SHALL still expose a `call <operationID>` command for that route

### Requirement: Remote discovery
When `--server` is specified, the CLI SHALL fetch route metadata from `GET /cli/routes` on that server and build commands from the fetched metadata instead of local metadata.

#### Scenario: Discover remote routes
- **WHEN** user runs `<binary> --server http://example.com get users`
- **THEN** the CLI SHALL fetch `/cli/routes` from that server and generate the `get users` command from the response

### Requirement: Metadata cache and refresh
The CLI SHALL cache remote metadata locally. Cache SHALL be keyed by server address, have a TTL (default 5 minutes), support `--refresh` force refresh, and send `If-None-Match` using the cached version. When remote fetch fails and a cache exists, the CLI SHALL use the stale cache and print a warning.

#### Scenario: Cache hit
- **WHEN** remote metadata was fetched within TTL
- **THEN** the CLI SHALL reuse the cache without requesting `/cli/routes`

#### Scenario: Forced refresh
- **WHEN** user runs `routes --refresh`
- **THEN** the CLI SHALL ignore TTL and re-fetch remote metadata

#### Scenario: Conditional refresh
- **WHEN** cache is stale and the server returns `304`
- **THEN** the CLI SHALL keep the cached metadata and refresh the cache timestamp

#### Scenario: Stale fallback
- **WHEN** remote fetch fails and a cache file exists
- **THEN** the CLI SHALL execute using the cached metadata and output a warning

### Requirement: Request parameter binding
The CLI SHALL bind path parameters into the URL template, query parameters into the query string, header parameters into request headers, and body parameters from `--data` or `-f`. For GET/HEAD routes, `json` tag fields SHALL be treated as query parameters; for other methods they SHALL be treated as body.

#### Scenario: Path and query parameters
- **WHEN** command `get users --id=1 --status=active` maps to `GET /users/:id`
- **THEN** the request URL SHALL be `/users/1?status=active`

#### Scenario: Header parameter
- **WHEN** a parameter has header source
- **THEN** its flag value SHALL be set as a request header

#### Scenario: Body from file
- **WHEN** user passes `-f request.json`
- **THEN** the file content SHALL be sent as JSON request body

### Requirement: Output formats
The CLI SHALL support `table`, `json`, and `yaml` output. Default SHALL be `table`, selected by `--output`.

#### Scenario: JSON output
- **WHEN** user runs command with `--output json`
- **THEN** the CLI SHALL print the raw unified JSON response

#### Scenario: YAML output
- **WHEN** user runs command with `--output yaml`
- **THEN** the CLI SHALL print the response converted to YAML

### Requirement: Header and token passthrough
The CLI SHALL support global `--token` and repeatable `--header key=value` flags, attaching them to every outgoing request.

#### Scenario: Token header
- **WHEN** user passes `--token abc`
- **THEN** every request SHALL include the configured auth header with value `abc`

#### Scenario: Custom header
- **WHEN** user passes `--header X-Trace=123`
- **THEN** the request SHALL include header `X-Trace: 123`
