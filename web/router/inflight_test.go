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
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/caiflower/common-tools/web/app"
)

// newSlowHandler builds a Handler with a single blocking GET /slow route.
// It signals entered once per request and then blocks until release is closed.
func newSlowHandler(entered chan<- struct{}, release <-chan struct{}) *Handler {
	h := NewHandler(HandlerCfg{}, logger.DefaultLogger())
	NewRouterGroup(h).GET("/slow", app.HandlerFunc(func(_ context.Context, _ *app.RequestContext) {
		if entered != nil {
			entered <- struct{}{}
		}
		if release != nil {
			<-release
		}
	}))
	return h
}

func TestInflightRejectsEnterAfterDrain(t *testing.T) {
	f := newInflight()

	if err := f.drain(context.Background()); err != nil {
		t.Fatalf("drain returned error: %v", err)
	}
	if f.enter() {
		t.Fatal("enter should be rejected after drain")
	}
}

func TestDrainWaitsForInflightRequest(t *testing.T) {
	entered := make(chan struct{}, 1)
	release := make(chan struct{})
	h := newSlowHandler(entered, release)

	go h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/slow", nil))
	<-entered

	drained := make(chan error, 1)
	go func() { drained <- h.Drain(context.Background()) }()

	select {
	case <-drained:
		t.Fatal("Drain returned while a request was still in-flight")
	case <-time.After(50 * time.Millisecond):
	}

	close(release)

	select {
	case err := <-drained:
		if err != nil {
			t.Fatalf("Drain returned error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Drain did not return after the request finished")
	}
}

func TestDrainTimesOut(t *testing.T) {
	entered := make(chan struct{}, 1)
	release := make(chan struct{})
	h := newSlowHandler(entered, release)

	go h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/slow", nil))
	<-entered

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	if err := h.Drain(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Drain error = %v, want context.DeadlineExceeded", err)
	}

	close(release)
}

func TestDrainRejectsNewRequests(t *testing.T) {
	h := newSlowHandler(nil, nil)

	if err := h.Drain(context.Background()); err != nil {
		t.Fatalf("Drain returned error: %v", err)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/slow", nil))

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}
