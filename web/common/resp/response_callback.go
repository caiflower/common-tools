package resp

import (
	"net/http"
	"sync"

	golocalv1 "github.com/caiflower/common-tools/pkg/golocal/v1"
	"github.com/caiflower/common-tools/web/app"
	"github.com/caiflower/common-tools/web/common/e"
)

var once sync.Once

var DefaultResultCallback = func(ctx *app.RequestContext) bool {
	if err := ctx.GetError(); err != nil {
		res := Result{
			RequestID: golocalv1.GetTraceID(),
			Error:     &e.Error{Code: err.GetCode(), Message: err.GetMessage(), Type: err.GetType(), Cause: err.GetCause()},
		}
		if ctx.IsRestful() {
			ctx.JSON(err.GetCode(), res)
		} else {
			ctx.JSON(http.StatusOK, res)
		}
	} else {
		res := Result{
			RequestID: golocalv1.GetTraceID(),
			Data:      ctx.GetData(),
		}
		ctx.JSON(http.StatusOK, res)
	}
	return false
}
