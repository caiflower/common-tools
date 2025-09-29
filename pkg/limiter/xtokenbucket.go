/*
 * Copyright 2024 caiflower Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

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
