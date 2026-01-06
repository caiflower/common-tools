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

package suite

import (
	"sync"

	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/caiflower/common-tools/web/app"
)

// Core is the core interface that promises to be provided for the protocol layer extensions
type Core interface {
	// IsRunning Check whether engine is running or not
	IsRunning() bool
	// GetCtxPool A RequestContext pool ready for protocol server impl
	GetCtxPool() *sync.Pool
	// Serve Business logic entrance
	// After pre-read works, protocol server may call this method
	// to introduce the middlewares and handlers
	Serve(ctx *app.RequestCtx)
	GetLogger() logger.ILog
}
