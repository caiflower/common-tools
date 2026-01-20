package e

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func ConvertErrorToErrorCode(err error) *Error {
	status, ok := status.FromError(err)
	if ok {
		return ConvertGrpcCodeToErrorCode(status)
	}
	return NewApiError(Unknown, err.Error(), nil)
}

func ConvertGrpcCodeToErrorCode(s *status.Status) *Error {
	switch s.Code() {
	case codes.OK:
		return nil
	case codes.Canceled:
		return NewApiError(Timeout, "request canceled", s.Err())
	case codes.Unknown:
		return NewApiError(Unknown, s.Message(), s.Err())
	case codes.InvalidArgument:
		return NewApiError(InvalidArgument, s.Message(), s.Err())
	case codes.DeadlineExceeded:
		return NewApiError(Timeout, "deadline exceeded", s.Err())
	case codes.NotFound:
		return NewApiError(NotFound, s.Message(), s.Err())
	case codes.AlreadyExists:
		return NewApiError(Conflict, s.Message(), s.Err())
	case codes.PermissionDenied:
		return NewApiError(Forbidden, s.Message(), s.Err())
	case codes.ResourceExhausted:
		return NewApiError(TooManyRequests, s.Message(), s.Err())
	case codes.FailedPrecondition:
		return NewApiError(FailedPrecondition, s.Message(), s.Err())
	case codes.Aborted:
		return NewApiError(Aborted, s.Message(), s.Err())
	case codes.OutOfRange:
		return NewApiError(OutOfRange, s.Message(), s.Err())
	case codes.Unimplemented:
		return NewApiError(Unimplemented, "unimplemented", s.Err())
	case codes.Internal:
		return NewApiError(Internal, s.Message(), s.Err())
	case codes.Unavailable:
		return NewApiError(Unavailable, "service unavailable", s.Err())
	case codes.DataLoss:
		return NewApiError(DataLoss, "data loss", s.Err())
	case codes.Unauthenticated:
		return NewApiError(Unauthorized, s.Message(), s.Err())
	default:
		return NewInternalError(s.Err())
	}
}

// ConvertErrorCodeToGrpcCode 将自定义业务错误码转换为 gRPC 标准错误码
func ConvertErrorCodeToGrpcCode(err *Error) *status.Status {
	// 空错误处理：避免访问 nil 的 Message 字段
	if err == nil {
		return status.New(codes.OK, "")
	}

	// 根据自定义错误类型反向映射 gRPC Status
	var c codes.Code
	switch err.GetType() {
	case Timeout.Type:
		// 原映射中 Timeout 对应 Canceled 和 DeadlineExceeded，优先返回 DeadlineExceeded
		c = codes.DeadlineExceeded
	case Unknown.Type:
		c = codes.Unknown
	case InvalidArgument.Type:
		c = codes.InvalidArgument
	case NotFound.Type:
		c = codes.NotFound
	case Conflict.Type:
		c = codes.AlreadyExists
	case Forbidden.Type:
		c = codes.PermissionDenied
	case TooManyRequests.Type:
		c = codes.ResourceExhausted
	case FailedPrecondition.Type:
		c = codes.FailedPrecondition
	case Aborted.Type:
		c = codes.Aborted
	case OutOfRange.Type:
		c = codes.OutOfRange
	case Unimplemented.Type:
		c = codes.Unimplemented
	case Internal.Type:
		c = codes.Internal
	case Unavailable.Type:
		c = codes.Unavailable
	case DataLoss.Type:
		c = codes.DataLoss
	case Unauthorized.Type:
		c = codes.Unauthenticated
	default:
		// 未匹配的错误码默认返回 Unknown
		c = codes.Unknown
	}

	// 返回带错误码和错误信息的 gRPC Status
	return status.New(c, err.Message)
}
