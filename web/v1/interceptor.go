package v1

import (
	"net/http"

	"github.com/caiflower/common-tools/web/e"
)

type Interceptor interface {
	BeforeCallTargetMethod(w http.ResponseWriter, r *http.Request, ctx *RequestCtx) e.ApiError
	AfterCallTargetMethod(w http.ResponseWriter, r *http.Request, ctx *RequestCtx) e.ApiError
}

type InterceptorSort []InterceptorItem

type InterceptorItem struct {
	interceptor Interceptor
	order       int
}

func (itemList InterceptorSort) Len() int {
	return len([]InterceptorItem(itemList))
}

func (itemList InterceptorSort) Less(i, j int) bool {
	return itemList[i].order < itemList[j].order
}

func (itemList InterceptorSort) Swap(i, j int) {
	itemList[i], itemList[j] = itemList[j], itemList[i]
}
