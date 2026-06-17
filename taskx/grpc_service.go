package taskx

import (
	"context"

	golocalv1 "github.com/caiflower/common-tools/pkg/golocal/v1"
	"github.com/caiflower/common-tools/taskx/proto"
)

type taskXServiceServer struct {
	proto.UnimplementedTaskXServiceServer
	receiver *taskReceiver
}

func newTaskXServiceServer(receiver *taskReceiver) *taskXServiceServer {
	return &taskXServiceServer{receiver: receiver}
}

func (s *taskXServiceServer) DeliverTask(ctx context.Context, req *proto.DeliverRequest) (*proto.DeliverResponse, error) {
	if req.TraceId != "" {
		golocalv1.PutTraceID(req.TraceId)
		defer golocalv1.Clean()
	}
	if err := s.receiver.deliverTask(ctx, req.Ids); err != nil {
		return nil, err
	}
	return &proto.DeliverResponse{}, nil
}

func (s *taskXServiceServer) DeliverSubtask(ctx context.Context, req *proto.DeliverRequest) (*proto.DeliverResponse, error) {
	if req.TraceId != "" {
		golocalv1.PutTraceID(req.TraceId)
		defer golocalv1.Clean()
	}
	if err := s.receiver.deliverSubtask(ctx, req.Ids); err != nil {
		return nil, err
	}
	return &proto.DeliverResponse{}, nil
}

func (s *taskXServiceServer) DeliverSubtaskRollback(ctx context.Context, req *proto.DeliverRequest) (*proto.DeliverResponse, error) {
	if req.TraceId != "" {
		golocalv1.PutTraceID(req.TraceId)
		defer golocalv1.Clean()
	}
	if err := s.receiver.deliverSubtaskRollback(ctx, req.Ids); err != nil {
		return nil, err
	}
	return &proto.DeliverResponse{}, nil
}

func (s *taskXServiceServer) HandleTaskImmediately(ctx context.Context, req *proto.HandleTaskImmediatelyRequest) (*proto.HandleTaskImmediatelyResponse, error) {
	if req.TraceId != "" {
		golocalv1.PutTraceID(req.TraceId)
		defer golocalv1.Clean()
	}
	s.receiver.TaskDispatcher.enqueueTaskIDs(req.TaskIds)
	return &proto.HandleTaskImmediatelyResponse{}, nil
}
