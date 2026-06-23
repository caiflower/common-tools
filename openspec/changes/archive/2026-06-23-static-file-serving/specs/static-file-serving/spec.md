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
