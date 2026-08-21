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

package otel

import (
	"context"
	"errors"
	"testing"

	"github.com/caiflower/common-tools/web/protocol"
	"github.com/stretchr/testify/assert"
)

func TestNewWebClientMiddleware(t *testing.T) {
	mw := NewWebClientMiddleware()
	req := protocol.NewRequest("GET", "http://example.com/test", nil)
	resp := protocol.AcquireResponse()
	defer protocol.ReleaseResponse(resp)

	called := false
	endpoint := mw(func(ctx context.Context, req *protocol.Request, resp *protocol.Response) error {
		called = true
		resp.SetStatusCode(200)
		return nil
	})

	err := endpoint(context.Background(), req, resp)
	assert.NoError(t, err)
	assert.True(t, called)
}

func TestNewWebClientMiddlewareError(t *testing.T) {
	mw := NewWebClientMiddleware()
	req := protocol.NewRequest("GET", "http://example.com/test", nil)

	endpoint := mw(func(ctx context.Context, req *protocol.Request, resp *protocol.Response) error {
		return errors.New("boom")
	})

	err := endpoint(context.Background(), req, nil)
	assert.EqualError(t, err, "boom")
}

func TestNewWebClientMiddlewarePanic(t *testing.T) {
	mw := NewWebClientMiddleware()
	req := protocol.NewRequest("GET", "http://example.com/test", nil)

	endpoint := mw(func(ctx context.Context, req *protocol.Request, resp *protocol.Response) error {
		panic("boom")
	})

	assert.Panics(t, func() {
		_ = endpoint(context.Background(), req, nil)
	})
}
