package dao

import (
	"context"

	"github.com/caiflower/common-tools/pkg/basic"
	"github.com/caiflower/common-tools/taskx/dao/model"
)

// TaskDAO defines the storage-agnostic interface for task persistence.
// Implementations live in sub-packages: sqld (SQL) and redisd (Redis).
type TaskDAO interface {
	GetStore() Store
	Insert(ctx context.Context, data *model.Task) (int64, error)
	QueryPage(ctx context.Context, filter *model.TaskFilter) (res []model.Task, cnt int, err error)
	GetByID(ctx context.Context, id string) (*model.Task, error)
	DeleteByID(ctx context.Context, id string) (int64, error)
	SoftDeleteByID(ctx context.Context, id string) (int64, error)
	GetByIDs(ctx context.Context, taskIDs []string) ([]model.Task, error)
	GetTodoTask(ctx context.Context, taskState []string, time basic.Time) ([]model.Task, error)
	CASWorkerAndState(ctx context.Context, taskID string, worker, state string, oldWorker string) (int64, error)
	SetState(ctx context.Context, id string, state string) (int64, error)
	SetOutputAndState(ctx context.Context, taskID string, output, state string) error
	SetRetry(ctx context.Context, taskID string, retry int8) error
}
