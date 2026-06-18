package sqld

import (
	"context"
	"time"

	dbv1 "github.com/caiflower/common-tools/db/v1"
	"github.com/caiflower/common-tools/pkg/basic"
	"github.com/caiflower/common-tools/taskx/dao"
	"github.com/caiflower/common-tools/taskx/dao/model"
	"github.com/uptrace/bun"
)

const DefaultTableNameOfTask = "task"

type taskDAO struct {
	client    *dbv1.Client
	store     dao.Store
	tableName string
}

// db returns the appropriate bun.IDB for the given context.
func (d *taskDAO) db(ctx context.Context) bun.IDB {
	if tx := dao.TxFromContext(ctx); tx != nil {
		if bunTx, ok := tx.(*bun.Tx); ok {
			return bunTx
		}
	}
	return d.client.GetDB()
}

func NewTaskDAOWithClient(client *dbv1.Client) dao.TaskDAO {
	return &taskDAO{client: client, store: NewStore(client), tableName: DefaultTableNameOfTask}
}

func NewTaskDAO() dao.TaskDAO {
	return &taskDAO{tableName: DefaultTableNameOfTask}
}

func NewTaskDAOWithConfig(client *dbv1.Client, tableName string) dao.TaskDAO {
	if tableName == "" {
		tableName = DefaultTableNameOfTask
	}
	return &taskDAO{client: client, store: NewStore(client), tableName: tableName}
}

func (d *taskDAO) GetStore() dao.Store {
	return d.store
}

func (d *taskDAO) Insert(ctx context.Context, data *model.Task) (int64, error) {
	return d.client.GetRowsAffected(d.db(ctx).NewInsert().Model(data).ModelTableExpr(d.tableName).Exec(ctx))
}

func (d *taskDAO) QueryPage(ctx context.Context, filter *model.TaskFilter) (res []model.Task, cnt int, err error) {
	res = make([]model.Task, 0)
	cnt, err = d.client.QueryPage(ctx, &res, filter)
	return
}

func (d *taskDAO) GetByID(ctx context.Context, id string) (*model.Task, error) {
	m := new(model.Task)
	err := d.client.GetDB().NewSelect().Model(m).ModelTableExpr(d.tableName).ColumnExpr("*").Where("status>0").Where("id = ?", id).Limit(1).Scan(ctx)
	if err != nil {
		if d.client.ParseErr(err) == nil {
			return nil, nil
		}
		return nil, err
	}
	return m, err
}

func (d *taskDAO) DeleteByID(ctx context.Context, id string) (int64, error) {
	result, err := d.db(ctx).NewDelete().Table(d.tableName).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (d *taskDAO) SoftDeleteByID(ctx context.Context, id string) (int64, error) {
	result, err := d.db(ctx).NewUpdate().Table(d.tableName).Set("status = ?", -1).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (d *taskDAO) GetByIDs(ctx context.Context, taskIDs []string) ([]model.Task, error) {
	var tasks []model.Task
	err := d.client.GetDB().NewSelect().Model(&tasks).ModelTableExpr(d.tableName).ColumnExpr("*").Where("status>0").Where("id IN (?)", bun.In(taskIDs)).Scan(ctx)
	if err != nil {
		return nil, err
	}
	return tasks, err
}

func (d *taskDAO) GetTodoTask(ctx context.Context, taskState []string, t basic.Time) ([]model.Task, error) {
	var res []model.Task
	err := d.client.GetDB().NewSelect().
		Model(&res).
		ModelTableExpr(d.tableName).
		Where("state IN (?)", bun.List(taskState)).
		Where("execute_time < ? or execute_time IS NULL", t.DBString()).
		Where("status = ?", 1).
		Scan(ctx, &res)
	if err != nil {
		return nil, err
	}
	return res, err
}

func (d *taskDAO) CASWorkerAndState(ctx context.Context, id string, worker, state string, oldWorker string) (int64, error) {
	return d.client.GetRowsAffected(
		d.db(ctx).NewUpdate().
			Table(d.tableName).
			Set("worker = ?", worker).
			Set("state = ?", state).
			Where("id = ?", id).
			Where("worker = ?", oldWorker).
			Exec(ctx))
}

func (d *taskDAO) SetState(ctx context.Context, id string, state string) (int64, error) {
	return d.client.GetRowsAffected(d.db(ctx).NewUpdate().
		Table(d.tableName).
		Set("state = ?", state).
		Where("id = ?", id).
		Exec(ctx))
}

func (d *taskDAO) SetOutputAndState(ctx context.Context, taskID string, output, state string) error {
	_, err := d.client.GetRowsAffected(
		d.db(ctx).NewUpdate().
			Table(d.tableName).
			Set("output = ?", output).
			Set("state = ?", state).
			Set("last_run_time = ?", basic.NewFromTime(time.Now()).DBString()).
			Where("id = ?", taskID).
			Exec(ctx))
	return err
}

func (d *taskDAO) SetRetry(ctx context.Context, taskID string, retry int8) error {
	_, err := d.client.GetRowsAffected(
		d.db(ctx).NewUpdate().
			Table(d.tableName).
			Set("retry = ?", retry).
			Where("id = ?", taskID).
			Exec(ctx))
	return err
}
