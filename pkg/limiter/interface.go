package limiter

import "time"

type Limiter interface {
	TakeToken()
	TakeTokenNonBlocking() bool
	TakeTokenWithTimeout(timeout time.Duration) bool
}
