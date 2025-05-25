package telemetry

import (
	"errors"
	"net/http"

	golocalv1 "github.com/caiflower/common-tools/pkg/golocal/v1"
	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/caiflower/common-tools/pkg/tools"
	"github.com/caiflower/common-tools/web"
	"github.com/caiflower/common-tools/web/e"
	"go.opentelemetry.io/otel/attribute"
)

const (
	uptraceDone    = "uptraceDone"
	uptraceContent = "uptraceContent"
)

type WebInterceptor struct {
}

func (wi *WebInterceptor) Before(ctx *web.Context) e.ApiError {
	content := new(Content)
	attrs := make([]attribute.KeyValue, 3)
	attrs[0] = attribute.String("action", ctx.GetAction())
	content.Attrs = attrs
	traceId := golocalv1.GetTraceID()

	done, err := DefaultClient.Record(traceId, "webv1", content)
	if err != nil {
		logger.Error("telemetry failed. Error: %v", err)
	}
	ctx.Put(uptraceDone, done)
	ctx.Put(uptraceContent, content)
	return nil
}

func (wi *WebInterceptor) After(ctx *web.Context, err e.ApiError) e.ApiError {
	defer func() {
		done := ctx.Get(uptraceDone).(chan<- struct{})
		done <- struct{}{}
	}()

	content := ctx.Get(uptraceContent).(*Content)
	if err != nil {
		content.Attrs[1] = attribute.Int("code", err.GetCode())
		content.Attrs[2] = attribute.String("error.message", err.GetMessage())
		content.Failed = err.GetCause()
	} else {
		content.Attrs[1] = attribute.Int("code", http.StatusOK)
		content.Attrs[2] = attribute.String("data", tools.ToJson(ctx.GetResponse()))
	}

	return nil
}

func (wi *WebInterceptor) OnPanic(ctx *web.Context, recover interface{}) e.ApiError {
	defer func() {
		done := ctx.Get(uptraceDone).(chan<- struct{})
		done <- struct{}{}
	}()

	content := ctx.Get(uptraceContent).(*Content)
	content.Attrs[1] = attribute.Int("code", http.StatusInternalServerError)
	content.Failed = errors.New(tools.ToJson(recover))

	return nil
}
