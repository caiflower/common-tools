package safego

import (
	"github.com/caiflower/common-tools/pkg/e"
	golocalv1 "github.com/caiflower/common-tools/pkg/golocal/v1"
)

func Go(fn func()) {
	context := golocalv1.GetContext()
	go func() {
		defer e.OnError("safeGo")
		golocalv1.PutContext(context)
		defer golocalv1.Clean()
		
		fn()
	}()
}
