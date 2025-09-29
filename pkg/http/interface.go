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

 package http

import "net/url"

type Response struct {
	StatusCode int
	Data       interface{}
}

type HttpClient interface {
	Get(requestId, url string, params map[string][]string, response *Response, header map[string]string) error
	GetJson(requestId, url string, request interface{}, response *Response, header map[string]string) error
	PostJson(requestId, url string, request interface{}, response *Response, header map[string]string) error
	PostForm(requestId, url string, form map[string]interface{}, response *Response, header map[string]string) error
	Put(requestId, url string, request interface{}, response *Response, header map[string]string) error
	Patch(requestId, url string, request interface{}, response *Response, header map[string]string) error
	Delete(requestId, url string, request interface{}, response *Response, header map[string]string) error
	SetRequestIdCallBack(func(requestId string, header map[string]string))
	AddHook(hook Hook)
	Do(method, requestId, url, contentType string, request interface{}, values url.Values, response *Response, header map[string]string) (err error)
}
