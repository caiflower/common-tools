package telemetry

import (
	"context"
	"reflect"
	"sync"
	"time"

	"github.com/caiflower/common-tools/global"
	"github.com/caiflower/common-tools/pkg/cache"
	golocalv1 "github.com/caiflower/common-tools/pkg/golocal/v1"
	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/caiflower/common-tools/pkg/tools"
	"github.com/uptrace/uptrace-go/uptrace"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type Config struct {
	Enable         bool   `yaml:"enable" json:"enable"`
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
	if config.DNS == "" {
		config.Enable = false
	}

	_ = tools.DoTagFunc(&config, nil, []func(reflect.StructField, reflect.Value, interface{}) error{tools.SetDefaultValueIfNil})

	uptrace.ConfigureOpentelemetry(
		// copy your project DSN here or use UPTRACE_DSN env var
		uptrace.WithDSN(config.DNS),

		uptrace.WithServiceName(config.ServiceName),
		uptrace.WithServiceVersion(config.ServiceVersion),
		uptrace.WithDeploymentEnvironment(config.DeploymentEnv),
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

func (c *client) Record(traceId string, name string, content *Content) (chan<- struct{}, error) {
	done := make(chan struct{}, 1)

	if c.config.Enable {
		tracer := otel.Tracer(traceId)

		traceID, err := trace.TraceIDFromHex(traceId)
		if err != nil {
			return done, err
		}
		_, span := tracer.Start(trace.ContextWithSpanContext(context.Background(), trace.NewSpanContext(trace.SpanContextConfig{
			TraceID: traceID,
		})), name, trace.WithTimestamp(time.Now()))

		go func() {
			<-done
			if content != nil {
				span.SetAttributes(content.Attrs...)
				if content.Failed != nil {
					span.RecordError(content.Failed)
					span.SetStatus(codes.Error, content.Failed.Error())
				}
			}
			span.End(trace.WithTimestamp(time.Now()))
			logger.Info("trace: %s\n", uptrace.TraceURL(span))
		}()
	}

	return done, nil
}

func (c *client) Clean() {
	cache.LocalCache.Delete(golocalv1.GetTraceID())
}

func (c *client) Close() {
	uptrace.Shutdown(context.Background())
}
