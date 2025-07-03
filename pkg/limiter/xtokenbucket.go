package limiter

import (
	"context"
	"golang.org/x/time/rate"
	"time"
)

type XTokenBucket struct {
	qos     int
	limiter *rate.Limiter
}

func NewXTokenBucket(qos, burst int) *XTokenBucket {
	l := &XTokenBucket{
		qos:     qos,
		limiter: rate.NewLimiter(rate.Limit(qos), burst),
	}

	return l
}

func (x *XTokenBucket) TakeToken() {
	_ = x.limiter.Wait(context.Background())
	return
}

func (x *XTokenBucket) TakeTokenNonBlocking() bool {
	return x.limiter.Allow()
}

func (x *XTokenBucket) TakeTokenWithTimeout(timeout time.Duration) bool {
	withTimeout, cancelFunc := context.WithTimeout(context.Background(), timeout)
	defer cancelFunc()
	return x.limiter.Wait(withTimeout) == nil
}
