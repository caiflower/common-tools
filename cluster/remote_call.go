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
	"time"

	"github.com/caiflower/common-tools/pkg/cache"
	"github.com/caiflower/common-tools/pkg/tools"
)

const (
	remoteCall    = "remote-call/"
	defaultTimout = 3 * time.Second
)

type FuncSpec struct {
	traceId   string      //请求ID
	uuid      string      //唯一ID
	nodeName  string      //目标节点
	funcName  string      //函数名称
	param     interface{} //参数
	sync      bool        //是否同步
	result    interface{} //结果
	err       error       //错误信息
	timeout   time.Duration
	ctx       context.Context    // 上下文
	cancel    context.CancelFunc // 取消函数
	finished  bool
	attribute map[string]interface{}
}

// NewFuncSpec 同步调用，timeout是同步超时时间
func NewFuncSpec(nodeName, funcName string, param interface{}, timeout time.Duration) *FuncSpec {
	spec := NewAsyncFuncSpec(nodeName, funcName, param, timeout)
	spec.sync = true
	spec.ctx, spec.cancel = context.WithCancel(context.Background())
	return spec
}

// NewAsyncFuncSpec 异步调用，timeout + 5是等待结果返回的超时时间
func NewAsyncFuncSpec(nodeName, funcName string, param interface{}, timeout time.Duration) *FuncSpec {
	if timeout.Seconds() <= 0 {
		timeout = defaultTimout
	}
	f := &FuncSpec{
		uuid:     tools.UUID(),
		nodeName: nodeName,
		funcName: funcName,
		param:    param,
		timeout:  timeout,
	}
	f.traceId = f.uuid
	return f
}

func (fs *FuncSpec) SetTraceId(traceId string) *FuncSpec {
	fs.traceId = traceId
	return fs
}

func (fs *FuncSpec) GetTraceId() string {
	return fs.traceId
}

func (fs *FuncSpec) setResult(result interface{}, err error) {
	fs.result = result
	fs.err = err
	fs.finished = true
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
		fs.setResult(nil, fmt.Errorf("remote call timed out")) //超时
	}
}

func (fs *FuncSpec) GetResult() (interface{}, error) {
	if _, e := cache.LocalCache.Get(remoteCall + fs.uuid); !e && !fs.finished {
		return nil, fmt.Errorf("remote call timed out")
	} else {
		return fs.result, fs.err
	}
}

func (fs *FuncSpec) SetAttribute(key string, v interface{}) {
	fs.attribute[key] = v
}

func (fs *FuncSpec) GetAttribute(key string) interface{} {
	return fs.attribute[key]
}
