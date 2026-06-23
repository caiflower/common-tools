## Tasks

### Implementation
- [x] Add `app.FS` struct and `NewRequestHandler()` in `web/app/fs.go`
- [x] Add `RequestCtx.File(filepath)` method in `web/app/context.go`
- [x] Add `StaticFile`, `Static`, `StaticFS` to `IRoutes` interface in `web/router/routergroup.go`
- [x] Add `StaticFile`, `Static`, `StaticFS` implementations to `RouterGroup`
- [x] Add `StaticFile`, `Static`, `StaticFS` delegation methods to `Engine`
- [x] Wire up `StaticFS` handler with `/*filepath` URL pattern and `fs.NewRequestHandler()`

### Testing
- [x] Add `TestStaticFile` — single file serving with correct content type and status
- [x] Add `TestStaticFileNotFound` — 404 for missing file
- [x] Add `TestStaticFileHEAD` — HEAD request returns headers only, no body
- [x] Add `TestStaticFileWithGroup` — StaticFile on a RouterGroup with prefix
- [x] Add `TestStatic` — directory serving with index.html detection
- [x] Add `TestStaticDirectoryListing` — directory without index file
- [x] Add `TestStaticFS` — custom FS with PathNotFound handler
- [x] Add `TestStaticFSCustomIndexNames` — custom IndexNames
- [x] Add `TestStaticPanicOnParams` — validate panic on URL params
- [x] Add `TestStaticFileContentType` — verify correct Content-Type headers
- [x] Add `TestStaticFileRange` — verify byte range support
- [x] Add `TestStaticDirectoryTraversal` — security: prevent path traversal

### Verification
- [x] Run `go test ./web/...` to verify no regression
