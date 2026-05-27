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

	"github.com/caiflower/common-tools/cluster/proto"
	"github.com/caiflower/common-tools/pkg/tools"
	"google.golang.org/protobuf/types/known/anypb"
)

type clusterServiceServer struct {
	proto.UnimplementedClusterServiceServer
	cluster *Cluster
}

func newClusterServiceServer(c *Cluster) *clusterServiceServer {
	return &clusterServiceServer{cluster: c}
}

func (s *clusterServiceServer) AskLeader(ctx context.Context, req *proto.AskLeaderRequest) (*proto.AskLeaderResponse, error) {
	c := s.cluster
	resp := &proto.AskLeaderResponse{
		NodeName: c.GetMyName(),
		Term:     req.Term,
		Success:  false,
	}

	if c.IsReady() && req.Term <= int32(c.GetMyTerm()) {
		resp.Term = int32(c.GetMyTerm())
		resp.LeaderNodeName = c.GetLeaderName()
		resp.Success = true
	}

	if _, ok := c.aliveNodes.Load(req.NodeName); !ok {
		go c.reconnect()
	}

	return resp, nil
}

func (s *clusterServiceServer) AskVote(ctx context.Context, req *proto.AskVoteRequest) (*proto.AskVoteResponse, error) {
	c := s.cluster
	resp := &proto.AskVoteResponse{
		NodeName:     c.GetMyName(),
		Term:         req.Term,
		VoteNodeName: c.getVoteNodeName(int(req.Term), req.NodeName),
		Success:      true,
	}

	if !c.IsReady() || req.Term > int32(c.GetMyTerm()) {
		c.releaseLeader()
		c.logger.Info("[cluster] received AskVote term=%d from %s, cluster ready=%v, released leader", req.Term, req.NodeName, c.IsReady())
	} else {
		resp.Success = false
	}
	c.logger.Info("[cluster] AskVote term=%d from %s, voted for %s, success=%v", req.Term, req.NodeName, resp.VoteNodeName, resp.Success)

	return resp, nil
}

func (s *clusterServiceServer) BroadcastLeader(ctx context.Context, req *proto.BroadcastLeaderRequest) (*proto.BroadcastLeaderResponse, error) {
	c := s.cluster
	resp := &proto.BroadcastLeaderResponse{
		NodeName: c.GetMyName(),
		Term:     req.Term,
		Success:  false,
	}

	if c.GetMyTerm() > int(req.Term) {
		resp.Term = int32(c.GetMyTerm())
	} else {
		var leaderNode *Node
		if v, ok := c.aliveNodes.Load(req.LeaderNodeName); ok {
			leaderNode = v.(*Node)
		} else if req.LeaderNodeName == c.GetMyName() {
			leaderNode = c.curNode
		} else if v, ok := c.allNode.Load(req.LeaderNodeName); ok {
			leaderNode = v.(*Node)
			go c.reconnect()
		}
		if leaderNode != nil {
			resp.Success = true
			c.signLeader(leaderNode, int(req.Term))
			resp.LeaderNodeName = req.LeaderNodeName
		}
	}

	c.logger.Trace("[cluster] BroadcastLeader from %s, success=%v", req.NodeName, resp.Success)
	return resp, nil
}

func (s *clusterServiceServer) Heartbeat(stream proto.ClusterService_HeartbeatServer) error {
	for {
		req, err := stream.Recv()
		if err != nil {
			return err
		}

		c := s.cluster
		resp := &proto.HeartbeatResponse{
			NodeName:       c.GetMyName(),
			Term:           req.Term,
			LeaderNodeName: c.GetLeaderName(),
		}

		if c.GetMyTerm() > int(req.Term) {
			resp.Term = int32(c.GetMyTerm())
			resp.Success = false
		} else if c.GetLeaderName() != req.NodeName {
			resp.Success = false
			c.logger.Debug("[cluster] heartbeat from %s rejected, current leader is %s", req.NodeName, c.GetLeaderName())
		} else {
			resp.Success = true
			c.curNode.updateHeartbeat()
		}

		if err := stream.Send(resp); err != nil {
			return err
		}
	}
}

func (s *clusterServiceServer) RemoteCall(ctx context.Context, req *proto.RemoteCallRequest) (*proto.RemoteCallResponse, error) {
	c := s.cluster
	f := &FuncSpec{
		traceId:  req.TraceId,
		uuid:     req.Uuid,
		funcName: req.FuncName,
		sync:     req.Sync,
	}

	if req.Param != nil {
		param, err := anyToInterface(req.Param)
		if err != nil {
			c.logger.Warn("[cluster] unmarshal remote call param failed: %s", err.Error())
		} else {
			f.param = param
		}
	}

	c.logger.Trace("[cluster] %s executing remote func '%s'", c.GetMyName(), f.funcName)
	c.callLocalFunc(f)

	resp := &proto.RemoteCallResponse{
		TraceId:  f.traceId,
		Uuid:     f.uuid,
		FuncName: f.funcName,
	}

	if f.result != nil {
		anyResult, err := interfaceToAny(f.result)
		if err != nil {
			c.logger.Warn("[cluster] marshal remote call result failed: %s", err.Error())
		} else {
			resp.Result = anyResult
		}
	}

	if f.err != nil {
		resp.Err = f.err.Error()
	}

	return resp, nil
}

func anyToInterface(anyMsg *anypb.Any) (interface{}, error) {
	if anyMsg == nil {
		return nil, nil
	}
	var result interface{}
	if err := tools.Unmarshal(anyMsg.Value, &result); err != nil {
		return nil, fmt.Errorf("unmarshal any value: %w", err)
	}
	if s, ok := result.(string); ok {
		return s, nil
	}
	return result, nil
}

func interfaceToAny(data interface{}) (*anypb.Any, error) {
	bytes, err := tools.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("marshal data to bytes: %w", err)
	}
	return &anypb.Any{
		TypeUrl: "type.googleapis.com/cluster.RemoteCallParam",
		Value:   bytes,
	}, nil
}
