package cluster

import (
	"context"

	golocalv1 "github.com/caiflower/common-tools/pkg/golocal/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func traceIdUnaryServerInterceptor(traceIdKey string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if md, ok := metadata.FromIncomingContext(ctx); ok {
			if values := md.Get(traceIdKey); len(values) > 0 {
				golocalv1.PutTraceID(values[0])
			}
		}
		golocalv1.PutContext(ctx)
		resp, err := handler(ctx, req)
		golocalv1.Clean()
		return resp, err
	}
}

func traceIdStreamServerInterceptor(traceIdKey string) grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		ctx := ss.Context()
		if md, ok := metadata.FromIncomingContext(ctx); ok {
			if values := md.Get(traceIdKey); len(values) > 0 {
				golocalv1.PutTraceID(values[0])
			}
		}
		golocalv1.PutContext(ctx)
		err := handler(srv, ss)
		golocalv1.Clean()
		return err
	}
}

func traceIdUnaryClientInterceptor(traceIdKey string) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		traceId := golocalv1.GetTraceID()
		if traceId != "" {
			md, ok := metadata.FromOutgoingContext(ctx)
			if !ok {
				md = metadata.Pairs(traceIdKey, traceId)
			} else {
				md = md.Copy()
				md.Set(traceIdKey, traceId)
			}
			ctx = metadata.NewOutgoingContext(ctx, md)
		}
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

func traceIdStreamClientInterceptor(traceIdKey string) grpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		traceId := golocalv1.GetTraceID()
		if traceId != "" {
			md, ok := metadata.FromOutgoingContext(ctx)
			if !ok {
				md = metadata.Pairs(traceIdKey, traceId)
			} else {
				md = md.Copy()
				md.Set(traceIdKey, traceId)
			}
			ctx = metadata.NewOutgoingContext(ctx, md)
		}
		return streamer(ctx, desc, cc, method, opts...)
	}
}
