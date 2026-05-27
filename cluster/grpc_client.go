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
	"fmt"
	"time"

	"github.com/caiflower/common-tools/cluster/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type grpcNodeClient struct {
	conn   *grpc.ClientConn
	client proto.ClusterServiceClient
}

func newGrpcNodeClient(address string) (*grpcNodeClient, error) {
	conn, err := grpc.NewClient(address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("grpc new client %s failed: %w", address, err)
	}

	client := &grpcNodeClient{
		conn:   conn,
		client: proto.NewClusterServiceClient(conn),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	_, err = client.AskLeader(ctx, &proto.AskLeaderRequest{})
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("grpc verify connection %s failed: %w", address, err)
	}

	return client, nil
}

func (g *grpcNodeClient) IsReady() bool {
	if g.conn == nil {
		return false
	}
	return g.conn.GetState().String() == "READY" || g.conn.GetState().String() == "IDLE"
}

func (g *grpcNodeClient) Close() error {
	if g.conn != nil {
		return g.conn.Close()
	}
	return nil
}

func (g *grpcNodeClient) AskLeader(ctx context.Context, req *proto.AskLeaderRequest) (*proto.AskLeaderResponse, error) {
	return g.client.AskLeader(ctx, req)
}

func (g *grpcNodeClient) AskVote(ctx context.Context, req *proto.AskVoteRequest) (*proto.AskVoteResponse, error) {
	return g.client.AskVote(ctx, req)
}

func (g *grpcNodeClient) BroadcastLeader(ctx context.Context, req *proto.BroadcastLeaderRequest) (*proto.BroadcastLeaderResponse, error) {
	return g.client.BroadcastLeader(ctx, req)
}

func (g *grpcNodeClient) RemoteCall(ctx context.Context, req *proto.RemoteCallRequest) (*proto.RemoteCallResponse, error) {
	return g.client.RemoteCall(ctx, req)
}

func (g *grpcNodeClient) Heartbeat(ctx context.Context) (proto.ClusterService_HeartbeatClient, error) {
	return g.client.Heartbeat(ctx)
}

func newRemoteCallRequest(f *FuncSpec) (*proto.RemoteCallRequest, error) {
	req := &proto.RemoteCallRequest{
		TraceId:  f.traceId,
		Uuid:     f.uuid,
		FuncName: f.funcName,
		Sync:     f.sync,
	}

	if f.param != nil {
		paramAny, err := interfaceToAny(f.param)
		if err != nil {
			return nil, fmt.Errorf("marshal param to any: %w", err)
		}
		req.Param = paramAny
	}

	return req, nil
}

func newRemoteCallResult(resp *proto.RemoteCallResponse) (interface{}, error) {
	if resp.Result != nil {
		return anyToInterface(resp.Result)
	}
	return nil, nil
}
