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
