# OTel Native SDK Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace Uptrace-based OTel initialization with native OpenTelemetry SDK and support gRPC/HTTP exporters.

**Architecture:** `otel.Config` gains `Endpoint` and `Protocol`; `Init` creates an OTLP exporter, builds a TracerProvider, and registers it globally. Existing hooks continue to use the global TracerProvider.

**Tech Stack:** Go, OpenTelemetry SDK, `otlptracegrpc`, `otlptracehttp`.

---

### Task 1: Endpoint and protocol normalization

**Files:**
- Modify: `otel/client.go`
- Test: `otel/client_test.go`

- [ ] **Step 1: Write failing tests**

Add to `otel/client_test.go`:

```go
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
```

- [ ] **Step 2: Run tests and verify they fail**

Run: `go test ./otel/ -run 'TestNormalizeProtocol|TestNormalizeEndpoint'`
Expected: FAIL with `normalizeProtocol` and `normalizeEndpoint` undefined.

- [ ] **Step 3: Implement helpers in `otel/client.go`**

Add imports `fmt`, `net/url` if not already present, and add:

```go
func normalizeProtocol(protocol string) string {
	protocol = strings.ToLower(strings.TrimSpace(protocol))
	if protocol == "" {
		return "grpc"
	}
	return protocol
}

func normalizeEndpoint(endpoint string) (string, bool, error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return "", false, fmt.Errorf("otel endpoint is empty")
	}
	if !strings.Contains(endpoint, "://") {
		return endpoint, true, nil
	}

	u, err := url.Parse(endpoint)
	if err != nil {
		return "", false, err
	}
	if u.Host == "" {
		return "", false, fmt.Errorf("otel endpoint has no host")
	}

	switch u.Scheme {
	case "http":
		return u.Host, true, nil
	case "https":
		return u.Host, false, nil
	default:
		return "", false, fmt.Errorf("unsupported otel endpoint scheme %q", u.Scheme)
	}
}
```

- [ ] **Step 4: Run tests and verify they pass**

Run: `go test ./otel/ -run 'TestNormalizeProtocol|TestNormalizeEndpoint'`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add otel/client.go otel/client_test.go
git commit -m "test: add otel endpoint normalization"
```

---

### Task 2: Rewrite Init with native OTel SDK

**Files:**
- Modify: `otel/client.go`
- Test: `otel/client_test.go`

- [ ] **Step 1: Write failing exporter tests**

Add to `otel/client_test.go`:

```go
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
```

Add `"context"` to `otel/client_test.go` imports.

- [ ] **Step 2: Run tests and verify they fail**

Run: `go test ./otel/ -run TestNewTraceExporter`
Expected: FAIL with `newTraceExporter` undefined.

- [ ] **Step 3: Rewrite `Config`, `client`, and `Init`**

Replace the `Config` struct:

```go
type Config struct {
	Endpoint       string `yaml:"endpoint" json:"endpoint"`
	Protocol       string `yaml:"protocol" json:"protocol"`
	ServiceName    string `yaml:"serviceName" json:"serviceName"`
	ServiceVersion string `yaml:"serviceVersion" json:"serviceVersion"`
	DeploymentEnv  string `yaml:"deploymentEnv" json:"deploymentEnv"`
}
```

Replace the client struct:

```go
type client struct {
	config         Config
	tracerProvider *sdktrace.TracerProvider
}
```

Replace `Init`:

```go
func Init(config Config) {
	config = resolveConfig(config)
	protocol := normalizeProtocol(config.Protocol)
	exporter, err := newTraceExporter(config.Endpoint, protocol)
	if err != nil {
		panic(fmt.Sprintf("init otel failed. err: %v", err))
	}

	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(newResource(config)),
	)
	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	once.Do(func() {
		DefaultClient = &client{config: config, tracerProvider: provider}
		global.DefaultResourceManger.Add(DefaultClient)
	})
}
```

Add exporter and resource helpers:

```go
func newTraceExporter(endpoint, protocol string) (sdktrace.SpanExporter, error) {
	addr, secure, err := normalizeEndpoint(endpoint)
	if err != nil {
		return nil, err
	}

	switch protocol {
	case "grpc":
		opts := []otlptracegrpc.Option{otlptracegrpc.WithEndpoint(addr)}
		if !secure {
			opts = append(opts, otlptracegrpc.WithInsecure())
		}
		return otlptracegrpc.New(context.Background(), opts...)
	case "http":
		opts := []otlptracehttp.Option{otlptracehttp.WithEndpoint(addr)}
		if !secure {
			opts = append(opts, otlptracehttp.WithInsecure())
		}
		return otlptracehttp.New(context.Background(), opts...)
	default:
		return nil, fmt.Errorf("unsupported otel protocol %q", protocol)
	}
}

