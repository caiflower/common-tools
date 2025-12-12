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

package web

import "net/http"

type RequestContext interface {
	SetResponse(data interface{})
	GetResponse() interface{}
	IsFinish() bool
	GetPath() string
	GetPathParams() map[string]string
	GetParams() map[string][]string
	GetMethod() string
	GetAction() string
	GetVersion() string
	GetResponseWriterAndRequest() (http.ResponseWriter, *http.Request)
	UpgradeWebsocket()
}

type Context struct {
	RequestContext
	Attributes map[string]interface{}
}

func (c *Context) Put(key string, value interface{}) {
	c.Attributes[key] = value
}

func (c *Context) Get(key string) interface{} {
	return c.Attributes[key]
}
