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
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"

	"github.com/caiflower/common-tools/global"
	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/uptrace/uptrace-go/uptrace"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type Config struct {
	DNS            string `yaml:"dns" json:"dns"`
	ServiceName    string `yaml:"serviceName" json:"serviceName"`
	ServiceVersion string `yaml:"serviceVersion" json:"serviceVersion"`
	DeploymentEnv  string `yaml:"deploymentEnv" json:"deploymentEnv"`
}

const (
	envServiceName    = "OTEL_SERVICE_NAME"
	envServiceVersion = "OTEL_SERVICE_VERSION"
	envDeploymentEnv  = "OTEL_DEPLOYMENT_ENVIRONMENT"
	envResourceAttrs  = "OTEL_RESOURCE_ATTRIBUTES"
)

var (
	// These values can be replaced at build time with:
	// go build -ldflags "-X github.com/caiflower/common-tools/otel.buildServiceName=..."
	// go build -ldflags "-X github.com/caiflower/common-tools/otel.buildServiceVersion=..."
	// go build -ldflags "-X github.com/caiflower/common-tools/otel.buildDeploymentEnv=..."
	buildServiceName    = "unset"
	buildServiceVersion = "v1.0.0"
	buildDeploymentEnv  = "prod"
)

var once sync.Once
var DefaultClient *client

type client struct {
	config Config
}

func Init(config Config) {
	config = resolveConfig(config)
	uptrace.SetLogger(logger.DefaultLogger())

	options := make([]uptrace.Option, 0, 10)
	if config.DNS != "" {
		options = append(options, uptrace.WithDSN(config.DNS))
	}

	options = append(options, uptrace.WithServiceName(config.ServiceName))
	options = append(options, uptrace.WithServiceVersion(config.ServiceVersion))
	options = append(options, uptrace.WithDeploymentEnvironment(config.DeploymentEnv))

	uptrace.ConfigureOpentelemetry(
		// copy your project DSN here or use UPTRACE_DSN env var
		options...,
	)

	once.Do(func() {
		DefaultClient = &client{config: config}
		global.DefaultResourceManger.Add(DefaultClient)
	})
}

// resolveConfig fills ServiceName, ServiceVersion and DeploymentEnv with the
// following priority: environment variables, config values, build-time values.
// Supported env vars are OTEL_SERVICE_NAME, OTEL_SERVICE_VERSION,
// OTEL_DEPLOYMENT_ENVIRONMENT and OTEL_RESOURCE_ATTRIBUTES (service.name,
// service.version, deployment.environment).
func resolveConfig(config Config) Config {
	attrs := resourceAttributes()
	config.ServiceName = firstNonEmpty(
		strings.TrimSpace(os.Getenv(envServiceName)),
		attrs["service.name"],
		config.ServiceName,
		buildServiceName,
	)
	config.ServiceVersion = firstNonEmpty(
		strings.TrimSpace(os.Getenv(envServiceVersion)),
		attrs["service.version"],
		config.ServiceVersion,
		buildServiceVersion,
	)
	config.DeploymentEnv = firstNonEmpty(
		strings.TrimSpace(os.Getenv(envDeploymentEnv)),
		attrs["deployment.environment"],
		config.DeploymentEnv,
		buildDeploymentEnv,
	)
	return config
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

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
		return endpoint, false, nil
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
		return u.Host, false, nil
	case "https":
		return u.Host, true, nil
	default:
		return "", false, fmt.Errorf("unsupported otel endpoint scheme %q", u.Scheme)
	}
}

func resourceAttributes() map[string]string {
	attrs := make(map[string]string)
	for _, pair := range strings.Split(os.Getenv(envResourceAttrs), ",") {
		key, value, found := strings.Cut(pair, "=")
		if !found {
			continue
		}
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		value = strings.TrimSpace(value)
		if unescaped, err := url.PathUnescape(value); err == nil {
			value = unescaped
		}
		attrs[key] = value
	}
	return attrs
}

type Content struct {
	Attrs  []attribute.KeyValue
	Failed error
}

func (c *client) Start(_traceID string, tracerName, spanName string, kind trace.SpanKind) trace.Span {
	_tracer := otel.Tracer(tracerName)
	traceID, err := trace.TraceIDFromHex(_traceID)
	if err != nil {
		logger.Error("telemetry get traceId from hex failed. Error: %v", err)
	}

	_, span := _tracer.Start(
		trace.ContextWithSpanContext(context.Background(),
			trace.NewSpanContext(trace.SpanContextConfig{
				TraceID: traceID,
			})),
		spanName,
		trace.WithSpanKind(kind))

	return span
}

func (c *client) End(span trace.Span, content *Content) {
	if span == nil || !span.IsRecording() {
		return
	}
	defer span.End()

	if content != nil {
		span.SetAttributes(content.Attrs...)
		if content.Failed != nil {
			span.RecordError(content.Failed)
			span.SetStatus(codes.Error, content.Failed.Error())
		}
	}

	logger.Trace("uptrace: %s\n", uptrace.TraceURL(span))
}

func (c *client) Close() {
	err := uptrace.Shutdown(context.Background())
	if err != nil {
		logger.Error("*** telemetry client shutdown failed. *** Error: %s", err)
	}
	logger.Info("*** telemetry client shutdown successfully. ***")
}
