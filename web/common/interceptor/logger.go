package interceptor

import (
	"time"

	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/caiflower/common-tools/web/app"
	"github.com/caiflower/common-tools/web/common/e"
)

// LoggerInterceptor is deprecated. Use RouterGroup.Use() middleware instead.
// Example replacement:
//
//	engine.Use(func(ctx context.Context, reqCtx *app.RequestContext) {
//	    start := time.Now()
//	    reqCtx.Next(ctx)
//	    latency := time.Since(start)
//	    logger.Info("| %3d | %13v | %15s | %7s %s",
//	        reqCtx.GetStatusCode(), latency, reqCtx.ClientIP(), reqCtx.GetAction(), reqCtx.GetPath())
//	})
type LoggerInterceptor struct{}

func (l *LoggerInterceptor) Before(ctx *app.Context) e.ApiError {
	ctx.Set("common-tool:interceptor:beginTime", time.Now())
	return nil
}

func (l *LoggerInterceptor) After(ctx *app.Context, err e.ApiError) e.ApiError {
	t, _ := ctx.Get("common-tool:interceptor:beginTime")
	latency := time.Now().Sub(t.(time.Time))
	logger.Info("| %3d | %13v | %15s | %7s %s",
		ctx.GetStatusCode(),
		latency,
		ctx.ClientIP(),
		ctx.GetAction(),
		ctx.GetPath(),
	)
	return nil
}

func (l *LoggerInterceptor) OnPanic(ctx *app.Context, err interface{}) e.ApiError {
	return nil
}
