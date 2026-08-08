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

package cli

import (
	"github.com/caiflower/common-tools/global"
	"github.com/caiflower/common-tools/web"
)

// ResourceManager is the subset of global.DefaultResourceManger used by serve.
type ResourceManager interface {
	AddDaemonWithOrder(global.DaemonResource, int)
	Signal()
}

type options struct {
	name       string
	manager    ResourceManager
	serveOrder int
	runner     commandRunner
}

type Option func(*options)

func WithName(name string) Option {
	return func(o *options) {
		o.name = name
	}
}

func WithResourceManager(manager ResourceManager) Option {
	return func(o *options) {
		o.manager = manager
	}
}

func WithServeOrder(order int) Option {
	return func(o *options) {
		o.serveOrder = order
	}
}

func WithRunner(runner commandRunner) Option {
	return func(o *options) {
		o.runner = runner
	}
}

func defaultOptions(engine *web.Engine, opts ...Option) *options {
	o := &options{
		name:       engine.Name(),
		manager:    global.DefaultResourceManger,
		serveOrder: 500,
	}
	for _, opt := range opts {
		opt(o)
	}
	return o
}
