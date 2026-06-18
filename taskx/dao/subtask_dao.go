package dao

import (
	"context"

	"github.com/caiflower/common-tools/taskx/dao/model"
)

// SubtaskDAO defines the storage-agnostic interface for subtask persistence.
// Only methods actually called by the dispatcher/receiver are included.
type SubtaskDAO interface {
	BatchInsert(ctx context.Context, data []model.Subtask) (int64, error)
	GetByID(ctx context.Context, id string) (*model.Subtask, error)
	GetByTaskID(ctx context.Context, taskID string) ([]model.Subtask, error)
	CASWorkerAndState(ctx context.Context, id string, worker, state string, oldWorker string) (int64, error)
	CASWorkerAndRollback(ctx context.Context, id string, worker, rollback string, oldWorker string) (int64, error)
	GetByIDs(ctx context.Context, ids []string) ([]model.Subtask, error)
	SetOutputAndState(ctx context.Context, id string, output, state string) error
	SetRollbackAndState(ctx context.Context, id, rollback, output string) error
	SetInput(ctx context.Context, id, input string) error
	SetRetry(ctx context.Context, id string, retry int8) error
}
