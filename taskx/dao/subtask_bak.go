package dao

import (
	"context"

	"github.com/caiflower/common-tools/db/v1"
	"github.com/caiflower/common-tools/taskx/dao/model"
	"github.com/uptrace/bun"
)

type SubtaskBakDAO interface {
	GetClient() dbv1.DB
	Insert(ctx context.Context, data *model.SubtaskBak, tx ...*bun.Tx) (int64, error)
	GetByID(ctx context.Context, id string) (*model.SubtaskBak, error)
	QueryPage(ctx context.Context, filter *model.SubtaskBakFilter) (res []model.SubtaskBak, cnt int, err error)
	DeleteByID(ctx context.Context, id string, tx ...*bun.Tx) (int64, error)
	SoftDeleteByID(ctx context.Context, id string, tx ...*bun.Tx) (int64, error)
	GetByTaskID(ctx context.Context, taskID string) ([]model.SubtaskBak, error)
}

const TableNameOfSubtaskBak = "subtask_bak"

type subtaskBakDAO struct {
	Client *dbv1.Client `autowired:""`
}

// NewSubtaskBakDAOWithClient new client with db client
func NewSubtaskBakDAOWithClient(db *dbv1.Client) SubtaskBakDAO {
	return &subtaskBakDAO{Client: db}
}

// NewSubtaskBakDAO new client
func NewSubtaskBakDAO() SubtaskBakDAO {
	return &subtaskBakDAO{}
}

// GetClient get the db client
func (d *subtaskBakDAO) GetClient() dbv1.DB {
	return d.Client
}

// Insert create a new record
func (d *subtaskBakDAO) Insert(ctx context.Context, data *model.SubtaskBak, tx ...*bun.Tx) (int64, error) {
	return d.Client.Insert(ctx, data, tx...)
}

// QueryPage query by page
func (d *subtaskBakDAO) QueryPage(ctx context.Context, filter *model.SubtaskBakFilter) (res []model.SubtaskBak, cnt int, err error) {
	res = make([]model.SubtaskBak, 0)
	cnt, err = d.Client.QueryPage(ctx, &res, filter)
	return
}

// DeleteByID physically delete record by primaryKey
func (d *subtaskBakDAO) DeleteByID(ctx context.Context, id string, tx ...*bun.Tx) (int64, error) {
	result, err := d.Client.DB.NewDelete().Table(TableNameOfSubtaskBak).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// GetByID get by primaryKey, return nil if not found
func (d *subtaskBakDAO) GetByID(ctx context.Context, id string) (*model.SubtaskBak, error) {
	m := new(model.SubtaskBak)
	err := d.Client.GetSelect(m).Where("id = ?", id).Limit(1).Scan(ctx)
	if err != nil {
		if d.Client.ParseErr(err) == nil {
			return nil, nil
		}
		return nil, err
	}
	return m, err
}

// SoftDeleteByID logically delete record by primaryKey (set status=-1)
func (d *subtaskBakDAO) SoftDeleteByID(ctx context.Context, id string, tx ...*bun.Tx) (int64, error) {
	result, err := d.Client.DB.NewUpdate().Table(TableNameOfSubtaskBak).Set("status = ?", -1).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (d *subtaskBakDAO) GetByTaskID(ctx context.Context, taskID string) ([]model.SubtaskBak, error) {
	var subtasks []model.SubtaskBak
	err := d.Client.GetSelect(&subtasks).Where("task_id = ?", taskID).Scan(ctx, &subtasks)
	if err != nil {
		return nil, err
	}
	return subtasks, nil
}
