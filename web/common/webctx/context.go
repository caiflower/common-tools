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

package webctx

import "net/http"

type RequestContext interface {
	GetData() interface{}
	IsFinish() bool
	GetPath() string
	GetParams() map[string][]string
	GetMethod() string
	GetAction() string
	GetResponseWriterAndRequest() (http.ResponseWriter, *http.Request)
	UpgradeWebsocket()
}

type Context struct {
	RequestContext
	Attributes map[string]interface{}
}

func (c *Context) Put(key string, value interface{}) {
	if c.Attributes == nil {
		c.Attributes = make(map[string]interface{})
	}
	c.Attributes[key] = value
}

func (c *Context) Get(key string) interface{} {
	if c.Attributes == nil {
		c.Attributes = make(map[string]interface{})
	}
	return c.Attributes[key]
}
