package dao

import (
	"context"

	"github.com/caiflower/common-tools/db/v1"
	"github.com/caiflower/common-tools/taskx/dao/model"
	"github.com/uptrace/bun"
)

type TaskBakDAO interface {
	GetClient() dbv1.DB
	Insert(ctx context.Context, data *model.TaskBak, tx ...*bun.Tx) (int64, error)
	QueryPage(ctx context.Context, filter *model.TaskBakFilter) (res []model.TaskBak, cnt int, err error)
	GetByID(ctx context.Context, id string) (*model.TaskBak, error)
	DeleteByID(ctx context.Context, id string, tx ...*bun.Tx) (int64, error)
	SoftDeleteByID(ctx context.Context, id string, tx ...*bun.Tx) (int64, error)
}

const TableNameOfTaskBak = "task_bak"

type taskBakDAO struct {
	Client *dbv1.Client `autowired:""`
}

// NewTaskBakDAOWithClient new client with db client
func NewTaskBakDAOWithClient(db *dbv1.Client) TaskBakDAO {
	return &taskBakDAO{Client: db}
}

// NewTaskBakDAO new client
func NewTaskBakDAO() TaskBakDAO {
	return &taskBakDAO{}
}

// GetClient get the db client
func (d *taskBakDAO) GetClient() dbv1.DB {
	return d.Client
}

// Insert create a new record
func (d *taskBakDAO) Insert(ctx context.Context, data *model.TaskBak, tx ...*bun.Tx) (int64, error) {
	return d.Client.Insert(ctx, data, tx...)
}

// QueryPage query by page
func (d *taskBakDAO) QueryPage(ctx context.Context, filter *model.TaskBakFilter) (res []model.TaskBak, cnt int, err error) {
	res = make([]model.TaskBak, 0)
	cnt, err = d.Client.QueryPage(ctx, &res, filter)
	return
}

// DeleteByID physically delete record by primaryKey
func (d *taskBakDAO) DeleteByID(ctx context.Context, id string, tx ...*bun.Tx) (int64, error) {
	result, err := d.Client.DB.NewDelete().Table(TableNameOfTaskBak).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// GetByID get by primaryKey, return nil if not found
func (d *taskBakDAO) GetByID(ctx context.Context, id string) (*model.TaskBak, error) {
	m := new(model.TaskBak)
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
func (d *taskBakDAO) SoftDeleteByID(ctx context.Context, id string, tx ...*bun.Tx) (int64, error) {
	result, err := d.Client.DB.NewUpdate().Table(TableNameOfTaskBak).Set("status = ?", -1).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
