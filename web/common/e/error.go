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

package e

import (
	"net/http"

	"github.com/caiflower/common-tools/pkg/tools"
)

type ApiError interface {
	GetCode() int
	GetType() string
	GetMessage() string
	GetCause() error
}

type Error = apiError

type apiError struct {
	Code    int
	Type    string
	Message string
	Cause   error `json:"-"`
}

func (e *apiError) GetCode() int {
	return e.Code
}

func (e *apiError) GetType() string {
	return e.Type
}

func (e *apiError) GetMessage() string {
	return e.Message
}

func (e *apiError) GetCause() error {
	return e.Cause
}

func (e *apiError) Error() string {
	return tools.ToJson(e.Cause)
}

type ErrorCode struct {
	Code int
	Type string
}

var (
	NotFound        = &ErrorCode{Code: http.StatusNotFound, Type: "NotFound"}
	NotAcceptable   = &ErrorCode{Code: http.StatusNotAcceptable, Type: "NotAcceptable"}
	Unknown         = &ErrorCode{Code: http.StatusInternalServerError, Type: "Unknown"}
	Internal        = &ErrorCode{Code: http.StatusInternalServerError, Type: "InternalError"}
	TooManyRequests = &ErrorCode{Code: http.StatusTooManyRequests, Type: "TooManyRequests"}

	InvalidArgument = &ErrorCode{Code: http.StatusBadRequest, Type: "InvalidArgument"}
)

func NewApiError(errCode *ErrorCode, msg string, err error) *Error {
	return &apiError{
		Code:    errCode.Code,
		Type:    errCode.Type,
		Message: msg,
		Cause:   err,
	}
}

func NewInternalError(err error) *Error {
	return &apiError{
		Code:    Internal.Code,
		Type:    Internal.Type,
		Message: "internal server error",
		Cause:   err,
	}
}
