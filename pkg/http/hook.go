package http

import (
	"context"
	"net/http"
)

type Hook interface {
	BeforeRequest(context.Context, *http.Request) (context.Context, error)
	AfterRequest(context.Context, *http.Request, *http.Response, error) error
}
