package dao

import (
	"context"

	"github.com/caiflower/common-tools/taskx/dao/model"
)

// TaskEdgeDAO defines the storage-agnostic interface for task edge persistence.
type TaskEdgeDAO interface {
	GetStore() Store
	Insert(ctx context.Context, data *model.TaskEdge) (int64, error)
	BatchInsert(ctx context.Context, data []model.TaskEdge) (int64, error)
	QueryPage(ctx context.Context, filter *model.TaskEdgeFilter) (res []model.TaskEdge, cnt int, err error)
	DeleteByID(ctx context.Context, id string) (int64, error)
	GetByTaskID(ctx context.Context, taskID string) ([]model.TaskEdge, error)
	DeleteByTaskID(ctx context.Context, taskID string) (int64, error)
}
