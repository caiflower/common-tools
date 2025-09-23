package safego

import "github.com/caiflower/common-tools/pkg/e"

func Go(fn func()) {
	go func() {
		defer e.OnError("safeGo")

		fn()
	}()
}
