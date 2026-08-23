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

package otel

import (
	"context"
	"io"
	"strings"
	"testing"

	golocalv1 "github.com/caiflower/common-tools/pkg/golocal/v1"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel"
	otelcodes "go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"google.golang.org/grpc"
	grpccodes "google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	grpcTestTraceID      = "0123456789abcdef0123456789abcdef"
	grpcTestParentSpanID = "fedcba9876543210"
	grpcTestTraceparent  = "00-0123456789abcdef0123456789abcdef-fedcba9876543210-01"
)

func setupGRPCTest(t *testing.T) *tracetest.SpanRecorder {
	t.Helper()

	recorder := setupRecordingWebClient(t)
	oldPropagator := otel.GetTextMapPropagator()
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	t.Cleanup(func() { otel.SetTextMapPropagator(oldPropagator) })
	return recorder
}

func TestGRPCUnaryClientInterceptor(t *testing.T) {
	recorder := setupGRPCTest(t)
	golocalv1.PutTraceID(grpcTestTraceID)
	defer golocalv1.Clean()

	interceptor := NewGRPCUnaryClientInterceptor()
	var outgoing metadata.MD
	err := interceptor(context.Background(), "/pkg.Service/Method", nil, nil, nil, func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
		outgoing, _ = metadata.FromOutgoingContext(ctx)
		return nil
	})

	assert.NoError(t, err)
	spans := recorder.Ended()
	assert.Len(t, spans, 1)
	if len(spans) != 1 {
		return
	}
	assert.Equal(t, "pkg.Service/Method", spans[0].Name())
	assert.Equal(t, grpcTestTraceID, spans[0].SpanContext().TraceID().String())

	attrs := spanAttributes(t, recorder)
	assert.Equal(t, "grpc", attrs["rpc.system"])
	assert.Equal(t, "pkg.Service", attrs["rpc.service"])
	assert.Equal(t, "Method", attrs["rpc.method"])
	assert.Equal(t, "0", attrs["rpc.grpc.status_code"])

	traceparents := outgoing.Get("traceparent")
	assert.NotEmpty(t, traceparents)
	assert.True(t, strings.HasPrefix(traceparents[0], "00-"+grpcTestTraceID+"-"))
}

func TestGRPCUnaryClientInterceptorError(t *testing.T) {
	recorder := setupGRPCTest(t)
	golocalv1.PutTraceID(grpcTestTraceID)
	defer golocalv1.Clean()

	interceptor := NewGRPCUnaryClientInterceptor()
	err := interceptor(context.Background(), "/pkg.Service/Method", nil, nil, nil, func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
		return status.Error(grpccodes.Internal, "boom")
	})

	assert.Error(t, err)
	spans := recorder.Ended()
	assert.Len(t, spans, 1)
	if len(spans) != 1 {
		return
	}
	attrs := spanAttributes(t, recorder)
	assert.Equal(t, "13", attrs["rpc.grpc.status_code"])
	assert.Equal(t, otelcodes.Error, spans[0].Status().Code)
}

func TestGRPCUnaryServerInterceptor(t *testing.T) {
	recorder := setupGRPCTest(t)
	md := metadata.Pairs("traceparent", grpcTestTraceparent)
	ctx := metadata.NewIncomingContext(context.Background(), md)

	interceptor := NewGRPCUnaryServerInterceptor()
	var handlerTraceID string
	resp, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/pkg.Service/Method"}, func(ctx context.Context, req interface{}) (interface{}, error) {
		handlerTraceID = golocalv1.GetTraceID()
		return nil, status.Error(grpccodes.NotFound, "missing")
	})

	assert.Nil(t, resp)
	assert.Error(t, err)
	assert.Equal(t, grpcTestTraceID, handlerTraceID)

	spans := recorder.Ended()
	assert.Len(t, spans, 1)
	if len(spans) != 1 {
		return
	}
	assert.Equal(t, grpcTestTraceID, spans[0].SpanContext().TraceID().String())
	assert.Equal(t, grpcTestParentSpanID, spans[0].Parent().SpanID().String())

	attrs := spanAttributes(t, recorder)
	assert.Equal(t, "grpc", attrs["rpc.system"])
	assert.Equal(t, "pkg.Service", attrs["rpc.service"])
	assert.Equal(t, "Method", attrs["rpc.method"])
	assert.Equal(t, "5", attrs["rpc.grpc.status_code"])
	assert.Equal(t, otelcodes.Error, spans[0].Status().Code)
}

