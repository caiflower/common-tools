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

package cluster

import (
	"context"
	"testing"

	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
)

func TestNewClusterWithArgsAppliesGRPCInterceptorOptions(t *testing.T) {
	serverUnary := grpc.UnaryServerInterceptor(func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		return handler(ctx, req)
	})
	serverStream := grpc.StreamServerInterceptor(func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		return handler(srv, ss)
	})
	clientUnary := grpc.UnaryClientInterceptor(func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		return invoker(ctx, method, req, reply, cc, opts...)
	})
	clientStream := grpc.StreamClientInterceptor(func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		return streamer(ctx, desc, cc, method, opts...)
	})

	cluster, err := NewClusterWithArgs(
		Config{Enable: "false"},
		WithLogger(logger.NewLogger(&logger.Config{Level: "FATAL"})),
		WithServerUnaryInterceptors(serverUnary),
		WithServerStreamInterceptors(serverStream),
		WithClientUnaryInterceptors(clientUnary),
		WithClientStreamInterceptors(clientStream),
	)
	assert.NoError(t, err)
	assert.Len(t, cluster.serverUnaryInterceptors, 1)
	assert.Len(t, cluster.serverStreamInterceptors, 1)
	assert.Len(t, cluster.clientUnaryInterceptors, 1)
	assert.Len(t, cluster.clientStreamInterceptors, 1)
}

func TestNewClusterWithArgsIgnoresNilOptions(t *testing.T) {
	cluster, err := NewClusterWithArgs(
		Config{Enable: "false"},
		WithLogger(logger.NewLogger(&logger.Config{Level: "FATAL"})),
		nil,
		WithServerUnaryInterceptors(),
	)
	assert.NoError(t, err)
	assert.Empty(t, cluster.serverUnaryInterceptors)
}
