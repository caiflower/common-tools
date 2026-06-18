package dao

import (
	"context"

	"github.com/caiflower/common-tools/taskx/dao/model"
)

// SubtaskBakDAO defines the storage-agnostic interface for subtask backup persistence.
type SubtaskBakDAO interface {
	GetStore() Store
	Insert(ctx context.Context, data *model.SubtaskBak) (int64, error)
	GetByID(ctx context.Context, id string) (*model.SubtaskBak, error)
	QueryPage(ctx context.Context, filter *model.SubtaskBakFilter) (res []model.SubtaskBak, cnt int, err error)
	DeleteByID(ctx context.Context, id string) (int64, error)
	SoftDeleteByID(ctx context.Context, id string) (int64, error)
	GetByTaskID(ctx context.Context, taskID string) ([]model.SubtaskBak, error)
}
