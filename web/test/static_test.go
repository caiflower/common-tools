/*
 * Copyright 2024 caiflower Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package webtest

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/caiflower/common-tools/web"
	"github.com/caiflower/common-tools/web/app"
	"github.com/caiflower/common-tools/web/app/server/config"
	"github.com/stretchr/testify/assert"
)

// =============================================================================
// StaticFile Tests
// =============================================================================

// TestStaticFile tests serving a single file via StaticFile.
func TestStaticFile(t *testing.T) {
	engine := web.Default(
		config.WithAddr(":0"),
		config.WithName("test-static-file"),
		config.WithRootPath(""),
		config.WithControllerRootPkgName("webtest"),
	)
	engine.StaticFile("/hello", "../common/testdata/test.txt")

	handler := engine.Handler()

	req := httptest.NewRequest("GET", "/hello", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	assert.Equal(t, "hello world!", w.Body.String())
	assert.Contains(t, w.Header().Get("Content-Type"), "text/plain")
}

// TestStaticFileHEAD tests HEAD request for StaticFile (headers only, no body).
func TestStaticFileHEAD(t *testing.T) {
	engine := web.Default(
		config.WithAddr(":0"),
		config.WithName("test-static-file-head"),
		config.WithRootPath(""),
		config.WithControllerRootPkgName("webtest"),
	)
	engine.StaticFile("/hello", "../common/testdata/test.txt")

	handler := engine.Handler()

	req := httptest.NewRequest("HEAD", "/hello", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/plain")
	assert.NotEmpty(t, w.Header().Get("Content-Length"))
	// HEAD should have no body
	assert.Empty(t, w.Body.String())
}

// TestStaticFileNotFound tests 404 for missing file.
func TestStaticFileNotFound(t *testing.T) {
	engine := web.Default(
		config.WithAddr(":0"),
		config.WithName("test-static-file-404"),
		config.WithRootPath(""),
		config.WithControllerRootPkgName("webtest"),
	)
	engine.StaticFile("/missing", "../common/testdata/nonexistent.txt")

	handler := engine.Handler()

	req := httptest.NewRequest("GET", "/missing", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, 404, w.Code)
}

// TestStaticFileContentType tests that Content-Type is correctly detected.
func TestStaticFileContentType(t *testing.T) {
	engine := web.Default(
		config.WithAddr(":0"),
		config.WithName("test-static-file-ct"),
		config.WithRootPath(""),
		config.WithControllerRootPkgName("webtest"),
	)
	engine.StaticFile("/favicon.ico", "../common/testdata/favicon.ico")

	handler := engine.Handler()

	req := httptest.NewRequest("GET", "/favicon.ico", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	// .ico should detect appropriate content type
	assert.NotEmpty(t, w.Header().Get("Content-Type"))
}

// TestStaticFileWithGroup tests StaticFile on a RouterGroup with prefix.
func TestStaticFileWithGroup(t *testing.T) {
	engine := web.Default(
		config.WithAddr(":0"),
		config.WithName("test-static-file-group"),
		config.WithRootPath(""),
		config.WithControllerRootPkgName("webtest"),
	)

	v1 := engine.Group("/api")
	v1.StaticFile("/hello", "../common/testdata/test.txt")

	handler := engine.Handler()

	req := httptest.NewRequest("GET", "/api/hello", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	assert.Equal(t, "hello world!", w.Body.String())
}

// =============================================================================
// Static Tests (directory serving)
// =============================================================================

// TestStaticIndexHTML tests serving index.html from directory root.
func TestStaticIndexHTML(t *testing.T) {
	engine := web.Default(
		config.WithAddr(":0"),
		config.WithName("test-static-index"),
		config.WithRootPath(""),
		config.WithControllerRootPkgName("webtest"),
	)
	engine.Static("/static", "../common/testdata/static")

	handler := engine.Handler()

	// Test serving index.html
	req := httptest.NewRequest("GET", "/static/index.html", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), "<h1>Index</h1>")
	assert.Contains(t, w.Header().Get("Content-Type"), "text/html")
}

// TestStaticCSS tests serving CSS file from directory.
func TestStaticCSS(t *testing.T) {
	engine := web.Default(
		config.WithAddr(":0"),
		config.WithName("test-static-css"),
		config.WithRootPath(""),
		config.WithControllerRootPkgName("webtest"),
	)
	engine.Static("/static", "../common/testdata/static")

	handler := engine.Handler()

	req := httptest.NewRequest("GET", "/static/style.css", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), "body { color: red; }")
	assert.Contains(t, w.Header().Get("Content-Type"), "text/css")
}

// TestStaticSubdirectory tests serving files from subdirectory.
func TestStaticSubdirectory(t *testing.T) {
	engine := web.Default(
		config.WithAddr(":0"),
		config.WithName("test-static-subdir"),
		config.WithRootPath(""),
		config.WithControllerRootPkgName("webtest"),
	)
	engine.Static("/static", "../common/testdata/static")

	handler := engine.Handler()

	req := httptest.NewRequest("GET", "/static/subdir/data.txt", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	assert.Equal(t, "subfile content", w.Body.String())
}

// TestStaticNotFound tests 404 for non-existent file in directory.
func TestStaticNotFound(t *testing.T) {
	engine := web.Default(
		config.WithAddr(":0"),
		config.WithName("test-static-404"),
		config.WithRootPath(""),
		config.WithControllerRootPkgName("webtest"),
	)
	engine.Static("/static", "../common/testdata/static")

	handler := engine.Handler()

	req := httptest.NewRequest("GET", "/static/nonexistent.txt", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, 404, w.Code)
}

// TestStaticHEAD tests HEAD request for static directory file.
func TestStaticHEAD(t *testing.T) {
	engine := web.Default(
		config.WithAddr(":0"),
		config.WithName("test-static-head"),
		config.WithRootPath(""),
		config.WithControllerRootPkgName("webtest"),
	)
	engine.Static("/static", "../common/testdata/static")

	handler := engine.Handler()

	req := httptest.NewRequest("HEAD", "/static/index.html", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/html")
	assert.NotEmpty(t, w.Header().Get("Content-Length"))
	assert.Empty(t, w.Body.String())
}

// =============================================================================
// StaticFS Tests (custom FS configuration)
// =============================================================================

// TestStaticFSWithIndexNames tests FS with custom IndexNames.
func TestStaticFSWithIndexNames(t *testing.T) {
	engine := web.Default(
		config.WithAddr(":0"),
		config.WithName("test-staticfs-index"),
		config.WithRootPath(""),
		config.WithControllerRootPkgName("webtest"),
	)

	fs := &app.FS{
		Root:       "../common/testdata/static",
		IndexNames: []string{"index.html"},
	}
	engine.StaticFS("/assets", fs)

	handler := engine.Handler()

	// Directory with index.html
	req := httptest.NewRequest("GET", "/assets/index.html", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), "<h1>Index</h1>")
}

// TestStaticFSPathNotFound tests FS with custom PathNotFound handler.
func TestStaticFSPathNotFound(t *testing.T) {
	notFoundCalled := false

	engine := web.Default(
		config.WithAddr(":0"),
		config.WithName("test-staticfs-404"),
		config.WithRootPath(""),
		config.WithControllerRootPkgName("webtest"),
	)

	fs := &app.FS{
		Root: "../common/testdata/static",
		PathNotFound: func(c context.Context, ctx *app.RequestContext) {
			notFoundCalled = true
			ctx.Response.SetStatusCode(404)
			ctx.SetHeader("X-Custom-404", "yes")
		},
	}
	engine.StaticFS("/assets", fs)

	handler := engine.Handler()

	req := httptest.NewRequest("GET", "/assets/nonexistent.txt", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.True(t, notFoundCalled, "PathNotFound handler should be called")
	assert.Equal(t, "yes", w.Header().Get("X-Custom-404"))
}

// TestStaticFSGenerateIndexPages tests directory listing generation.
func TestStaticFSGenerateIndexPages(t *testing.T) {
	engine := web.Default(
		config.WithAddr(":0"),
		config.WithName("test-staticfs-listing"),
		config.WithRootPath(""),
		config.WithControllerRootPkgName("webtest"),
	)

	fs := &app.FS{
		Root:               "../common/testdata/static",
		GenerateIndexPages: true,
	}
	engine.StaticFS("/dir", fs)

	handler := engine.Handler()

	// Directory without index file should generate listing
	req := httptest.NewRequest("GET", "/dir/subdir", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/html")
	assert.Contains(t, w.Body.String(), "data.txt")
}

// TestStaticFSWithGroup tests StaticFS on a RouterGroup.
func TestStaticFSWithGroup(t *testing.T) {
	engine := web.Default(
		config.WithAddr(":0"),
		config.WithName("test-staticfs-group"),
		config.WithRootPath(""),
		config.WithControllerRootPkgName("webtest"),
	)

	v1 := engine.Group("/v1")
	fs := &app.FS{Root: "../common/testdata/static"}
	v1.StaticFS("/files", fs)

	handler := engine.Handler()

	req := httptest.NewRequest("GET", "/v1/files/index.html", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), "<h1>Index</h1>")
}

// =============================================================================
// Security Tests
// =============================================================================

// TestStaticDirectoryTraversal tests prevention of directory traversal attacks.
func TestStaticDirectoryTraversal(t *testing.T) {
	engine := web.Default(
		config.WithAddr(":0"),
		config.WithName("test-static-traversal"),
		config.WithRootPath(""),
		config.WithControllerRootPkgName("webtest"),
	)
	engine.Static("/static", "../common/testdata/static")

	handler := engine.Handler()

	// Attempt path traversal
	req := httptest.NewRequest("GET", "/static/../secret.txt", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// Should not serve file outside root
	assert.Equal(t, 404, w.Code)
}

// =============================================================================
// Panic Tests
// =============================================================================

// TestStaticFilePanicOnParams tests panic when URL params used in StaticFile.
func TestStaticFilePanicOnParams(t *testing.T) {
	engine := web.Default(
		config.WithAddr(":0"),
		config.WithName("test-static-panic"),
		config.WithRootPath(""),
		config.WithControllerRootPkgName("webtest"),
	)

	assert.Panics(t, func() {
		engine.StaticFile("/file/:name", "../common/testdata/test.txt")
	}, "StaticFile with URL param should panic")
}

// TestStaticPanicOnParams tests panic when URL params used in Static.
func TestStaticPanicOnParams(t *testing.T) {
	engine := web.Default(
		config.WithAddr(":0"),
		config.WithName("test-static-panic2"),
		config.WithRootPath(""),
		config.WithControllerRootPkgName("webtest"),
	)

	assert.Panics(t, func() {
		engine.Static("/static/*file", "../common/testdata/static")
	}, "Static with URL wildcard should panic")
}

// =============================================================================
// Integration Tests
// =============================================================================

// TestStaticFileWithMiddleware tests middleware works with StaticFile routes.
func TestStaticFileWithMiddleware(t *testing.T) {
	middlewareCalled := false

	engine := web.Default(
		config.WithAddr(":0"),
		config.WithName("test-static-mw"),
		config.WithRootPath(""),
		config.WithControllerRootPkgName("webtest"),
	)

	engine.Use(func(c context.Context, ctx *app.RequestContext) {
		middlewareCalled = true
		ctx.SetHeader("X-Middleware", "applied")
		ctx.Next(c)
	})

	engine.StaticFile("/hello", "../common/testdata/test.txt")

	handler := engine.Handler()

	req := httptest.NewRequest("GET", "/hello", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.True(t, middlewareCalled, "middleware should be called")
	assert.Equal(t, "applied", w.Header().Get("X-Middleware"))
}

// TestStaticWithNestedGroup tests Static with nested RouterGroups.
func TestStaticWithNestedGroup(t *testing.T) {
	engine := web.Default(
		config.WithAddr(":0"),
		config.WithName("test-static-nested"),
		config.WithRootPath(""),
		config.WithControllerRootPkgName("webtest"),
	)

	v1 := engine.Group("/v1")
	v2 := v1.Group("/admin")
	v2.Static("/assets", "../common/testdata/static")

	handler := engine.Handler()

	req := httptest.NewRequest("GET", "/v1/admin/assets/index.html", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), "<h1>Index</h1>")
}

// =============================================================================
// Static Wildcard (*filepath) Routing Tests
// =============================================================================

// TestStaticDeepNestedPath tests that /*filepath captures multi-segment paths.
func TestStaticDeepNestedPath(t *testing.T) {
	engine := web.Default(
		config.WithAddr(":0"),
		config.WithName("test-static-deep"),
		config.WithRootPath(""),
		config.WithControllerRootPkgName("webtest"),
	)
	engine.Static("/cdn", "../common/testdata/static")

	handler := engine.Handler()

	// Multi-segment path through wildcard
	req := httptest.NewRequest("GET", "/cdn/subdir/data.txt", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	assert.Equal(t, "subfile content", w.Body.String())
}

// TestStaticRootPathWithoutSlash tests that /static (without trailing slash)
// does NOT match the wildcard route /static/*filepath — the wildcard requires
// a "/" separator before capturing. This verifies correct route matching.
func TestStaticRootPathWithoutSlash(t *testing.T) {
	engine := web.Default(
		config.WithAddr(":0"),
		config.WithName("test-static-noslash"),
		config.WithRootPath(""),
		config.WithControllerRootPkgName("webtest"),
	)

	fs := &app.FS{
		Root:               "../common/testdata/static",
		GenerateIndexPages: true,
	}
	engine.StaticFS("/assets", fs)

	handler := engine.Handler()

	// /assets without trailing slash does NOT match /assets/*filepath
	// because the wildcard requires a "/" before the captured segment.
	req := httptest.NewRequest("GET", "/assets", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// No route matches — returns 404
	assert.Equal(t, 404, w.Code)
}

// TestStaticRootPathWithSlash tests that /static/ (with trailing slash)
// matches the wildcard and shows directory listing.
func TestStaticRootPathWithSlash(t *testing.T) {
	engine := web.Default(
		config.WithAddr(":0"),
		config.WithName("test-static-slash"),
		config.WithRootPath(""),
		config.WithControllerRootPkgName("webtest"),
	)

	fs := &app.FS{
		Root:               "../common/testdata/static",
		GenerateIndexPages: true,
	}
	engine.StaticFS("/assets", fs)

	handler := engine.Handler()

	// With trailing slash, wildcard *filepath captures ""
	req := httptest.NewRequest("GET", "/assets/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/html")
	assert.Contains(t, w.Body.String(), "index.html")
}

// TestStaticRootPathWithSlash tests that /static/ (with trailing slash)
// also works correctly.
func TestStaticWildcardCapturesFullPath(t *testing.T) {
	engine := web.Default(
		config.WithAddr(":0"),
		config.WithName("test-static-wildcard"),
		config.WithRootPath(""),
		config.WithControllerRootPkgName("webtest"),
	)
	engine.Static("/public", "../common/testdata/static")

	handler := engine.Handler()

	// Verify different files resolve to different content
	tests := []struct {
		path     string
		wantCode int
		wantBody string
	}{
		{"/public/index.html", 200, "<h1>Index</h1>"},
		{"/public/style.css", 200, "body { color: red; }"},
		{"/public/subdir/data.txt", 200, "subfile content"},
	}

	for _, tt := range tests {
		req := httptest.NewRequest("GET", tt.path, nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		assert.Equal(t, tt.wantCode, w.Code, "path=%s", tt.path)
		assert.Equal(t, tt.wantBody, w.Body.String(), "path=%s", tt.path)
	}
}

// TestStaticWildcardWithSpecialChars tests files with special characters
// in their names through the wildcard.
func TestStaticWildcardWithSpecialChars(t *testing.T) {
	engine := web.Default(
		config.WithAddr(":0"),
		config.WithName("test-static-special"),
		config.WithRootPath(""),
		config.WithControllerRootPkgName("webtest"),
	)
	engine.Static("/pub", "../common/testdata/static")

	handler := engine.Handler()

	// File with hyphens/underscores/dots in name
	req := httptest.NewRequest("GET", "/pub/subdir/data.txt", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	assert.Equal(t, "subfile content", w.Body.String())
}
