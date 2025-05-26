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
	ServiceName    string `yaml:"serviceName" json:"serviceName"`
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

	uptrace.ConfigureOpentelemetry(
		// copy your project DSN here or use UPTRACE_DSN env var
		uptrace.WithDSN(config.DNS),

		uptrace.WithServiceName(config.ServiceName),
		uptrace.WithServiceVersion(config.ServiceVersion),
		uptrace.WithDeploymentEnvironment(config.DeploymentEnv),
	)
	uptrace.SetLogger(logger.DefaultLogger())

	once.Do(func() {
		DefaultClient = &client{config: config}
		global.DefaultResourceManger.Add(DefaultClient)
	})
}

type Content struct {
	Attrs  []attribute.KeyValue
	Failed error
}

func (c *client) Record(traceId string, tracerName, name string, content *Content) (chan<- struct{}, error) {
	done := make(chan struct{}, 1)

	tracer := otel.Tracer(tracerName)

	traceID, err := trace.TraceIDFromHex(traceId)
	if err != nil {
		return done, err
	}
	_, span := tracer.Start(trace.ContextWithSpanContext(context.Background(), trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: traceID,
	})), name)

	go func() {
		<-done
		if !span.IsRecording() {
			return
		}

		if content != nil {
			span.SetAttributes(content.Attrs...)
			if content.Failed != nil {
				span.RecordError(content.Failed)
				span.SetStatus(codes.Error, content.Failed.Error())
			}
		}
		span.End()
		logger.Debug("uptrace: %s\n", uptrace.TraceURL(span))
	}()

	return done, nil
}

func (c *client) Close() {
	err := uptrace.Shutdown(context.Background())
	if err != nil {
		logger.Error("telemetry client shutdown failed. Error: %s", err)
	}
	logger.Info("telemetry client shutdown successfully.")
}
