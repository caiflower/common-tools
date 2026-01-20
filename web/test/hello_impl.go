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

package webtest

import (
	"context"
	"fmt"
	"strings"

	"github.com/caiflower/common-tools/web/common/e"
)

type HelloImpl struct {
	UnimplementedIServiceServer
}

func (h *HelloImpl) Search(ctx context.Context, req *SearchRequest) (*SearchResponse, error) {
	if req.Query == "1" {
		if len(req.Hobby) >= int(req.GetPageNumber()) {
			return &SearchResponse{Code: 1, Message: req.Hobby[req.PageNumber-1]}, nil
		}

		return nil, e.NewApiError(e.InvalidArgument, fmt.Sprintf("pageNumber %v out of range", req.GetPageNumber()), nil)
	} else if req.Query == "2" {
		return &SearchResponse{Code: 1, Message: strings.Join(req.Hobby, ",")}, nil
	}
	return nil, e.NewApiError(e.OutOfRange, fmt.Sprintf("query %v is not impl", req.Query), nil)
}