func TestGRPCUnaryServerInterceptorPanic(t *testing.T) {
	recorder := setupGRPCTest(t)

	interceptor := NewGRPCUnaryServerInterceptor()
	assert.Panics(t, func() {
		_, _ = interceptor(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/pkg.Service/Method"}, func(ctx context.Context, req interface{}) (interface{}, error) {
			panic("boom")
		})
	})

	spans := recorder.Ended()
	assert.Len(t, spans, 1)
	if len(spans) != 1 {
		return
	}
	assert.Equal(t, otelcodes.Error, spans[0].Status().Code)
}

type fakeGRPCServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s *fakeGRPCServerStream) Context() context.Context {
	return s.ctx
}

func TestGRPCStreamServerInterceptor(t *testing.T) {
	recorder := setupGRPCTest(t)
	md := metadata.Pairs("traceparent", grpcTestTraceparent)
	stream := &fakeGRPCServerStream{ctx: metadata.NewIncomingContext(context.Background(), md)}

	interceptor := NewGRPCStreamServerInterceptor()
	var handlerTraceID string
	err := interceptor(nil, stream, &grpc.StreamServerInfo{FullMethod: "/pkg.Service/Stream"}, func(srv interface{}, ss grpc.ServerStream) error {
		handlerTraceID = golocalv1.GetTraceID()
		return status.Error(grpccodes.Aborted, "boom")
	})

	assert.Error(t, err)
	assert.Equal(t, grpcTestTraceID, handlerTraceID)

	spans := recorder.Ended()
	assert.Len(t, spans, 1)
	if len(spans) != 1 {
		return
	}
	assert.Equal(t, grpcTestParentSpanID, spans[0].Parent().SpanID().String())
	attrs := spanAttributes(t, recorder)
	assert.Equal(t, "Stream", attrs["rpc.method"])
	assert.Equal(t, "10", attrs["rpc.grpc.status_code"])
	assert.Equal(t, otelcodes.Error, spans[0].Status().Code)
}

type fakeGRPCClientStream struct {
	ctx     context.Context
	recvErr error
}

func (s *fakeGRPCClientStream) Header() (metadata.MD, error) {
	return nil, nil
}

func (s *fakeGRPCClientStream) Trailer() metadata.MD {
	return nil
}

func (s *fakeGRPCClientStream) CloseSend() error {
	return nil
}

func (s *fakeGRPCClientStream) Context() context.Context {
	return s.ctx
}

func (s *fakeGRPCClientStream) SendMsg(m interface{}) error {
	return nil
}

func (s *fakeGRPCClientStream) RecvMsg(m interface{}) error {
	return s.recvErr
}

func TestGRPCStreamClientInterceptor(t *testing.T) {
	recorder := setupGRPCTest(t)
	golocalv1.PutTraceID(grpcTestTraceID)
	defer golocalv1.Clean()

	interceptor := NewGRPCStreamClientInterceptor()
	var outgoing metadata.MD
	stream, err := interceptor(context.Background(), &grpc.StreamDesc{}, nil, "/pkg.Service/Stream", func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		outgoing, _ = metadata.FromOutgoingContext(ctx)
		return &fakeGRPCClientStream{ctx: ctx, recvErr: io.EOF}, nil
	})

	assert.NoError(t, err)
	assert.NotEmpty(t, outgoing.Get("traceparent"))
	assert.ErrorIs(t, stream.RecvMsg(nil), io.EOF)

	spans := recorder.Ended()
	assert.Len(t, spans, 1)
	if len(spans) != 1 {
		return
	}
	attrs := spanAttributes(t, recorder)
	assert.Equal(t, "Stream", attrs["rpc.method"])
	assert.Equal(t, "0", attrs["rpc.grpc.status_code"])
	assert.NotEqual(t, otelcodes.Error, spans[0].Status().Code)
}

func TestGRPCStreamClientInterceptorError(t *testing.T) {
	recorder := setupGRPCTest(t)
	golocalv1.PutTraceID(grpcTestTraceID)
	defer golocalv1.Clean()

	interceptor := NewGRPCStreamClientInterceptor()
	stream, err := interceptor(context.Background(), &grpc.StreamDesc{}, nil, "/pkg.Service/Stream", func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		return &fakeGRPCClientStream{ctx: ctx, recvErr: status.Error(grpccodes.DataLoss, "lost")}, nil
	})

	assert.NoError(t, err)
	assert.Error(t, stream.RecvMsg(nil))

	spans := recorder.Ended()
	assert.Len(t, spans, 1)
	if len(spans) != 1 {
		return
	}
	attrs := spanAttributes(t, recorder)
	assert.Equal(t, "15", attrs["rpc.grpc.status_code"])
	assert.Equal(t, otelcodes.Error, spans[0].Status().Code)
}

