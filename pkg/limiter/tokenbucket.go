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
	"time"
)

// 令牌桶 https://www.cnblogs.com/niumoo/p/16007224.html 6

type TokenBucket struct {
	qos    int
	clock  time.Duration
	bucket chan struct{}
	ctx    context.Context
	cancel context.CancelFunc
}

func NewTokenBucket(qos int) *TokenBucket {
	l := &TokenBucket{
		qos:   qos,
		clock: 1000 * time.Millisecond / time.Duration(qos),
	}

	return l
}

func (l *TokenBucket) Startup() {
	l.bucket = make(chan struct{}, l.qos)
	for i := 0; i < l.qos; i++ {
		l.bucket <- struct{}{}
	}

	l.ctx, l.cancel = context.WithCancel(context.Background())

	go func(ctx context.Context) {
		ticker := time.NewTicker(l.clock)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				select {
				case l.bucket <- struct{}{}:
				default:
					// bucket is full
				}
			}
		}
	}(l.ctx)
}

func (l *TokenBucket) Close() {
	if l.bucket != nil {
		l.cancel()

		close(l.bucket)
		l.bucket = nil
	}
}

func (l *TokenBucket) TakeToken() {
	<-l.bucket
}

func (l *TokenBucket) TakeTokenNonBlocking() bool {
	select {
	case <-l.bucket:
		return true
	default:
		return false
	}
}

func (l *TokenBucket) TakeTokenWithTimeout(timeout time.Duration) bool {
	ctx, _ := context.WithTimeout(context.Background(), timeout)
	for {
		select {
		case <-ctx.Done():
			return false
		case <-l.bucket:
			return true
		default:

		}
	}
}
