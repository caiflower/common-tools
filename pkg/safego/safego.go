package safego

import (
	"github.com/caiflower/common-tools/pkg/e"
	golocalv1 "github.com/caiflower/common-tools/pkg/golocal/v1"
)

func Go(fn func()) {
	localMap := golocalv1.GetLocalMap()
	go func() {
		defer e.OnError("safeGo")
		golocalv1.PutLocalMap(localMap)
		defer golocalv1.Clean()

		fn()
	}()
}
