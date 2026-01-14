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
	"net/http"
	"reflect"
	"strings"

	"github.com/caiflower/common-tools/pkg/basic"
	"github.com/caiflower/common-tools/pkg/tools"
	"github.com/caiflower/common-tools/pkg/tools/bytesconv"
	"github.com/caiflower/common-tools/web/app"
	"github.com/caiflower/common-tools/web/common/compress"
	"github.com/caiflower/common-tools/web/common/e"
	"github.com/caiflower/common-tools/web/common/goai"
)

var (
	pathByte   = []byte("path")
	pathStr    = "path"
	paramByte  = []byte("query")
	paramStr   = "query"
	headerByte = []byte("header")
	headerStr  = "header"
)

var goaiInstance *goai.OpenApiV3

func setGoAIInstance(o *goai.OpenApiV3) {
	goaiInstance = o
}

func validArgs(arg interface{}) e.ApiError {
	if goaiInstance != nil {
		if err := goaiInstance.Validate(arg); err != nil {
			return e.NewApiError(e.InvalidArgument, err.Error(), nil)
		}
		return nil
	}

	// fallback: no goai instance, skip validation
	return nil
}

func setArgsOptimized(ctx *app.RequestCtx, arg interface{}, argInfo *basic.ArgInfo) e.ApiError {
	var (
		method = ctx.GetMethod()
	)

	if argInfo == nil {
		return e.NewApiError(e.Internal, "arg info not found", nil)
	}

	builder := basic.NewArgBuilder()

	// body
	if ctx.GetContentLength() != 0 && (!ctx.IsRestful() || method == http.MethodPost || method == http.MethodPut || method == http.MethodDelete || method == http.MethodPatch) {
		bytes := ctx.GetBody()
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

	// paths
	if ctx.IsRestful() && len(ctx.Paths) > 0 && argInfo.HasTagName(pathStr) {
		for _, path := range ctx.Paths {
			_ = builder.WithOption(basic.WithTag(pathByte, bytesconv.S2b(path.Key))).SetFieldValueUsingIndex(structVal, bytesconv.S2b(path.Value), argInfo)
		}
	}

	if ctx.IsNetpoll() {
		// params
		if (!ctx.IsRestful() || method == http.MethodGet) && argInfo.HasTagName(paramStr) {
			ctx.Request.URI().QueryArgs().VisitAll(func(key, value []byte) {
				_ = builder.WithOption(basic.WithTag(paramByte, key)).SetFieldValueUsingIndex(structVal, value, argInfo)
			})
		}

		// header
		if argInfo.HasTagName(headerStr) {
			ctx.Request.Header.VisitAll(func(key, value []byte) {
				_ = builder.WithOption(basic.WithTag(headerByte, key)).SetFieldValueUsingIndex(structVal, value, argInfo)
			})
		}
	} else {
		_, r := ctx.GetResponseWriterAndRequest()

		// params
		if (!ctx.IsRestful() || method == http.MethodGet) && argInfo.HasTagName(paramStr) {
			for key, values := range r.URL.Query() {
				for _, value := range values {
					_ = builder.WithOption(basic.WithTag(paramByte, bytesconv.S2b(key))).SetFieldValueUsingIndex(structVal, bytesconv.S2b(value), argInfo)
				}
			}
		}

		// header
		if argInfo.HasTagName(headerStr) {
			for key, values := range r.Header {
				for _, value := range values {
					_ = builder.WithOption(basic.WithTag(headerByte, []byte(key))).SetFieldValueUsingIndex(structVal, []byte(value), argInfo)
				}
			}
		}
	}

	return nil
}
