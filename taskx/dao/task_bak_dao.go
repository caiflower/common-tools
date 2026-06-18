package dao

import (
	"context"

	"github.com/caiflower/common-tools/taskx/dao/model"
)

// TaskBakDAO defines the storage-agnostic interface for task backup persistence.
// Only methods actually called by the dispatcher/receiver are included.
type TaskBakDAO interface {
	GetByID(ctx context.Context, id string) (*model.TaskBak, error)
}
