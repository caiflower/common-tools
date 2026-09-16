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

package router

import (
	"context"
	"sync"
)

// inflight counts the requests currently being handled so that a server can
// drain them during graceful shutdown.
//
// Both HTTP server forms (standard and netpoll) funnel every request through
// Handler.ServeHTTP / Handler.Serve, so tracking here gives request-level
// draining to both of them without touching the transports.
type inflight struct {
	mu       sync.Mutex
	num      int
	draining bool
	done     chan struct{}
	doneOnce sync.Once
}

func newInflight() *inflight {
	return &inflight{done: make(chan struct{})}
}

// enter registers an in-flight request. It returns false once draining started,
// so the caller can reject the request instead of handling it.
func (f *inflight) enter() bool {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.draining {
		return false
	}

	f.num++
	return true
}

func (f *inflight) leave() {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.num > 0 {
		f.num--
	}

	if f.draining && f.num == 0 {
		f.closeDone()
	}
}

// drain stops accepting new requests and waits until every in-flight request
// finished, or ctx is done.
func (f *inflight) drain(ctx context.Context) error {
	f.mu.Lock()
	f.draining = true
	empty := f.num == 0
	f.mu.Unlock()

	if empty {
		f.closeDone()
	}

	select {
	case <-f.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (f *inflight) closeDone() {
	f.doneOnce.Do(func() {
		close(f.done)
	})
}
