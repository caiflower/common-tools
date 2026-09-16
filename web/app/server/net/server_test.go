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

package net

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/caiflower/common-tools/global"
	"github.com/caiflower/common-tools/web/app"
	"github.com/caiflower/common-tools/web/router"
)

func TestHttpServerClosesFirst(t *testing.T) {
	s := NewHttpServer(NormalConfig{})
	if got := s.Order(); got != global.OrderHTTPServer {
		t.Fatalf("Order() = %d, want %d", got, global.OrderHTTPServer)
	}
}

func TestCloseDrainsInflightRequest(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	addr := ln.Addr().String()
	entered := make(chan struct{}, 1)
	release := make(chan struct{})

	s := NewHttpServer(NormalConfig{Listener: ln})
	router.NewRouterGroup(s.Handler).GET("/slow", app.HandlerFunc(func(_ context.Context, _ *app.RequestContext) {
		entered <- struct{}{}

		<-release
	}))

	if err := s.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer s.Close()

	go func() {
		resp, err := http.Get("http://" + addr + "/slow")
		if err == nil {
			_ = resp.Body.Close()
		}
	}()

	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("request never reached the handler")
	}

	closed := make(chan struct{})
	go func() {
		s.Close()
		close(closed)
	}()

	select {
	case <-closed:
		t.Fatal("Close returned while a request was still in-flight")
	case <-time.After(50 * time.Millisecond):
	}

	close(release)

	select {
	case <-closed:
	case <-time.After(3 * time.Second):
		t.Fatal("Close did not return after the request finished")
	}
}
