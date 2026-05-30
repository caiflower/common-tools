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
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"time"

	"github.com/caiflower/common-tools/cluster/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
)

type grpcNodeClient struct {
	conn   *grpc.ClientConn
	client proto.ClusterServiceClient
}

func (g *grpcNodeClient) ClientConn() grpc.ClientConnInterface {
	return g.conn
}

func newGrpcNodeClient(ctx context.Context, address string, tlsCfg *TLSConfig) (*grpcNodeClient, error) {
	var opts []grpc.DialOption
	opts = append(opts,
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                30 * time.Second,
			Timeout:             5 * time.Second,
			PermitWithoutStream: true,
		}),
	)

	if tlsCfg != nil && tlsCfg.Enabled {
		creds, err := loadTLSClientCredentials(tlsCfg)
		if err != nil {
			return nil, fmt.Errorf("grpc load TLS credentials for %s failed: %w", address, err)
		}
		opts = append(opts, grpc.WithTransportCredentials(creds))
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	conn, err := grpc.NewClient(address, opts...)
	if err != nil {
		return nil, fmt.Errorf("grpc new client %s failed: %w", address, err)
	}

	client := proto.NewClusterServiceClient(conn)
	pingTimeout := 2 * time.Second
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining < pingTimeout {
			pingTimeout = remaining
		}
	}
	pingCtx, cancel := context.WithTimeout(ctx, pingTimeout)
	defer cancel()
	if _, err := client.Ping(pingCtx, &proto.PingRequest{}); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("grpc ping %s failed: %w", address, err)
	}

	return &grpcNodeClient{
		conn:   conn,
		client: client,
	}, nil
}

func loadTLSClientCredentials(cfg *TLSConfig) (credentials.TransportCredentials, error) {
	cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("load client cert=%s key=%s failed: %w", cfg.CertFile, cfg.KeyFile, err)
	}

	tlsCfg := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}

	if cfg.CAFile != "" {
		caData, err := os.ReadFile(cfg.CAFile)
		if err != nil {
			return nil, fmt.Errorf("read CA cert %s failed: %w", cfg.CAFile, err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(caData) {
			return nil, fmt.Errorf("failed to append CA cert from %s", cfg.CAFile)
		}
		tlsCfg.RootCAs = pool
	}

	return credentials.NewTLS(tlsCfg), nil
}

func (g *grpcNodeClient) IsReady() bool {
	if g.conn == nil {
		return false
	}
	state := g.conn.GetState()
	return state == connectivity.Ready || state == connectivity.Idle
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
