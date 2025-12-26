package webtest

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type HelloImpl struct {
	UnimplementedIServiceServer
}

func (h *HelloImpl) Search(ctx context.Context, req *SearchRequest) (*SearchResponse, error) {
	if req.Query == "1" {
		if len(req.Hobby) >= int(req.GetPageNumber()) {
			return &SearchResponse{Code: 1, Message: req.Hobby[req.PageNumber-1]}, nil
		}

		return nil, status.Errorf(codes.InvalidArgument, fmt.Sprintf("pageNumber %v out of range", req.GetPageNumber()))
	} else if req.Query == "2" {
		return &SearchResponse{Code: 1, Message: strings.Join(req.Hobby, ",")}, nil
	}
	return nil, status.Errorf(codes.OutOfRange, fmt.Sprintf("query %v is not impl", req.Query))
}
