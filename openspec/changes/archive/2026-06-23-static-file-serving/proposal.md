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
