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
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	golocalv1 "github.com/caiflower/common-tools/pkg/golocal/v1"
	"github.com/caiflower/common-tools/pkg/logger"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.10.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	grpccodes "google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const grpcTracerName = "google.golang.org/grpc"

// GRPCOption configures how gRPC calls are traced.
type GRPCOption func(*grpcTraceOptions)

type grpcTraceOptions struct {
	skipMethods    map[string]struct{}
	skipServicePfx []string
}

func defaultGRPCTraceOptions() *grpcTraceOptions {
	return &grpcTraceOptions{
		skipMethods: make(map[string]struct{}),
	}
}

// WithGRPCSkipMethods skips tracing for the given full method paths, for
// example "/pkg.Service/Check". The leading slash is optional.
func WithGRPCSkipMethods(methods ...string) GRPCOption {
	return func(options *grpcTraceOptions) {
		for _, method := range methods {
			if method = normalizeGRPCMethod(method); method != "" {
				options.skipMethods[method] = struct{}{}
			}
		}
	}
}

// WithGRPCSkipServicePrefixes skips tracing for every method whose service
// name starts with one of the given prefixes, for example "pkg.Health".
func WithGRPCSkipServicePrefixes(prefixes ...string) GRPCOption {
	return func(options *grpcTraceOptions) {
		for _, prefix := range prefixes {
			if prefix = strings.TrimSpace(prefix); prefix != "" {
				options.skipServicePfx = append(options.skipServicePfx, prefix)
			}
		}
	}
}

// NewGRPCUnaryServerInterceptor returns a grpc.UnaryServerInterceptor that
// extracts W3C trace context from incoming metadata and records a server span.
func NewGRPCUnaryServerInterceptor(options ...GRPCOption) grpc.UnaryServerInterceptor {
	traceOptions := defaultGRPCTraceOptions()
	for _, option := range options {
		option(traceOptions)
	}

	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		if !IsEnabled() || traceOptions.shouldSkipGRPCMethod(info.FullMethod) {
			return handler(ctx, req)
		}

		ctx = extractGRPCContext(ctx)
		ctx, span := startGRPCSpan(ctx, info.FullMethod, trace.SpanKindServer)
		setGRPCAttrs(span, info.FullMethod)
		setGolocalGRPC(ctx, span)

		var recovered interface{}
		defer func() {
			if r := recover(); r != nil {
				recovered = r
				err = fmt.Errorf("grpc handler panic: %v", recovered)
			}
			if recovered != nil {
				finishGRPCSpan(span, err)
				golocalv1.Clean()
				panic(recovered)
			}
			finishGRPCSpan(span, err)
			golocalv1.Clean()
		}()

		resp, err = handler(ctx, req)
		return
	}
}

// NewGRPCStreamServerInterceptor returns a grpc.StreamServerInterceptor that
// extracts W3C trace context from incoming metadata and records a server span.
func NewGRPCStreamServerInterceptor(options ...GRPCOption) grpc.StreamServerInterceptor {
	traceOptions := defaultGRPCTraceOptions()
	for _, option := range options {
		option(traceOptions)
	}

	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) (err error) {
		if !IsEnabled() || traceOptions.shouldSkipGRPCMethod(info.FullMethod) {
			return handler(srv, ss)
		}

		ctx := extractGRPCContext(ss.Context())
		ctx, span := startGRPCSpan(ctx, info.FullMethod, trace.SpanKindServer)
		setGRPCAttrs(span, info.FullMethod)
		setGolocalGRPC(ctx, span)

		var recovered interface{}
		defer func() {
			if r := recover(); r != nil {
				recovered = r
				err = fmt.Errorf("grpc handler panic: %v", recovered)
			}
			if recovered != nil {
				finishGRPCSpan(span, err)
				golocalv1.Clean()
				panic(recovered)
			}
			finishGRPCSpan(span, err)
			golocalv1.Clean()
		}()

		err = handler(srv, &grpcServerStream{ServerStream: ss, ctx: ctx})
		return
	}
}

// NewGRPCUnaryClientInterceptor returns a grpc.UnaryClientInterceptor that
// records a client span and injects W3C trace context into outgoing metadata.
func NewGRPCUnaryClientInterceptor(options ...GRPCOption) grpc.UnaryClientInterceptor {
	traceOptions := defaultGRPCTraceOptions()
	for _, option := range options {
		option(traceOptions)
	}

	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) (err error) {
		if !IsEnabled() || traceOptions.shouldSkipGRPCMethod(method) {
			return invoker(ctx, method, req, reply, cc, opts...)
		}

		ctx, span := startGRPCSpan(ctx, method, trace.SpanKindClient)
		setGRPCAttrs(span, method)
		if span.IsRecording() && cc != nil {
			span.SetAttributes(attribute.String("net.peer.name", cc.Target()))
		}
		ctx = injectGRPCContext(ctx)

		var recovered interface{}
		defer func() {
			if r := recover(); r != nil {
				recovered = r
				err = fmt.Errorf("grpc invoker panic: %v", recovered)
			}
			if recovered != nil {
				finishGRPCSpan(span, err)
				panic(recovered)
			}
			finishGRPCSpan(span, err)
		}()

		err = invoker(ctx, method, req, reply, cc, opts...)
		return
	}
}

