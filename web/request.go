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
