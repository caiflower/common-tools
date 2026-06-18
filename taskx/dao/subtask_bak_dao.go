package dao

import (
	"context"

	"github.com/caiflower/common-tools/taskx/dao/model"
)

// SubtaskBakDAO defines the storage-agnostic interface for subtask backup persistence.
// Only methods actually called by the dispatcher/receiver are included.
type SubtaskBakDAO interface {
	GetByTaskID(ctx context.Context, taskID string) ([]model.SubtaskBak, error)
}
