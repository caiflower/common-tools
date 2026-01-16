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
	"strings"

	"github.com/caiflower/common-tools/web/common/e"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func ConvertGrpcCodeToErrorCode(s *status.Status) *e.Error {
	switch s.Code() {
	case codes.OK:
		return nil
	case codes.Canceled:
		return e.NewApiError(e.Timeout, "request canceled", s.Err())
	case codes.Unknown:
		return e.NewApiError(e.Unknown, s.Message(), s.Err())
	case codes.InvalidArgument:
		return e.NewApiError(e.InvalidArgument, s.Message(), s.Err())
	case codes.DeadlineExceeded:
		return e.NewApiError(e.Timeout, "deadline exceeded", s.Err())
	case codes.NotFound:
		return e.NewApiError(e.NotFound, s.Message(), s.Err())
	case codes.AlreadyExists:
		return e.NewApiError(e.Conflict, s.Message(), s.Err())
	case codes.PermissionDenied:
		return e.NewApiError(e.Forbidden, s.Message(), s.Err())
	case codes.ResourceExhausted:
		return e.NewApiError(e.TooManyRequests, s.Message(), s.Err())
	case codes.FailedPrecondition:
		return e.NewApiError(e.FailedPrecondition, s.Message(), s.Err())
	case codes.Aborted:
		return e.NewApiError(e.Aborted, s.Message(), s.Err())
	case codes.OutOfRange:
		return e.NewApiError(e.OutOfRange, s.Message(), s.Err())
	case codes.Unimplemented:
		return e.NewApiError(e.Unimplemented, "unimplemented", s.Err())
	case codes.Internal:
		return e.NewApiError(e.Internal, s.Message(), s.Err())
	case codes.Unavailable:
		return e.NewApiError(e.Unavailable, "service unavailable", s.Err())
	case codes.DataLoss:
		return e.NewApiError(e.DataLoss, "data loss", s.Err())
	case codes.Unauthenticated:
		return e.NewApiError(e.Unauthorized, s.Message(), s.Err())
	default:
		return e.NewInternalError(s.Err())
	}
}

// normalizeRestfulPath 将 /{param} 形式转换为 /:param，兼容原有语法
func normalizeRestfulPath(path string) string {
	if !strings.Contains(path, "{") {
		return path
	}
	var b strings.Builder
	b.Grow(len(path))
	for i := 0; i < len(path); i++ {
		c := path[i]
		if c == '{' {
			j := strings.IndexByte(path[i:], '}')
			if j == -1 || j == 1 { // 未闭合或空参数名，保持原样
				b.WriteByte(c)
				continue
			}
			name := path[i+1 : i+j]
			b.WriteByte(':')
			b.WriteString(name)
			i += j
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
}

// toSwaggerPath 将 /:id 转为 /{id} 以符合 swagger 规范
func toSwaggerPath(path string) string {
	if !strings.ContainsAny(path, ":") {
		return path
	}
	var b strings.Builder
	b.Grow(len(path) + 4)
	for i := 0; i < len(path); i++ {
		c := path[i]
		if c == ':' {
			b.WriteByte('{')
			j := i + 1
			for ; j < len(path) && path[j] != '/'; j++ {
			}
			b.WriteString(path[i+1 : j])
			b.WriteByte('}')
			i = j - 1
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
}
