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

package web

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/caiflower/common-tools/web/app/server/config"
	"github.com/stretchr/testify/assert"
)

// The default server mode is netpoll; CLI metadata options must reach the
// handler in that mode too, not only in ServerModeStandard.
func TestDefaultNetpollServesCLIRoutes(t *testing.T) {
	engine := Default(
		config.WithName("cli-test"),
		config.WithEnableCLI(true),
	)

	rec := httptest.NewRecorder()
	engine.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/cli/routes", nil))
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.NotEmpty(t, rec.Body.Bytes())
}

func TestDefaultNetpollCLIRoutesDisabledByDefault(t *testing.T) {
	engine := Default(config.WithName("cli-test"))

	rec := httptest.NewRecorder()
	engine.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/cli/routes", nil))
	assert.Equal(t, http.StatusNotFound, rec.Code)
}
