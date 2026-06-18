package dao

import (
	"context"

	"github.com/caiflower/common-tools/taskx/dao/model"
)

// TaskBakDAO defines the storage-agnostic interface for task backup persistence.
type TaskBakDAO interface {
	GetStore() Store
	Insert(ctx context.Context, data *model.TaskBak) (int64, error)
	QueryPage(ctx context.Context, filter *model.TaskBakFilter) (res []model.TaskBak, cnt int, err error)
	GetByID(ctx context.Context, id string) (*model.TaskBak, error)
	DeleteByID(ctx context.Context, id string) (int64, error)
	SoftDeleteByID(ctx context.Context, id string) (int64, error)
}
