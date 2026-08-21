# OTel Native SDK Design

Date: 2026-08-21

## Context

`common-tools/otel` currently depends on `uptrace-go` and uses Uptrace DSN plus OTLP HTTP exporter. This makes it incompatible with a plain OTLP gRPC endpoint such as `tempo.monitor:4317` and requires placeholder tokens when a plain HTTP endpoint is configured.

The goal is to move to the native OpenTelemetry SDK and support both OTLP gRPC and OTLP HTTP exporters through configuration.

## Goals

- Replace Uptrace-based initialization with native OTel SDK.
- Support `protocol: grpc` and `protocol: http`.
- Accept plain endpoints without tokens.
- Keep existing web/HTTP client/Redis/DB tracing hooks working through the global TracerProvider.
- Keep `otel.Init(config)` as the single entrypoint.

## Non-Goals

- Preserve backward compatibility with the old `dns` field.
- Keep Uptrace-specific features such as `TraceURL`.
- Add metrics or logs exporters.

## Config

Replace the existing `Config`:

```go
type Config struct {
    Endpoint       string `yaml:"endpoint" json:"endpoint"`
    Protocol       string `yaml:"protocol" json:"protocol"`
    ServiceName    string `yaml:"serviceName" json:"serviceName"`
    ServiceVersion string `yaml:"serviceVersion" json:"serviceVersion"`
    DeploymentEnv  string `yaml:"deploymentEnv" json:"deploymentEnv"`
}
```

- `Protocol` accepts `grpc` or `http`; empty value defaults to `grpc`.
- `Endpoint` accepts `host:port`, `http://host:port`, or `https://host:port`.
- `http://` and bare `host:port` are treated as insecure.
- `https://` enables TLS.

## Initialization

`Init(config)` performs the following steps:

1. Resolve `ServiceName`, `ServiceVersion`, and `DeploymentEnv` using environment variables, config values, and build-time values in that priority order.
2. Create the OTLP trace exporter:
   - `grpc`: `otlptracegrpc`
   - `http`: `otlptracehttp`
3. Build a resource with `service.name`, `service.version`, and `deployment.environment`.
4. Create `sdktrace.TracerProvider` with a batch span processor.
5. Set the provider and `TraceContext` propagator as globals.
6. Store the provider in `DefaultClient` and register the client with `global.DefaultResourceManger`.

## Shutdown

`client.Close()` calls `TracerProvider.Shutdown(context.Background())` instead of `uptrace.Shutdown`.

## Hooks

The following files use the global `otel.Tracer` and require no logic changes:

- `web_middleware.go`
- `web_client_otel.go`
- `httpclient_otel.go`
- `redisotel.go`
- `db_otel.go`

All `uptrace.TraceURL(span)` log calls are replaced with the standard trace ID from `span.SpanContext().TraceID()`.

## Dependencies

- Remove `github.com/uptrace/uptrace-go`.
- Add direct dependency `go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc`.
- Promote `go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp` and `go.opentelemetry.io/otel/sdk` to direct dependencies.

## Error Handling

`Init` keeps its current no-return signature. Invalid protocol values or exporter creation failures panic with a descriptive error so misconfiguration fails fast at startup.

## Testing

- Update `client_test.go` for new config resolution.
- Add tests for:
  - default protocol resolution
  - endpoint scheme parsing
  - exporter selection for `grpc` and `http`
- Keep existing web middleware tests.

## Consumer Migration

`algo-invoker` must update its OTel config from:

```yaml
otel:
  dns: "..."
```

to:

```yaml
otel:
  endpoint: "tempo.monitor:4317"
  protocol: "grpc"
```

The temporary `ToOTelDSN` compatibility helper in `algo-invoker` is removed.
