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
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/caiflower/common-tools/web/router"
	"github.com/stretchr/testify/assert"
)

func TestClientExecuteBuildsRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/users/42/orders/a/b", r.URL.Path)
		assert.Equal(t, "name=alice", r.URL.RawQuery)
		assert.Equal(t, "Bearer secret", r.Header.Get("Authorization"))
		assert.Equal(t, "v", r.Header.Get("X-Extra"))
		assert.Equal(t, "abc", r.Header.Get("X-Token"))
		assert.Equal(t, `{"payload":"hello"}`, readBody(r))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"requestID":"r1","data":{"ok":true}}`))
	}))
	defer server.Close()

	route := router.RouteInfo{
		Method: "POST",
		Path:   "/users/:id/orders/*rest",
		Params: []router.ParamInfo{
			{Name: "id", Source: "path"},
			{Name: "rest", Source: "path"},
			{Name: "name", Source: "query"},
			{Name: "X-Token", Source: "header"},
			{Name: "payload", Source: "body"},
		},
	}
	client := &Client{
		Server:  server.URL,
		Token:   "secret",
		Headers: []string{"X-Extra=v"},
	}
	body, err := client.Execute(context.Background(), route, map[string]string{
		"id":      "42",
		"rest":    "a/b",
		"name":    "alice",
		"X-Token": "abc",
	}, []byte(`{"payload":"hello"}`))
	assert.NoError(t, err)
	assert.Equal(t, `{"requestID":"r1","data":{"ok":true}}`, string(body))
}

func TestClientExecuteReturnsErrorOnBadStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"code":400,"message":"bad request"}}`))
	}))
	defer server.Close()

	client := &Client{Server: server.URL}
	_, err := client.Execute(context.Background(), router.RouteInfo{Method: "GET", Path: "/users"}, nil, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "400")
	assert.Contains(t, err.Error(), "bad request")
}

func TestRequestURLReplacesPathParamsWithoutPrefixCollision(t *testing.T) {
	client := &Client{Server: "http://example.com"}
	route := router.RouteInfo{
		Path: "/users/:userID/orders/:user",
		Params: []router.ParamInfo{
			{Name: "userID", Source: "path"},
			{Name: "user", Source: "path"},
		},
	}
	url := client.requestURL(route, map[string]string{"userID": "abc", "user": "def"})
	assert.Equal(t, "http://example.com/users/abc/orders/def", url)
}

func TestRequestURLEscapesCatchAllSegments(t *testing.T) {
	client := &Client{Server: "http://example.com"}
	route := router.RouteInfo{
		Path: "/files/*rest",
		Params: []router.ParamInfo{
			{Name: "rest", Source: "path"},
		},
	}
	url := client.requestURL(route, map[string]string{"rest": "a b/c"})
	assert.Equal(t, "http://example.com/files/a%20b/c", url)
}

func readBody(r *http.Request) string {
	if r.Body == nil {
		return ""
	}
	defer r.Body.Close()
	buf := new(strings.Builder)
	_, _ = io.Copy(buf, r.Body)
	return buf.String()
}
