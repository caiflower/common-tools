package telemetry

import (
	"errors"
	"go.opentelemetry.io/otel/trace"
	"net/http"

	golocalv1 "github.com/caiflower/common-tools/pkg/golocal/v1"
	"github.com/caiflower/common-tools/pkg/tools"
	"github.com/caiflower/common-tools/web"
	"github.com/caiflower/common-tools/web/e"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.10.0"
)

const (
	uptraceSpan    = "uptraceDone"
	uptraceContent = "uptraceContent"
)

type WebInterceptor struct {
}

func NewWebInterceptor() *WebInterceptor {
	return &WebInterceptor{}
}

func (wi *WebInterceptor) Before(ctx *web.Context) e.ApiError {
	content := new(Content)
	attrs := make([]attribute.KeyValue, 6, 10)
	attrs[0] = attribute.String("http.request.action", ctx.GetAction())
	attrs[1] = semconv.HTTPMethodKey.String(ctx.GetMethod())
	_, r := ctx.GetResponseWriterAndRequest()
	attrs[2] = semconv.HTTPClientIPKey.String(r.RemoteAddr)
	attrs[3] = semconv.HTTPRouteKey.String(ctx.GetPath())
	content.Attrs = attrs
	traceId := golocalv1.GetTraceID()

	span := DefaultClient.Start(traceId, "github.com/caiflower/common-tools/web/v1", ctx.GetAction(), trace.SpanKindServer)
	ctx.Put(uptraceSpan, span)
	ctx.Put(uptraceContent, content)

	return nil
}

func (wi *WebInterceptor) After(ctx *web.Context, err e.ApiError) e.ApiError {
	span := ctx.Get(uptraceSpan).(trace.Span)
	content := ctx.Get(uptraceContent).(*Content)
	defer DefaultClient.End(span, content)

	if err != nil {
		content.Attrs[4] = semconv.HTTPStatusCodeKey.Int(err.GetCode())
		content.Attrs[5] = attribute.String("http.response.error.message", err.GetMessage())
		content.Failed = err.GetCause()
	} else {
		content.Attrs[4] = semconv.HTTPStatusCodeKey.Int(http.StatusOK)
		content.Attrs[5] = attribute.String("http.response.data", tools.ToJson(ctx.GetResponse()))
	}

	return nil
}

func (wi *WebInterceptor) OnPanic(ctx *web.Context, recover interface{}) e.ApiError {
	span := ctx.Get(uptraceSpan).(trace.Span)
	content := ctx.Get(uptraceContent).(*Content)
	defer DefaultClient.End(span, content)

	content.Attrs[4] = semconv.HTTPStatusCodeKey.Int(http.StatusInternalServerError)
	content.Failed = errors.New(tools.ToJson(recover))

	return nil
}
