package app

import "github.com/caiflower/common-tools/web/common/webctx"

type HandlerFunc = func(ctx *webctx.RequestCtx)
