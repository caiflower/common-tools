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
