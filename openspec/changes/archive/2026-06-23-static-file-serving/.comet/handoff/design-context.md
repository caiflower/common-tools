# Comet Design Handoff

- Change: static-file-serving
- Phase: design
- Mode: compact
- Context hash: 3653ffda8efc4dec47bfec0b7ff2e6277b83b26771266967eea0c89df75b7eac

Generated-by: comet-handoff.sh

OpenSpec remains the canonical capability spec. This handoff is a deterministic, source-traceable context pack, not an agent-authored summary.

## openspec/changes/static-file-serving/proposal.md

- Source: openspec/changes/static-file-serving/proposal.md
- Lines: 1-24
- SHA256: aa8befab658a8ec3a4f8f92026544e7801156739d77b0e900689fd4459fd12a2

```md
## Why

The `web` package currently lacks static file serving capabilities (StaticFile, Static, StaticFS), which are standard in web frameworks like Hertz. Adding these methods enables developers to serve static assets (HTML, CSS, JS, images) directly from the filesystem without custom handler boilerplate.

## What Changes

- Add `app.FS` struct for configuring static file serving behavior
- Add `RequestCtx.File(filepath)` method for serving a single file
- Add `StaticFile(relativePath, filepath string) IRoutes` to `IRoutes` / `RouterGroup` / `Engine`
- Add `Static(relativePath, root string) IRoutes` to `IRoutes` / `RouterGroup` / `Engine`
- Add `StaticFS(relativePath string, fs *app.FS) IRoutes` to `IRoutes` / `RouterGroup` / `Engine`

## Capabilities

### New Capabilities
- `static-file-serving`: Ability to serve static files from the local filesystem via Engine/router methods

### Modified Capabilities
<!-- None - new capability, no existing specs modified -->

## Impact

- Affected code: `web/app/context.go`, `web/app/fs.go` (new), `web/router/routergroup.go`, `web/engine.go`
- New test files: functional tests in `web/test/` covering StaticFile, Static, StaticFS
```

## openspec/changes/static-file-serving/design.md

- Source: openspec/changes/static-file-serving/design.md
- Lines: 1-49
- SHA256: 767fea9ef287599a920e05df612ad923b7384395df298d92ad43a1b489c100ff

```md
## Design

### Overview

Port Hertz-like static file serving to the `web` package. The implementation references Hertz's approach at `/Users/lijinlong/code/hertz/pkg/app/fs.go` and `/Users/lijinlong/code/hertz/pkg/route/routergroup.go`.

### Architecture

```
Engine
  └─ RouterGroup
       ├─ StaticFile(path, filepath)   → GET/HEAD handler → ctx.File(filepath)
       ├─ Static(path, root)           → StaticFS(path, &app.FS{Root: root})
       └─ StaticFS(path, fs)           → GET/HEAD handler → fs.NewRequestHandler()
```

### Key Components

**1. `app.FS` struct** (`web/app/fs.go` - new)
- `Root string` — root directory path
- `IndexNames []string` — index file names (e.g., index.html)
- `GenerateIndexPages bool` — auto-generate directory listing
- `PathNotFound app.HandlerFunc` — custom 404 handler
- `NewRequestHandler() app.HandlerFunc` — creates the static file handler

**2. `RequestCtx.File(filepath)` method** (`web/app/context.go`)
- Opens the file, detects content-type via `http.DetectContentType` or extension mapping
- Sets Content-Type, Content-Length headers
- Serves file content directly
- Handles HEAD requests (headers only)

**3. Router methods** (`web/router/routergroup.go`)
- `StaticFile`: registers GET+HEAD with a handler that calls `ctx.File()`
- `Static`: delegates to `StaticFS` with `&app.FS{Root: root}`
- `StaticFS`: registers GET+HEAD with `urlPattern = path.Join(relativePath, "/*filepath")`, uses `fs.NewRequestHandler()`

**4. Interface updates**
- `IRoutes` interface gains `StaticFile`, `Static`, `StaticFS`
- `Engine` delegates to `RouterGroup` (existing pattern)

### Design Decisions

1. **Simplified FS**: Unlike Hertz's 1200-line FS with caching/compression/byte-range, start with a clean functional FS that serves files correctly. Caching can be added later.

2. **Content-Type detection**: Use `mime.TypeByExtension` with fallback to `http.DetectContentType` for robust MIME detection.

3. **Security**: Join paths using `filepath.Clean` to prevent directory traversal. Panic on URL params in static paths (matching Hertz behavior).

4. **No new dependencies**: Use only standard library + existing web package internals.
```

## openspec/changes/static-file-serving/tasks.md

- Source: openspec/changes/static-file-serving/tasks.md
- Lines: 1-26
- SHA256: ce7ba9703de14736f11a733aad9cb12a7ff918be9beb5a62f86d1fc25072f695

