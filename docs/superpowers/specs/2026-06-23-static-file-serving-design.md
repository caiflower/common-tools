---
comet_change: static-file-serving
role: technical-design
canonical_spec: openspec
archived-with: 2026-06-23-static-file-serving
status: final
---

# Static File Serving — Technical Design

## Architecture

```
Engine.StaticFile("/favicon.ico", "./assets/favicon.ico")
  → RouterGroup.StaticFile → GET+HEAD handler → ctx.File(filepath)

Engine.Static("/static", "./public")
  → RouterGroup.Static → StaticFS(path, &app.FS{Root: root})

Engine.StaticFS("/assets", &app.FS{Root: "./cdn", IndexNames: []string{"index.html"}})
  → RouterGroup.StaticFS → GET+HEAD /*filepath handler → fs.NewRequestHandler()
```

## Component Details

### 1. `app.FS` (new: `web/app/fs.go`)

```go
type FS struct {
    Root               string
    IndexNames         []string
    GenerateIndexPages bool
    PathNotFound       HandlerFunc
}
```

- `NewRequestHandler() HandlerFunc` — creates a `ServeHTTP`-compatible handler that:
  1. Strips the route prefix from the URL
  2. Resolves the file path with `filepath.Clean` (anti-traversal)
  3. Serves file content with proper Content-Type
  4. On directory access: tries IndexNames, falls back to directory listing or PathNotFound

### 2. `RequestCtx.File(filepath)` (add to `web/app/context.go`)

```go
func (ctx *RequestContext) File(filepath string)
```

- Opens file, detects Content-Type via `mime.TypeByExtension` + `http.DetectContentType`
- Sets Content-Type, Content-Length headers
- For HEAD requests: headers only, no body

### 3. Router Methods (add to `web/router/routergroup.go`)

```go
func (group *RouterGroup) StaticFile(relativePath, filepath string) IRoutes
func (group *RouterGroup) Static(relativePath, root string) IRoutes
func (group *RouterGroup) StaticFS(relativePath string, fs *app.FS) IRoutes
```

### 4. Interface Update

`IRoutes` interface gets 3 new methods:
```go
StaticFile(string, string) IRoutes
Static(string, string) IRoutes
StaticFS(string, *app.FS) IRoutes
```

### 5. Engine Delegation

`Engine` adds delegation methods matching existing GET/POST/etc pattern.

## Security Considerations

- Path traversal prevention: `filepath.Clean` on resolved paths
- Panic on `:` or `*` in `relativePath` (matches Hertz behavior)
- No symlink following beyond root (enforced by `filepath.Clean`)

## Testing Strategy

Functional tests in `web/test/` using `httptest.NewServer` pattern:
- StaticFile: serve file, HEAD, 404, content-type
- Static: directory serving, index.html, directory listing
- StaticFS: custom config, PathNotFound, IndexNames
- Security: path traversal prevention
- Edge cases: empty root, non-existent paths, large files

## No Regression

Run `go test ./web/...` before and after changes to verify no existing tests break.
