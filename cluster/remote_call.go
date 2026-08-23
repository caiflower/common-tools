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

package cluster

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/caiflower/common-tools/pkg/tools"
)

const (
	remoteCall        = "cluster:func_call:"
	defaultTimeout    = 3 * time.Second
	cacheTTLExtension = 5 * time.Second
)

var ErrResultNotReady = fmt.Errorf("remote call result not ready")

type FuncSpec struct {
	mu                    sync.RWMutex
	traceID               string
	uuid                  string
	nodeName              string
	funcName              string
	param                 interface{}
	sync                  bool
	result                interface{}
	err                   error
	timeout               time.Duration
	ctx                   context.Context
	cancel                context.CancelFunc
	timer                 *time.Timer
	finished              bool
	ignoreClusterNotReady bool
	onFinish              func()
}

// NewFuncSpec 同步调用，timeout是同步超时时间
func NewFuncSpec(nodeName, funcName string, param interface{}, timeout time.Duration) *FuncSpec {
	spec := NewAsyncFuncSpec(nodeName, funcName, param, timeout)
	spec.sync = true
	spec.ctx, spec.cancel = context.WithCancel(context.Background())
	return spec
}

// NewAsyncFuncSpec 异步调用，timeout为超时时间
func NewAsyncFuncSpec(nodeName, funcName string, param interface{}, timeout time.Duration) *FuncSpec {
	if timeout.Seconds() <= 0 {
		timeout = defaultTimeout
	}
	f := &FuncSpec{
		uuid:     tools.UUID(),
		nodeName: nodeName,
		funcName: funcName,
		param:    param,
		timeout:  timeout,
	}
	f.traceID = f.uuid
	return f
}

func (fs *FuncSpec) startTimer() {
	if fs.timer == nil {
		fs.timer = time.AfterFunc(fs.timeout, func() {
			fs.setResult(nil, fmt.Errorf("remote call timed out"))
		})
	}
}

func (fs *FuncSpec) SetTraceId(traceId string) *FuncSpec {
	fs.traceID = traceId
	return fs
}

func (fs *FuncSpec) GetTraceId() string {
	return fs.traceID
}

func (fs *FuncSpec) IgnoreNotReady() *FuncSpec {
	fs.ignoreClusterNotReady = true
	return fs
}

func (fs *FuncSpec) setResult(result interface{}, err error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	if fs.finished {
		return
	}
	if fs.timer != nil {
		fs.timer.Stop()
	}
	fs.result = result
	fs.err = err
	fs.finished = true
	if fs.onFinish != nil {
		fs.onFinish()
	}
	if fs.cancel != nil {
		fs.cancel()
	}
}

func (fs *FuncSpec) wait() {
	if !fs.sync {
		return
	}
	select {
	case <-fs.ctx.Done():
	case <-time.After(fs.timeout):
		fs.setResult(nil, fmt.Errorf("remote call timed out"))
	}
}

// Deprecated: Use GetResultAs[T] instead to avoid deserialization failures with remote calls.
func (fs *FuncSpec) GetResult() (interface{}, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()
	if !fs.finished {
		return nil, ErrResultNotReady
	}
	return fs.result, fs.err
}

func GetResultAs[T any](fs *FuncSpec) (T, error) {
	var zero T
	fs.mu.RLock()
	defer fs.mu.RUnlock()
	if !fs.finished {
		return zero, ErrResultNotReady
	}
	if fs.err != nil {
		return zero, fs.err
	}
	if v, ok := fs.result.(T); ok {
		return v, nil
	}
	logger.Trace("GetResultAs: type assertion failed, falling back to marshal/unmarshal. funcName=%s, resultType=%T", fs.funcName, fs.result)
	resultBytes, marshalErr := tools.Marshal(fs.result)
	if marshalErr != nil {
		return zero, fmt.Errorf("marshal result failed: %w", marshalErr)
	}
	var target T
	if unmarshalErr := tools.Unmarshal(resultBytes, &target); unmarshalErr != nil {
		return zero, fmt.Errorf("unmarshal result failed: %w", unmarshalErr)
	}
	return target, nil
}

func CallFuncAs[T any](c ICluster, fc *FuncSpec) (T, error) {
	var zero T
	_, err := c.CallFunc(fc)
	if err != nil {
		return zero, err
	}
	if !fc.sync {
		return zero, nil
	}
	return GetResultAs[T](fc)
}