// NewGRPCStreamClientInterceptor returns a grpc.StreamClientInterceptor that
// records a client span and injects W3C trace context into outgoing metadata.
// The span ends when the stream finishes normally or with an error.
func NewGRPCStreamClientInterceptor(options ...GRPCOption) grpc.StreamClientInterceptor {
	traceOptions := defaultGRPCTraceOptions()
	for _, option := range options {
		option(traceOptions)
	}

	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		if !IsEnabled() || traceOptions.shouldSkipGRPCMethod(method) {
			return streamer(ctx, desc, cc, method, opts...)
		}

		ctx, span := startGRPCSpan(ctx, method, trace.SpanKindClient)
		setGRPCAttrs(span, method)
		if span.IsRecording() && cc != nil {
			span.SetAttributes(attribute.String("net.peer.name", cc.Target()))
		}
		ctx = injectGRPCContext(ctx)

		stream, err := streamer(ctx, desc, cc, method, opts...)
		if err != nil {
			finishGRPCSpan(span, err)
			return nil, err
		}
		return &grpcClientStream{ClientStream: stream, ctx: ctx, span: span, endOnce: &sync.Once{}}, nil
	}
}

type grpcServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s *grpcServerStream) Context() context.Context {
	return s.ctx
}

type grpcClientStream struct {
	grpc.ClientStream
	ctx     context.Context
	span    trace.Span
	endOnce *sync.Once
}

func (s *grpcClientStream) Context() context.Context {
	return s.ctx
}

func (s *grpcClientStream) Header() (metadata.MD, error) {
	md, err := s.ClientStream.Header()
	if err != nil {
		s.finish(err)
	}
	return md, err
}

func (s *grpcClientStream) CloseSend() error {
	err := s.ClientStream.CloseSend()
	if err != nil {
		s.finish(err)
	}
	return err
}

func (s *grpcClientStream) SendMsg(m interface{}) error {
	err := s.ClientStream.SendMsg(m)
	if err != nil && !errors.Is(err, io.EOF) {
		s.finish(err)
	}
	return err
}

func (s *grpcClientStream) RecvMsg(m interface{}) error {
	err := s.ClientStream.RecvMsg(m)
	if errors.Is(err, io.EOF) {
		s.finish(nil)
	} else if err != nil {
		s.finish(err)
	}
	return err
}

func (s *grpcClientStream) finish(err error) {
	s.endOnce.Do(func() {
		finishGRPCSpan(s.span, err)
	})
}

type grpcMetadataCarrier struct {
	md metadata.MD
}

func (options *grpcTraceOptions) shouldSkipGRPCMethod(fullMethod string) bool {
	method := normalizeGRPCMethod(fullMethod)
	if method == "" {
		return false
	}
	if _, ok := options.skipMethods[method]; ok {
		return true
	}
	service := grpcServiceName(method)
	for _, prefix := range options.skipServicePfx {
		if strings.HasPrefix(service, prefix) {
			return true
		}
	}
	return false
}

func normalizeGRPCMethod(fullMethod string) string {
	method := strings.TrimSpace(fullMethod)
	return strings.TrimPrefix(method, "/")
}

func (c grpcMetadataCarrier) Get(key string) string {
	values := c.md.Get(key)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func (c grpcMetadataCarrier) Set(key, value string) {
	c.md.Set(key, value)
}

func (c grpcMetadataCarrier) Keys() []string {
	keys := make([]string, 0, len(c.md))
	for key := range c.md {
		keys = append(keys, key)
	}
	return keys
}

func extractGRPCContext(ctx context.Context) context.Context {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ctx
	}
	return otel.GetTextMapPropagator().Extract(ctx, grpcMetadataCarrier{md: md})
}

func injectGRPCContext(ctx context.Context) context.Context {
	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		md = metadata.MD{}
	} else {
		md = md.Copy()
	}
	otel.GetTextMapPropagator().Inject(ctx, grpcMetadataCarrier{md: md})
	return metadata.NewOutgoingContext(ctx, md)
}

func startGRPCSpan(ctx context.Context, fullMethod string, kind trace.SpanKind) (context.Context, trace.Span) {
	if !trace.SpanContextFromContext(ctx).IsValid() {
		if traceID, err := trace.TraceIDFromHex(getTraceID(ctx)); err == nil && traceID.IsValid() {
			ctx = trace.ContextWithSpanContext(ctx, trace.NewSpanContext(trace.SpanContextConfig{TraceID: traceID}))
		}
	}
	return otel.Tracer(grpcTracerName).Start(ctx, grpcSpanName(fullMethod), trace.WithSpanKind(kind))
}

func setGRPCAttrs(span trace.Span, fullMethod string) {
	if !span.IsRecording() {
		return
	}
	span.SetAttributes(
		semconv.RPCSystemGRPC,
		semconv.RPCServiceKey.String(grpcServiceName(fullMethod)),
		semconv.RPCMethodKey.String(grpcMethodName(fullMethod)),
	)
}

func finishGRPCSpan(span trace.Span, err error) {
	defer span.End()
	if !span.IsRecording() {
		return
	}

	code := status.Code(err)
	span.SetAttributes(semconv.RPCGRPCStatusCodeKey.Int(int(code)))
	if err != nil && code != grpccodes.OK {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	logger.Trace("otel trace: %s", span.SpanContext().TraceID())
}

func setGolocalGRPC(ctx context.Context, span trace.Span) {
	if traceID := span.SpanContext().TraceID(); traceID.IsValid() {
		golocalv1.PutTraceID(traceID.String())
	}
	golocalv1.PutContext(ctx)
}

func grpcSpanName(fullMethod string) string {
	return strings.TrimPrefix(fullMethod, "/")
}

func grpcServiceName(fullMethod string) string {
	name := grpcSpanName(fullMethod)
	if index := strings.LastIndexByte(name, '/'); index >= 0 {
		return name[:index]
	}
	return name
}

func grpcMethodName(fullMethod string) string {
	name := grpcSpanName(fullMethod)
	if index := strings.LastIndexByte(name, '/'); index >= 0 {
		return name[index+1:]
	}
	return name
}
