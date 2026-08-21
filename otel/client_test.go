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

package otel

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolveConfigPriority(t *testing.T) {
	clearEnv(t)
	t.Setenv(envServiceName, "env-name")
	t.Setenv(envServiceVersion, "env-version")
	t.Setenv(envDeploymentEnv, "env-deploy")
	t.Setenv(envResourceAttrs, "service.name=attr-name, service.version=attr-version, deployment.environment=attr-deploy")
	withBuildValues(t, "build-name", "build-version", "build-deploy")

	resolved := resolveConfig(Config{
		ServiceName:    "config-name",
		ServiceVersion: "config-version",
		DeploymentEnv:  "config-deploy",
	})

	assert.Equal(t, "env-name", resolved.ServiceName)
	assert.Equal(t, "env-version", resolved.ServiceVersion)
	assert.Equal(t, "env-deploy", resolved.DeploymentEnv)
}

func TestResolveConfigConfigOverBuild(t *testing.T) {
	clearEnv(t)
	withBuildValues(t, "build-name", "build-version", "build-deploy")

	resolved := resolveConfig(Config{
		ServiceName:    "config-name",
		ServiceVersion: "config-version",
		DeploymentEnv:  "config-deploy",
	})

	assert.Equal(t, "config-name", resolved.ServiceName)
	assert.Equal(t, "config-version", resolved.ServiceVersion)
	assert.Equal(t, "config-deploy", resolved.DeploymentEnv)
}

func TestResolveConfigBuildFallback(t *testing.T) {
	clearEnv(t)
	withBuildValues(t, "build-name", "build-version", "build-deploy")

	resolved := resolveConfig(Config{})

	assert.Equal(t, "build-name", resolved.ServiceName)
	assert.Equal(t, "build-version", resolved.ServiceVersion)
	assert.Equal(t, "build-deploy", resolved.DeploymentEnv)
}

func TestResolveConfigResourceAttributes(t *testing.T) {
	clearEnv(t)
	t.Setenv(envResourceAttrs, "service.name=attr-name, service.version=1.2%2E3, deployment.environment=staging%2C-eu, invalid-entry")
	withBuildValues(t, "build-name", "build-version", "build-deploy")

	resolved := resolveConfig(Config{})

	assert.Equal(t, "attr-name", resolved.ServiceName)
	assert.Equal(t, "1.2.3", resolved.ServiceVersion)
	assert.Equal(t, "staging,-eu", resolved.DeploymentEnv)
}

func TestFirstNonEmpty(t *testing.T) {
	assert.Equal(t, "config", firstNonEmpty("", "config", "build"))
	assert.Equal(t, "build", firstNonEmpty("", "", "build"))
	assert.Equal(t, "", firstNonEmpty("", ""))
}

func TestNormalizeProtocol(t *testing.T) {
	assert.Equal(t, "grpc", normalizeProtocol(""))
	assert.Equal(t, "grpc", normalizeProtocol("GRPC"))
	assert.Equal(t, "http", normalizeProtocol("http"))
}

func TestNormalizeEndpoint(t *testing.T) {
	tests := []struct {
		name       string
		endpoint   string
		wantAddr   string
		wantSecure bool
		wantErr    bool
	}{
		{name: "bare host port", endpoint: "tempo.monitor:4317", wantAddr: "tempo.monitor:4317"},
		{name: "http scheme", endpoint: "http://tempo.monitor:4317", wantAddr: "tempo.monitor:4317"},
		{name: "https scheme", endpoint: "https://tempo.monitor:4317", wantAddr: "tempo.monitor:4317", wantSecure: true},
		{name: "empty", endpoint: "", wantErr: true},
		{name: "bad scheme", endpoint: "ftp://tempo.monitor:4317", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addr, secure, err := normalizeEndpoint(tt.endpoint)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.wantAddr, addr)
			assert.Equal(t, tt.wantSecure, secure)
		})
	}
}

func TestNewTraceExporter(t *testing.T) {
	grpcExporter, err := newTraceExporter("localhost:4317", "grpc")
	assert.NoError(t, err)
	assert.NotNil(t, grpcExporter)
	t.Cleanup(func() { _ = grpcExporter.Shutdown(context.Background()) })

	httpExporter, err := newTraceExporter("localhost:4318", "http")
	assert.NoError(t, err)
	assert.NotNil(t, httpExporter)
	t.Cleanup(func() { _ = httpExporter.Shutdown(context.Background()) })

	_, err = newTraceExporter("localhost:4317", "bad")
	assert.Error(t, err)
}

func clearEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{envServiceName, envServiceVersion, envDeploymentEnv, envResourceAttrs} {
		t.Setenv(key, "")
	}
}

func withBuildValues(t *testing.T, name, version, env string) {
	t.Helper()
	oldName, oldVersion, oldEnv := buildServiceName, buildServiceVersion, buildDeploymentEnv
	buildServiceName, buildServiceVersion, buildDeploymentEnv = name, version, env
	t.Cleanup(func() {
		buildServiceName, buildServiceVersion, buildDeploymentEnv = oldName, oldVersion, oldEnv
	})
}
