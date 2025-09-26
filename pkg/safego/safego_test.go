package safego

import (
	"context"
	"sync"
	"testing"

	golocalv1 "github.com/caiflower/common-tools/pkg/golocal/v1"
	"github.com/stretchr/testify/assert"
)

func TestSageGo(t *testing.T) {
	ctx := context.Background()
	trace := "test"
	golocalv1.PutTraceID(trace)
	golocalv1.PutContext(ctx)

	wg := &sync.WaitGroup{}
	for i := 0; i < 10000; i++ {
		wg.Add(1)
		Go(func() {
			defer wg.Done()
			assert.Equal(t, golocalv1.GetTraceID(), trace)
			assert.Equal(t, golocalv1.GetContext(), ctx)
		})
	}
	wg.Wait()
}
