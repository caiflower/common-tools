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

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/caiflower/common-tools/web/router"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newDiscoveryServer(handler http.HandlerFunc) *httptest.Server {
	return httptest.NewServer(handler)
}

func discoveryMetadata() Metadata {
	return Metadata{
		Name:      "myapp",
		Version:   "v1",
		Resources: []string{"users"},
		Routes: []router.RouteInfo{
			{
				Method:      "GET",
				Path:        "/users/:id",
				OperationID: "GetUser",
				Resource:    "users",
				Verb:        "get",
				Params: []router.ParamInfo{
					{Name: "id", Source: "path", Required: true},
				},
			},
		},
	}
}

func TestDiscoverFetchesAndCaches(t *testing.T) {
	var requests atomic.Int32
	server := newDiscoveryServer(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.Header().Set("ETag", `"v1-v1"`)
		_ = json.NewEncoder(w).Encode(discoveryMetadata())
	})
	defer server.Close()

	dir := t.TempDir()
	meta, err := Discover(context.Background(), server.URL, CacheOptions{TTL: time.Minute, Dir: dir})
	require.NoError(t, err)
	assert.Equal(t, "myapp", meta.Name)
	assert.Len(t, meta.Routes, 1)
	assert.Equal(t, int32(1), requests.Load())

	path, err := cacheFilePath(server.URL, dir)
	require.NoError(t, err)
	_, err = os.Stat(path)
	assert.NoError(t, err)
}

func TestDiscoverUsesFreshCache(t *testing.T) {
	var requests atomic.Int32
	server := newDiscoveryServer(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.Header().Set("ETag", `"v1-v1"`)
		_ = json.NewEncoder(w).Encode(discoveryMetadata())
	})
	defer server.Close()

	dir := t.TempDir()
	opts := CacheOptions{TTL: time.Minute, Dir: dir}
	_, err := Discover(context.Background(), server.URL, opts)
	require.NoError(t, err)
	_, err = Discover(context.Background(), server.URL, opts)
	require.NoError(t, err)
	assert.Equal(t, int32(1), requests.Load())
}

func TestDiscoverExpiredRefreshesWithNotModified(t *testing.T) {
	var requests atomic.Int32
	server := newDiscoveryServer(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if requests.Load() == 2 {
			assert.Equal(t, `"v1-v1"`, r.Header.Get("If-None-Match"))
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", `"v1-v1"`)
		_ = json.NewEncoder(w).Encode(discoveryMetadata())
	})
	defer server.Close()

	dir := t.TempDir()
	_, err := Discover(context.Background(), server.URL, CacheOptions{TTL: time.Minute, Dir: dir})
	require.NoError(t, err)

	path, err := cacheFilePath(server.URL, dir)
	require.NoError(t, err)
	oldCache, err := readCacheFile(path)
	require.NoError(t, err)
	err = writeCacheFile(path, oldCache.Metadata, time.Now().Add(-2*time.Minute))
	require.NoError(t, err)

	meta, err := Discover(context.Background(), server.URL, CacheOptions{TTL: time.Minute, Dir: dir})
	require.NoError(t, err)
	assert.Equal(t, "myapp", meta.Name)
	assert.Equal(t, int32(2), requests.Load())

	fresh, err := readCacheFile(path)
	require.NoError(t, err)
	assert.True(t, time.Since(fresh.FetchedAt) < time.Minute)
}

func TestDiscoverRefreshIgnoresFreshCache(t *testing.T) {
	var requests atomic.Int32
	server := newDiscoveryServer(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.Header().Set("ETag", `"v1-v1"`)
		_ = json.NewEncoder(w).Encode(discoveryMetadata())
	})
	defer server.Close()

	dir := t.TempDir()
	opts := CacheOptions{TTL: time.Minute, Dir: dir}
	_, err := Discover(context.Background(), server.URL, opts)
	require.NoError(t, err)
	opts.Refresh = true
	_, err = Discover(context.Background(), server.URL, opts)
	require.NoError(t, err)
	assert.Equal(t, int32(2), requests.Load())
}

func TestDiscoverFallsBackOnNetworkFailure(t *testing.T) {
	server := newDiscoveryServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"v1-v1"`)
		_ = json.NewEncoder(w).Encode(discoveryMetadata())
	})
	dir := t.TempDir()
	_, err := Discover(context.Background(), server.URL, CacheOptions{TTL: time.Minute, Dir: dir})
	require.NoError(t, err)

	path, err := cacheFilePath(server.URL, dir)
	require.NoError(t, err)
	oldCache, err := readCacheFile(path)
	require.NoError(t, err)
	err = writeCacheFile(path, oldCache.Metadata, time.Now().Add(-2*time.Minute))
	require.NoError(t, err)

	server.Close()
	var warnings bytes.Buffer
	meta, err := Discover(context.Background(), server.URL, CacheOptions{TTL: time.Minute, Dir: dir, Stderr: &warnings})
	require.NoError(t, err)
	assert.Equal(t, "myapp", meta.Name)
	assert.Contains(t, warnings.String(), "using cached metadata")
}

func TestDiscoverWithoutCacheFails(t *testing.T) {
	server := newDiscoveryServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	server.Close()

	_, err := Discover(context.Background(), server.URL, CacheOptions{TTL: time.Minute, Dir: t.TempDir()})
	assert.Error(t, err)
}

func TestCacheFilePathUsesServerHash(t *testing.T) {
	dir := t.TempDir()
	path, err := cacheFilePath("http://127.0.0.1:8080", dir)
	require.NoError(t, err)
	assert.True(t, filepath.IsAbs(path))
	assert.True(t, filepath.HasPrefix(path, dir))
}

func TestRoutesCommandRemote(t *testing.T) {
	server := newDiscoveryServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"v1-v1"`)
		_ = json.NewEncoder(w).Encode(discoveryMetadata())
	})
	defer server.Close()

	engine := newCLIEngine()
	root := New(engine, WithName("myapp"))
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	root.SetArgs([]string{"routes", "--server", server.URL, "--cache-dir", t.TempDir()})
	assert.NoError(t, root.Execute())
}

func TestRemoteServerGeneratesCommands(t *testing.T) {
	server := newDiscoveryServer(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/cli/routes":
			w.Header().Set("ETag", `"v1-v1"`)
			_ = json.NewEncoder(w).Encode(discoveryMetadata())
		case "/users/1":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"requestID":"r1","data":{"ok":true}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
	defer server.Close()

	// The local engine intentionally has no routes: the command must come
	// from the remote /cli/routes metadata.
	engine := newCLIEngine()
	var out bytes.Buffer
	args := []string{"get", "users", "--id=1", "--server", server.URL, "--output=json", "--cache-dir", t.TempDir()}
	root := NewWithArgs(engine, args, WithName("myapp"))
	root.SetArgs(args)
	root.SetOut(&out)
	root.SetErr(io.Discard)
	assert.NoError(t, root.Execute())
	assert.Contains(t, out.String(), `"data":{"ok":true}`)
}

func TestRemoteDiscoveryFallsBackToLocal(t *testing.T) {
	engine := newCLIEngine()
	engine.GET("/users/:id", cliCmdGet)
	var warnings bytes.Buffer
	root := NewWithArgs(
		engine,
		[]string{"get", "users", "--server", "http://127.0.0.1:1", "--cache-dir", t.TempDir()},
		WithName("myapp"),
		WithStderr(&warnings),
	)

	cmd, _, err := root.Find([]string{"get", "users"})
	assert.NoError(t, err)
	assert.NotNil(t, cmd)
	assert.Contains(t, warnings.String(), "falling back to local route metadata")
}
