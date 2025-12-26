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
	"strings"

	"github.com/caiflower/common-tools/pkg/basic"
	"github.com/caiflower/common-tools/pkg/tools"
	"github.com/caiflower/common-tools/pkg/tools/bytesconv"
	"github.com/caiflower/common-tools/web/common/compress"
	"github.com/caiflower/common-tools/web/common/e"
	"github.com/caiflower/common-tools/web/common/reflectx"
	"github.com/caiflower/common-tools/web/common/webctx"
)

var (
	pathByte   = []byte("path")
	pathStr    = "path"
	paramByte  = []byte("query")
	paramStr   = "query"
	headerByte = []byte("header")
	headerStr  = "header"
)

func validArgs(arg interface{}) e.ApiError {
	elem := reflect.TypeOf(arg).Elem()
	pkgPath := elem.PkgPath()
	if err := tools.DoTagFunc(arg, []tools.FnObj{
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

func setArgsOptimized(ctx *webctx.RequestCtx, arg interface{}, argInfo *basic.ArgInfo) e.ApiError {
	var (
		method = ctx.GetMethod()
	)

	if argInfo == nil {
		return e.NewApiError(e.Internal, "arg info not found", nil)
	}

	builder := basic.NewArgBuilder()

	// body
	if ctx.GetContentLength() != 0 && (!ctx.IsRestful() || method == http.MethodPost || method == http.MethodPut || method == http.MethodDelete || method == http.MethodPatch) {
		bytes := getBody(ctx)
		encoding := ctx.GetContentEncoding()
		if strings.Contains(encoding, "gzip") {
			tmpBytes, err := compress.AppendGunzipBytes(nil, bytes)
			if err != nil {
				return e.NewApiError(e.InvalidArgument, fmt.Sprintf("parse param failed. ungzip failed. %s", err.Error()), nil)
			}

			bytes = tmpBytes
		} else if strings.Contains(encoding, "br") {
			tmpBytes, err := tools.UnBrotil(bytes)
			if err != nil {
				return e.NewApiError(e.InvalidArgument, fmt.Sprintf("parse param failed. unbr failed. %s", err.Error()), nil)
			}

			bytes = tmpBytes
		}

		if err := tools.Unmarshal(bytes, arg); err != nil {
			err = json.Unmarshal(bytes, arg)
			var typeError *json.UnmarshalTypeError
			if errors.As(err, &typeError) {
				return e.NewApiError(e.InvalidArgument, fmt.Sprintf("Malformed %s type '%s'", reflect.TypeOf(arg).Elem().Name()+"."+typeError.Field, typeError.Value), err)
			}

			return e.NewApiError(e.InvalidArgument, fmt.Sprintf("%s", err.Error()), err)
		}
	}

	structVal := reflect.ValueOf(arg)
	if structVal.Kind() == reflect.Ptr {
		structVal = structVal.Elem()
	}

	// params
	if (!ctx.IsRestful() || method == http.MethodGet) && argInfo.HasTagName(paramStr) {
		ctx.Request.URI().QueryArgs().VisitAll(func(key, value []byte) {
			_ = builder.WithOption(basic.WithTag(paramByte, key)).SetFieldValueUsingIndex(structVal, value, argInfo)
		})
	}

	// paths
	if ctx.IsRestful() && len(ctx.Paths) > 0 && argInfo.HasTagName(pathStr) {
		for _, path := range ctx.Paths {
			_ = builder.WithOption(basic.WithTag(pathByte, bytesconv.S2b(path.Key))).SetFieldValueUsingIndex(structVal, bytesconv.S2b(path.Value), argInfo)
		}
	}

	// header
	if argInfo.HasTagName(headerStr) {
		ctx.Request.Header.VisitAll(func(key, value []byte) {
			_ = builder.WithOption(basic.WithTag(headerByte, key)).SetFieldValueUsingIndex(structVal, value, argInfo)
		})
	}

	return nil
}

func setArgs(ctx *webctx.RequestCtx, arg interface{}, webContext *webctx.Context) e.ApiError {
	var (
		contentLen = ctx.GetContentLength()
		method     = ctx.GetMethod()
	)

	// set context
	indirect := reflect.Indirect(reflect.ValueOf(arg))
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
		encoding := ctx.GetContentEncoding()
		if strings.Contains(encoding, "gzip") {
			tmpBytes, err := compress.AppendGunzipBytes(nil, bytes)
			if err != nil {
				return e.NewApiError(e.InvalidArgument, fmt.Sprintf("parse param failed. ungzip failed. %s", err.Error()), nil)
			}

			bytes = tmpBytes
		} else if strings.Contains(encoding, "br") {
			tmpBytes, err := tools.UnBrotil(bytes)
			if err != nil {
				return e.NewApiError(e.InvalidArgument, fmt.Sprintf("parse param failed. unbr failed. %s", err.Error()), nil)
			}

			bytes = tmpBytes
		}

		if err := tools.Unmarshal(bytes, arg); err != nil {
			err = json.Unmarshal(bytes, arg)
			var typeError *json.UnmarshalTypeError
			if errors.As(err, &typeError) {
				return e.NewApiError(e.InvalidArgument, fmt.Sprintf("Malformed %s type '%s'", reflect.TypeOf(arg).Elem().Name()+"."+typeError.Field, typeError.Value), err)
			}

			return e.NewApiError(e.InvalidArgument, fmt.Sprintf("%s", err.Error()), err)
		}
	}

	params := ctx.GetParams()
	if len(params) >= 1 && ctx.IsRestful() || len(params) >= 2 {
		fnObjs = append(fnObjs, tools.FnObj{
			Fn:   reflectx.SetQuery,
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
	if ctx.HttpRequest != nil {
		fnObjs = append(fnObjs, tools.FnObj{
			Fn:   reflectx.SetHeader,
			Data: ctx.HttpRequest.Header,
		})
	}

	// set default
	//fnObjs = append(fnObjs, tools.FnObj{
	//	Fn: tools.SetDefaultValueIfNil,
	//})

	if err := tools.DoTagFunc(arg, fnObjs); err != nil {
		return e.NewInternalError(err)
	}

	return nil
}

func getBody(ctx *webctx.RequestCtx) (body []byte) {
	if ctx.HttpRequest != nil {
		body, _ = io.ReadAll(ctx.HttpRequest.Body)
	} else {
		body = ctx.Request.Body()
	}
	return
}
