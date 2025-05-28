package telemetry

import (
	"context"
	"github.com/caiflower/common-tools/global"
	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/caiflower/common-tools/pkg/tools"
	"github.com/uptrace/uptrace-go/uptrace"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"reflect"
	"sync"
)

type Config struct {
	DNS            string `yaml:"dns" json:"dns"`
	ServiceName    string `yaml:"serviceName" json:"serviceName" default:"unset"`
	ServiceVersion string `yaml:"serviceVersion" json:"serviceVersion" default:"v1.0.0"`
	DeploymentEnv  string `yaml:"deploymentEnv" json:"deploymentEnv" default:"prod"`
}

var once sync.Once
var DefaultClient *client

type client struct {
	config Config
}

func Init(config Config) {
	_ = tools.DoTagFunc(&config, nil, []func(reflect.StructField, reflect.Value, interface{}) error{tools.SetDefaultValueIfNil})
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

type Content struct {
	Attrs  []attribute.KeyValue
	Failed error
}

func (c *client) Start(traceId string, tracerName, spanName string, kind trace.SpanKind) trace.Span {
	_tracer := otel.Tracer(tracerName)
	traceID, err := trace.TraceIDFromHex(traceId)
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
