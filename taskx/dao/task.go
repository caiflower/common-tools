package dao

import (
	"context"
	"time"

	dbv1 "github.com/caiflower/common-tools/db/v1"
	"github.com/caiflower/common-tools/pkg/basic"
	"github.com/caiflower/common-tools/taskx/dao/model"
	"github.com/uptrace/bun"
)

type TaskDAO interface {
	GetClient() dbv1.DB
	Insert(ctx context.Context, data *model.Task, tx ...*bun.Tx) (int64, error)
	QueryPage(ctx context.Context, filter *model.TaskFilter) (res []model.Task, cnt int, err error)
	GetByID(ctx context.Context, id string) (*model.Task, error)
	DeleteByID(ctx context.Context, id string, tx ...*bun.Tx) (int64, error)
	SoftDeleteByID(ctx context.Context, id string, tx ...*bun.Tx) (int64, error)
	GetByIDs(ctx context.Context, taskIDs []string) ([]model.Task, error)
	GetTodoTask(ctx context.Context, taskState []string, time basic.Time) ([]model.Task, error)
	SetWorkerAndTaskStateWithOldWorker(ctx context.Context, taskID string, worker, state string, oldWorker string, tx ...*bun.Tx) (int64, error)
	SetState(ctx context.Context, id string, state string, tx ...*bun.Tx) (int64, error)
	SetOutputAndState(ctx context.Context, taskID string, output, state string, tx ...*bun.Tx) error
	SetRetry(ctx context.Context, taskID string, retry int8, tx ...*bun.Tx) error
}

const TableNameOfTask = "task"

type taskDAO struct {
	Client *dbv1.Client `autowired:""`
}

// NewTaskDAOWithClient new client with db client
func NewTaskDAOWithClient(db *dbv1.Client) TaskDAO {
	return &taskDAO{Client: db}
}

// NewTaskDAO new client
func NewTaskDAO() TaskDAO {
	return &taskDAO{}
}

// GetClient get the db client
func (d *taskDAO) GetClient() dbv1.DB {
	return d.Client
}

// Insert create a new record
func (d *taskDAO) Insert(ctx context.Context, data *model.Task, tx ...*bun.Tx) (int64, error) {
	return d.Client.Insert(ctx, data, tx...)
}

// QueryPage query by page
func (d *taskDAO) QueryPage(ctx context.Context, filter *model.TaskFilter) (res []model.Task, cnt int, err error) {
	res = make([]model.Task, 0)
	cnt, err = d.Client.QueryPage(ctx, &res, filter)
	return
}

// DeleteByID physically delete record by primaryKey
func (d *taskDAO) DeleteByID(ctx context.Context, id string, tx ...*bun.Tx) (int64, error) {
	result, err := d.Client.DB.NewDelete().Table(TableNameOfTask).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// GetByID get by primaryKey, return nil if not found
func (d *taskDAO) GetByID(ctx context.Context, id string) (*model.Task, error) {
	m := new(model.Task)
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
func (d *taskDAO) SoftDeleteByID(ctx context.Context, id string, tx ...*bun.Tx) (int64, error) {
	result, err := d.Client.DB.NewUpdate().Table(TableNameOfTask).Set("status = ?", -1).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (d *taskDAO) GetByIDs(ctx context.Context, taskIDs []string) ([]model.Task, error) {
	var tasks []model.Task
	err := d.Client.GetSelect(&tasks).Where("id IN (?)", bun.In(taskIDs)).Scan(ctx)
	if err != nil {
		return nil, err
	}
	return tasks, err
}

func (d *taskDAO) GetTodoTask(ctx context.Context, taskState []string, time basic.Time) ([]model.Task, error) {
	var res []model.Task
	err := d.Client.DB.NewSelect().
		Table(TableNameOfTask).
		Where("state IN (?)", bun.In(taskState)).
		Where("execute_time < ? or execute_time IS NULL", time.DBString()).
		Where("status = ?", 1).
		Scan(ctx, &res)
	if err != nil {
		return nil, err
	}
	return res, err
}

func (d *taskDAO) SetWorkerAndTaskStateWithOldWorker(ctx context.Context, id string, worker, state string, oldWorker string, tx ...*bun.Tx) (int64, error) {
	return d.Client.GetRowsAffected(
		d.Client.GetTx(tx...).NewUpdate().
			Table(TableNameOfTask).
			Set("worker = ?", worker).
			Set("state = ?", state).
			Where("id = ?", id).
			Where("worker = ?", oldWorker).
			Exec(ctx))
}

func (d *taskDAO) SetState(ctx context.Context, id string, state string, tx ...*bun.Tx) (int64, error) {
	return d.Client.GetRowsAffected(d.Client.GetTx(tx...).NewUpdate().
		Table(TableNameOfTask).
		Set("state = ?", state).
		Where("id = ?", id).
		Exec(ctx))
}

func (d *taskDAO) SetOutputAndState(ctx context.Context, taskID string, output, state string, tx ...*bun.Tx) error {
	_, err := d.Client.GetRowsAffected(
		d.Client.GetTx(tx...).NewUpdate().
			Table(TableNameOfTask).
			Set("output = ?", output).
			Set("state = ?", state).
			Set("last_run_time = ?", basic.NewFromTime(time.Now()).DBString()).
			Where("id = ?", taskID).
			Exec(ctx))
	if err != nil {
		return err
	}
	return nil
}

func (d *taskDAO) SetRetry(ctx context.Context, taskID string, retry int8, tx ...*bun.Tx) error {
	_, err := d.Client.GetRowsAffected(
		d.Client.GetTx(tx...).NewUpdate().
			Table(TableNameOfTask).
			Set("retry = ?", retry).
			Where("id = ?", taskID).
			Set("last_run_time = ?", basic.NewFromTime(time.Now()).DBString()).
			Exec(ctx))
	if err != nil {
		return err
	}
	return nil
}
