package interceptor

import (
	"github.com/caiflower/common-tools/web/e"
)

type Context struct {
	Ctx        CtxInterface
	Attributes map[string]interface{}
}

type CtxInterface interface {
	SetResponse(data interface{})
	IsFinish() bool
	GetPath() string
	GetPathParam() map[string]string
	GetParams() map[string][]string
}

func (c *Context) Put(key string, value interface{}) {
	c.Attributes[key] = value
}

func (c *Context) Get(key string) interface{} {
	return c.Attributes[key]
}

func (c *Context) SetResponse(data interface{}) {
	c.Ctx.SetResponse(data)
}

func (c *Context) IsFinish() bool {
	return c.Ctx.IsFinish()
}

func (c *Context) GetPath() string {
	return c.Ctx.GetPath()
}
func (c *Context) GetPathParam() map[string]string {
	return c.Ctx.GetPathParam()
}
func (c *Context) GetParams() map[string][]string {
	return c.Ctx.GetParams()
}

type Interceptor interface {
	Before(ctx *Context) e.ApiError                // 执行业务前执行
	After(ctx *Context, err e.ApiError) e.ApiError // 执行业务后执行，参数err为业务返回的ApiErr信息
	OnPanic(ctx *Context) e.ApiError               // 发生panic时执行
}

type ItemSort []Item

type Item struct {
	Interceptor Interceptor
	Order       int
}

func (itemList ItemSort) Len() int {
	return len([]Item(itemList))
}

func (itemList ItemSort) Less(i, j int) bool {
	return itemList[i].Order < itemList[j].Order
}

func (itemList ItemSort) Swap(i, j int) {
	itemList[i], itemList[j] = itemList[j], itemList[i]
}

func (itemList ItemSort) DoInterceptor(ctx *Context, doTargetMethod func() e.ApiError) e.ApiError {
	// Before
	for _, v := range itemList {
		apiErr := v.Interceptor.Before(ctx)
		if apiErr != nil {
			return apiErr
		}

		if ctx.IsFinish() {
			return nil
		}
	}

	// 执行目标方法
	err := doTargetMethod()

	// After
	for _, v := range itemList {
		apiErr := v.Interceptor.After(ctx, err)
		if apiErr != nil {
			return apiErr
		}
	}

	return err
}
