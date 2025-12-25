package suit

import (
	"sync"

	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/caiflower/common-tools/web/common/webctx"
)

// Core is the core interface that promises to be provided for the protocol layer extensions
type Core interface {
	// IsRunning Check whether engine is running or not
	IsRunning() bool
	// GetCtxPool A RequestContext pool ready for protocol server impl
	GetCtxPool() *sync.Pool
	// Serve Business logic entrance
	// After pre-read works, protocol server may call this method
	// to introduce the middlewares and handlers
	Serve(ctx *webctx.RequestCtx)
	GetLogger() logger.ILog
}
