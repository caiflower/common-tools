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

 package interceptor

import (
	"github.com/caiflower/common-tools/web"
	"github.com/caiflower/common-tools/web/e"
)

type Interceptor interface {
	Before(ctx *web.Context) e.ApiError                   // 执行业务前执行
	After(ctx *web.Context, err e.ApiError) e.ApiError    // 执行业务后执行，参数err为业务返回的ApiErr信息
	OnPanic(ctx *web.Context, err interface{}) e.ApiError // 发生panic时执行
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

func (itemList ItemSort) DoInterceptor(ctx *web.Context, doTargetMethod func() e.ApiError) e.ApiError {
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
