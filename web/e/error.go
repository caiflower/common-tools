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

type apiError struct {
	Code    int    `json:"-"`
	Type    string `json:"type"`
	Message string `json:"message"`
	Cause   error  `json:"-"`
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
	NotFound      = &ErrorCode{Code: http.StatusNotFound, Type: "NotFound"}
	NotAcceptable = &ErrorCode{Code: http.StatusNotAcceptable, Type: "NotAcceptable"}
	Unknown       = &ErrorCode{Code: http.StatusInternalServerError, Type: "Unknown"}
	Internal      = &ErrorCode{Code: http.StatusInternalServerError, Type: "InternalError"}

	InvalidArgument = &ErrorCode{Code: http.StatusBadRequest, Type: "InvalidArgument"}
)

func NewApiError(errCode *ErrorCode, msg string, err error) ApiError {
	return &apiError{
		Code:    errCode.Code,
		Type:    errCode.Type,
		Message: msg,
		Cause:   err,
	}
}