func newResource(config Config) *resource.Resource {
	return resource.NewSchemaless(
		attribute.String("service.name", config.ServiceName),
		attribute.String("service.version", config.ServiceVersion),
		attribute.String("deployment.environment", config.DeploymentEnv),
	)
}
```

Replace `client.Close`:

```go
func (c *client) Close() {
	if c.tracerProvider == nil {
		return
	}
	if err := c.tracerProvider.Shutdown(context.Background()); err != nil {
		logger.Error("*** telemetry client shutdown failed. *** Error: %s", err)
	}
	logger.Info("*** telemetry client shutdown successfully. ***")
}
```

Replace `client.End` trace log:

```go
logger.Trace("otel trace: %s", span.SpanContext().TraceID())
```

Update imports:

```go
import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"

	"github.com/caiflower/common-tools/global"
	"github.com/caiflower/common-tools/pkg/logger"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)
```

Remove all `github.com/uptrace/uptrace-go/uptrace` imports from this file.

- [ ] **Step 4: Run otel tests and verify they pass**

Run: `go test ./otel/...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add otel/client.go otel/client_test.go
git commit -m "feat: migrate otel init to native otel sdk"
```

---

### Task 3: Replace Uptrace trace logs in hooks

**Files:**
- Modify: `otel/redisotel.go`
- Modify: `otel/db_otel.go`
- Modify: `otel/httpclient_otel.go`
- Modify: `otel/web_client_otel.go`

- [ ] **Step 1: Replace all uptrace logs**

In each file, replace:

```go
logger.Trace("uptrace: %s\n", uptrace.TraceURL(span))
```

with:

```go
logger.Trace("otel trace: %s", span.SpanContext().TraceID())
```

Remove the corresponding import:

```go
"github.com/uptrace/uptrace-go/uptrace"
```

- [ ] **Step 2: Run tests**

Run: `go test ./otel/...`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add otel/redisotel.go otel/db_otel.go otel/httpclient_otel.go otel/web_client_otel.go
git commit -m "refactor: use standard trace id logging in otel hooks"
```

---

### Task 4: Dependency cleanup and full verification

**Files:**
- Modify: `go.mod`
- Modify: `go.sum`

- [ ] **Step 1: Remove Uptrace and promote native OTel dependencies**

Run:

```bash
go mod tidy
```

Then verify `go.mod`:

```bash
rg -n "uptrace-go|otlptracegrpc|otlptracehttp|otel/sdk" go.mod
```

Expected:
- `github.com/uptrace/uptrace-go` is gone.
- `go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc` is direct.
- `go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp` is direct.
- `go.opentelemetry.io/otel/sdk` is direct.

- [ ] **Step 2: Run full tests**

Run: `go test ./otel/...`
Expected: PASS

- [ ] **Step 3: Run full repo tests and vet**

Run:

```bash
go test ./...
go vet ./...
```

Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add go.mod go.sum
git commit -m "chore: replace uptrace dependency with native otel exporters"
```
