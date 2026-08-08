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

package router

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/stretchr/testify/assert"
)

func newCLITestHandler(enableCLI bool) *Handler {
	handler := NewHandler(HandlerCfg{
		Name:                   "cli-test",
		DisableOptimization:    true,
		EnableCLI:              enableCLI,
		CLIRoutesPath:          "/cli/routes",
		EnableActionController: false,
	}, logger.DefaultLogger())
	group := NewRouterGroup(handler)
	group.GET("/users/:id", cliTestGet)
	return handler
}

func TestCLIRoutesDisabled(t *testing.T) {
	handler := newCLITestHandler(false)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/cli/routes", nil))
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestCLIRoutesEnabled(t *testing.T) {
	handler := newCLITestHandler(true)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/cli/routes", nil))
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.NotEmpty(t, rec.Header().Get("ETag"))

	var payload struct {
		Name      string      `json:"name"`
		Version   string      `json:"version"`
		Resources []string    `json:"resources"`
		Routes    []RouteInfo `json:"routes"`
	}
	err := json.Unmarshal(rec.Body.Bytes(), &payload)
	assert.NoError(t, err)
	assert.Equal(t, "cli-test", payload.Name)
	assert.NotEmpty(t, payload.Version)
	assert.Contains(t, payload.Resources, "users")
	assert.Len(t, payload.Routes, 1)
	assert.Equal(t, "GET", payload.Routes[0].Method)
}

func TestCLIRoutesNotModified(t *testing.T) {
	handler := newCLITestHandler(true)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/cli/routes", nil))
	etag := rec.Header().Get("ETag")
	assert.NotEmpty(t, etag)

	req := httptest.NewRequest(http.MethodGet, "/cli/routes", nil)
	req.Header.Set("If-None-Match", etag)
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req)
	assert.Equal(t, http.StatusNotModified, rec2.Code)
	assert.Empty(t, rec2.Body.Bytes())
}
