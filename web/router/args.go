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

package router

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"

	"github.com/caiflower/common-tools/pkg/tools"
	"github.com/caiflower/common-tools/web/common/e"
	"github.com/caiflower/common-tools/web/common/reflectx"
	"github.com/caiflower/common-tools/web/common/webctx"
)

func validArgs(ctx *webctx.RequestCtx) e.ApiError {
	method := ctx.TargetMethod
	if !method.HasArgs() {
		return nil
	}

	elem := reflect.TypeOf(ctx.Args[0].Interface()).Elem()
	pkgPath := elem.PkgPath()
	if err := tools.DoTagFunc(ctx.Args[0].Interface(), []tools.FnObj{
		{
			Fn: reflectx.CheckParam,
			Data: reflectx.ValidObject{
				PkgPath:   pkgPath,
				FiledName: elem.Name(),
			}},
	}); err != nil {
		return e.NewApiError(e.InvalidArgument, err.Error(), nil)
	}

	return nil
}

func setArgs(ctx *webctx.RequestCtx, webContext *webctx.Context) e.ApiError {
	var (
		targetMethod = ctx.TargetMethod
		contentLen   = ctx.GetContentLength()
		method       = ctx.GetMethod()
	)

	if !targetMethod.HasArgs() {
		return nil
	}

	arg := targetMethod.GetArgs()[0]
	switch arg.Kind() {
	case reflect.Ptr:
		ctx.Args = append(ctx.Args, reflect.New(arg.Elem()))
	case reflect.Struct:
		ctx.Args = append(ctx.Args, reflect.New(arg))
	default:
		return e.NewApiError(e.InvalidArgument, fmt.Sprintf("parse param failed. not support kind %s", arg.Kind()), nil)
	}

	// set context
	indirect := reflect.Indirect(reflect.ValueOf(ctx.Args[0].Interface()))
	for i := 0; i < indirect.NumField(); i++ {
		field := indirect.Field(i)
		if field.Type().AssignableTo(assignableWebContextElem) {
			field.Set(reflect.ValueOf(*webContext))
			break
		} else if field.Type().AssignableTo(assignableWebContext) {
			field.Set(reflect.ValueOf(webContext))
			break
		}
	}

	fnObjs := make([]tools.FnObj, 0, 10)

	if contentLen != 0 && (!ctx.IsRestful() || method == http.MethodPost || method == http.MethodPut || method == http.MethodDelete || method == http.MethodPatch) {
		bytes := getBody(ctx)
		if isGzip(ctx) {
			tmpBytes, err := tools.Gunzip(bytes)
			if err != nil {
				return e.NewApiError(e.InvalidArgument, fmt.Sprintf("parse param failed. ungzip failed. %s", err.Error()), nil)
			}

			bytes = tmpBytes
		} else if isBr(ctx) {
			tmpBytes, err := tools.UnBrotil(bytes)
			if err != nil {
				return e.NewApiError(e.InvalidArgument, fmt.Sprintf("parse param failed. unbr failed. %s", err.Error()), nil)
			}

			bytes = tmpBytes
		}

		if err := tools.Unmarshal(bytes, ctx.Args[0].Interface()); err != nil {
			err = json.Unmarshal(bytes, ctx.Args[0].Interface())
			var typeError *json.UnmarshalTypeError
			if errors.As(err, &typeError) {
				return e.NewApiError(e.InvalidArgument, fmt.Sprintf("Malformed %s type '%s'", reflect.TypeOf(ctx.Args[0].Interface()).Elem().Name()+"."+typeError.Field, typeError.Value), err)
			}

			return e.NewApiError(e.InvalidArgument, fmt.Sprintf("%s", err.Error()), err)
		}
	}

	params := ctx.GetParams()
	if (len(params) > 1 && !ctx.IsRestful()) && (len(params) > 2 || method == http.MethodGet) {
		fnObjs = append(fnObjs, tools.FnObj{
			Fn:   reflectx.SetParam,
			Data: params,
		})
	}

	// set paths
	if ctx.IsRestful() && len(ctx.Paths) > 0 {
		fnObjs = append(fnObjs, tools.FnObj{
			Fn:   reflectx.SetPath,
			Data: ctx.Paths,
		})
	}

	// set header
	if ctx.Request != nil {
		fnObjs = append(fnObjs, tools.FnObj{
			Fn:   reflectx.SetHeader,
			Data: ctx.Request.Header,
		})
	}

	// set default
	//fnObjs = append(fnObjs, tools.FnObj{
	//	Fn: tools.SetDefaultValueIfNil,
	//})

	if err := tools.DoTagFunc(ctx.Args[0].Interface(), fnObjs); err != nil {
		return e.NewInternalError(err)
	}

	return nil
}

func getBody(ctx *webctx.RequestCtx) (body []byte) {
	if ctx.Request != nil {
		body, _ = io.ReadAll(ctx.Request.Body)
	} else {
		body = ctx.HttpRequest.Body()
	}
	return
}
