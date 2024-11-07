package web

import "net/http"

type ApiError struct {
	Code    int    `json:"-"`
	Type    string `json:"type"`
	Message string `json:"message"`
	Error   error  `json:"-"`
}

type ErrorCode struct {
	Code int
	Type string
}

var (
	NotFound = &ErrorCode{Code: http.StatusNotFound, Type: "NotFound"}
)

func NewApiError(errCode *ErrorCode, msg string, err error) *ApiError {
	return &ApiError{
		Code:    errCode.Code,
		Type:    errCode.Type,
		Message: msg,
		Error:   err,
	}
}
