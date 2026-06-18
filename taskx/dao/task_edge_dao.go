package dao

import (
	"context"

	"github.com/caiflower/common-tools/taskx/dao/model"
)

// TaskEdgeDAO defines the storage-agnostic interface for task edge persistence.
// Only methods actually called by the dispatcher/receiver are included.
type TaskEdgeDAO interface {
	BatchInsert(ctx context.Context, data []model.TaskEdge) (int64, error)
	GetByTaskID(ctx context.Context, taskID string) ([]model.TaskEdge, error)
}