```md
## Tasks

### Implementation
- [ ] Add `app.FS` struct and `NewRequestHandler()` in `web/app/fs.go`
- [ ] Add `RequestCtx.File(filepath)` method in `web/app/context.go`
- [ ] Add `StaticFile`, `Static`, `StaticFS` to `IRoutes` interface in `web/router/routergroup.go`
- [ ] Add `StaticFile`, `Static`, `StaticFS` implementations to `RouterGroup`
- [ ] Add `StaticFile`, `Static`, `StaticFS` delegation methods to `Engine`
- [ ] Wire up `StaticFS` handler with `/*filepath` URL pattern and `fs.NewRequestHandler()`

### Testing
- [ ] Add `TestStaticFile` — single file serving with correct content type and status
- [ ] Add `TestStaticFileNotFound` — 404 for missing file
- [ ] Add `TestStaticFileHEAD` — HEAD request returns headers only, no body
- [ ] Add `TestStaticFileWithGroup` — StaticFile on a RouterGroup with prefix
- [ ] Add `TestStatic` — directory serving with index.html detection
- [ ] Add `TestStaticDirectoryListing` — directory without index file
- [ ] Add `TestStaticFS` — custom FS with PathNotFound handler
- [ ] Add `TestStaticFSCustomIndexNames` — custom IndexNames
- [ ] Add `TestStaticPanicOnParams` — validate panic on URL params
- [ ] Add `TestStaticFileContentType` — verify correct Content-Type headers
- [ ] Add `TestStaticFileRange` — verify byte range support
- [ ] Add `TestStaticDirectoryTraversal` — security: prevent path traversal

### Verification
- [ ] Run `go test ./web/...` to verify no regression
```

## openspec/changes/static-file-serving/specs/static-file-serving/spec.md

- Source: openspec/changes/static-file-serving/specs/static-file-serving/spec.md
- Lines: 1-71
- SHA256: c9100b0f8cf7ad7dbc048ce809350520c5f8608342954b2a6832cbc3e378a65b

```md
# Static File Serving

## Overview

Provide `StaticFile`, `Static`, and `StaticFS` methods on the web Engine and RouterGroup for serving static files from the local filesystem.

## ADDED Requirements

### Requirement: StaticFile serves a single file
The `StaticFile(relativePath, filepath string) IRoutes` method MUST register GET and HEAD handlers that serve the specified local file with correct Content-Type and Content-Length headers.

#### Scenario: Serve existing file via GET
- **GIVEN** a file exists at `./testdata/hello.txt` containing "Hello, World!"
- **WHEN** `engine.StaticFile("/hello", "./testdata/hello.txt")` is called and GET `/hello` is requested
- **THEN** response status is 200, Content-Type is `text/plain; charset=utf-8`, body is "Hello, World!"

#### Scenario: HEAD request returns headers only
- **GIVEN** `engine.StaticFile("/hello", "./testdata/hello.txt")` is registered
- **WHEN** HEAD `/hello` is requested
- **THEN** response status is 200, Content-Type is set, Content-Length is set, body is empty

#### Scenario: File not found returns 404
- **GIVEN** `engine.StaticFile("/missing", "./testdata/nonexistent.txt")` is registered
- **WHEN** GET `/missing` is requested
- **THEN** response status is 404

### Requirement: Static serves directory contents
The `Static(relativePath, root string) IRoutes` method MUST serve files from a local directory, handling index files and directory access.

#### Scenario: Serve index.html from directory root
- **GIVEN** `./testdata/static/` contains `index.html` with `<h1>Index</h1>`
- **WHEN** `engine.Static("/static", "./testdata/static")` is called and GET `/static/` is requested
- **THEN** response status is 200, Content-Type is `text/html`, body contains `<h1>Index</h1>`

#### Scenario: Serve specific file from directory
- **GIVEN** `./testdata/static/` contains `style.css`
- **WHEN** GET `/static/style.css` is requested
- **THEN** response status is 200, Content-Type is `text/css`

### Requirement: StaticFS with custom configuration
The `StaticFS(relativePath string, fs *app.FS) IRoutes` method MUST allow custom FS configuration including IndexNames and PathNotFound handler.

#### Scenario: Custom PathNotFound handler
- **GIVEN** `StaticFS` is registered with a PathNotFound handler
- **WHEN** a non-existent file is requested
- **THEN** the PathNotFound handler is invoked

### Requirement: Security prevents path traversal
Static file serving MUST prevent directory traversal attacks by cleaning paths.

#### Scenario: Path traversal is blocked
- **GIVEN** `engine.Static("/static", "./testdata/static")` is registered
- **WHEN** GET `/static/../secret.txt` is requested
- **THEN** the request does not access files outside the root directory

### Requirement: URL parameters are rejected
Static methods MUST panic when `relativePath` contains `:` or `*` characters.

#### Scenario: Panic on URL parameter in path
- **GIVEN** a StaticFile call with `relativePath = "/file/:name"`
- **WHEN** the route is registered
- **THEN** the call panics

## ADDED Interface

The `IRoutes` interface gains three new methods:
```go
StaticFile(relativePath, filepath string) IRoutes
Static(relativePath, root string) IRoutes
StaticFS(relativePath string, fs *app.FS) IRoutes
```
```