func TestGRPCInterceptorsDisabled(t *testing.T) {
	oldClient := DefaultClient
	DefaultClient = &client{}
	defer func() { DefaultClient = oldClient }()

	serverCalled := false
	server := NewGRPCUnaryServerInterceptor()
	_, err := server(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/pkg.Service/Method"}, func(ctx context.Context, req interface{}) (interface{}, error) {
		serverCalled = true
		return nil, nil
	})
	assert.NoError(t, err)
	assert.True(t, serverCalled)

	clientCalled := false
	client := NewGRPCUnaryClientInterceptor()
	err = client(context.Background(), "/pkg.Service/Method", nil, nil, nil, func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
		clientCalled = true
		return nil
	})
	assert.NoError(t, err)
	assert.True(t, clientCalled)
}

func TestGRPCUnaryServerInterceptorSkipMethods(t *testing.T) {
	recorder := setupGRPCTest(t)

	interceptor := NewGRPCUnaryServerInterceptor(
		WithGRPCSkipMethods("/pkg.Service/Health", "pkg.Service/Skip"),
		WithGRPCSkipServicePrefixes("pkg.Health"),
	)
	handlerCalled := false
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		handlerCalled = true
		return nil, nil
	}

	for _, method := range []string{"/pkg.Service/Health", "/pkg.Service/Skip", "/pkg.HealthService/Check"} {
		_, err := interceptor(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: method}, handler)
		assert.NoError(t, err)
	}
	assert.True(t, handlerCalled)
	assert.Empty(t, recorder.Ended())

	_, err := interceptor(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/pkg.Service/Normal"}, handler)
	assert.NoError(t, err)
	assert.Len(t, recorder.Ended(), 1)
}

func TestGRPCStreamServerInterceptorSkipMethods(t *testing.T) {
	recorder := setupGRPCTest(t)
	stream := &fakeGRPCServerStream{ctx: context.Background()}

	interceptor := NewGRPCStreamServerInterceptor(WithGRPCSkipMethods("/pkg.Service/Stream"))
	handlerCalled := false
	err := interceptor(nil, stream, &grpc.StreamServerInfo{FullMethod: "/pkg.Service/Stream"}, func(srv interface{}, ss grpc.ServerStream) error {
		handlerCalled = true
		return nil
	})

	assert.NoError(t, err)
	assert.True(t, handlerCalled)
	assert.Empty(t, recorder.Ended())
}

func TestGRPCUnaryClientInterceptorSkipMethods(t *testing.T) {
	recorder := setupGRPCTest(t)
	golocalv1.PutTraceID(grpcTestTraceID)
	defer golocalv1.Clean()

	interceptor := NewGRPCUnaryClientInterceptor(
		WithGRPCSkipMethods("/pkg.Service/Skip"),
		WithGRPCSkipServicePrefixes("pkg.Health"),
	)
	invokerCalled := false
	invoker := func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
		invokerCalled = true
		return nil
	}

	err := interceptor(context.Background(), "/pkg.Service/Skip", nil, nil, nil, invoker)
	assert.NoError(t, err)
	err = interceptor(context.Background(), "/pkg.HealthService/Check", nil, nil, nil, invoker)
	assert.NoError(t, err)
	assert.True(t, invokerCalled)
	assert.Empty(t, recorder.Ended())

	md := metadata.New(nil)
	ctx := metadata.NewOutgoingContext(context.Background(), md)
	invoker = func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
		outgoing, _ := metadata.FromOutgoingContext(ctx)
		assert.NotEmpty(t, outgoing.Get("traceparent"))
		return nil
	}
	err = interceptor(ctx, "/pkg.Service/Normal", nil, nil, nil, invoker)
	assert.NoError(t, err)
	assert.Len(t, recorder.Ended(), 1)
}

func TestGRPCStreamClientInterceptorSkipMethods(t *testing.T) {
	recorder := setupGRPCTest(t)
	golocalv1.PutTraceID(grpcTestTraceID)
	defer golocalv1.Clean()

	interceptor := NewGRPCStreamClientInterceptor(WithGRPCSkipMethods("/pkg.Service/Stream"))
	streamerCalled := false
	stream, err := interceptor(context.Background(), &grpc.StreamDesc{}, nil, "/pkg.Service/Stream", func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		streamerCalled = true
		return &fakeGRPCClientStream{ctx: ctx, recvErr: io.EOF}, nil
	})

	assert.NoError(t, err)
	assert.True(t, streamerCalled)
	assert.ErrorIs(t, stream.RecvMsg(nil), io.EOF)
	assert.Empty(t, recorder.Ended())
}
