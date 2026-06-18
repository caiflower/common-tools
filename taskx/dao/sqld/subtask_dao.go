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

const DefaultTableNameOfSubtask = "subtask"

type subtaskDAO struct {
	client    *dbv1.Client
	store     dao.Store
	tableName string
}

func (d *subtaskDAO) db(ctx context.Context) bun.IDB {
	if tx := dao.TxFromContext(ctx); tx != nil {
		if bunTx, ok := tx.(*bun.Tx); ok {
			return bunTx
		}
	}
	return d.client.GetDB()
}

func NewSubtaskDAOWithClient(client *dbv1.Client) dao.SubtaskDAO {
	return &subtaskDAO{client: client, store: NewStore(client), tableName: DefaultTableNameOfSubtask}
}

func NewSubtaskDAO() dao.SubtaskDAO {
	return &subtaskDAO{tableName: DefaultTableNameOfSubtask}
}

func NewSubtaskDAOWithConfig(client *dbv1.Client, tableName string) dao.SubtaskDAO {
	if tableName == "" {
		tableName = DefaultTableNameOfSubtask
	}
	return &subtaskDAO{client: client, store: NewStore(client), tableName: tableName}
}

func (d *subtaskDAO) GetStore() dao.Store { return d.store }

func (d *subtaskDAO) Insert(ctx context.Context, data *model.Subtask) (int64, error) {
	return d.client.GetRowsAffected(d.db(ctx).NewInsert().Model(data).ModelTableExpr(d.tableName).Exec(ctx))
}

func (d *subtaskDAO) BatchInsert(ctx context.Context, data []model.Subtask) (int64, error) {
	if len(data) == 0 {
		return 0, nil
	}
	pageNumber := 1
	batch := 50
	count := int64(0)
	for {
		canSplit, start, end := dbv1.SplitIndex(pageNumber, batch, len(data))
		if !canSplit {
			break
		}
		batchList := data[start:end]
		cnt, err := d.client.GetRowsAffected(d.db(ctx).NewInsert().Model(&batchList).ModelTableExpr(d.tableName).Exec(ctx))
		if err != nil {
			return count, err
		}
		count += cnt
		pageNumber++
	}
	return count, nil
}

func (d *subtaskDAO) QueryPage(ctx context.Context, filter *model.SubtaskFilter) (res []model.Subtask, cnt int, err error) {
	res = make([]model.Subtask, 0)
	cnt, err = d.client.QueryPage(ctx, &res, filter)
	return
}

func (d *subtaskDAO) GetByID(ctx context.Context, id string) (*model.Subtask, error) {
	m := new(model.Subtask)
	err := d.client.GetDB().NewSelect().Model(m).ModelTableExpr(d.tableName).ColumnExpr("*").Where("status>0").Where("id = ?", id).Limit(1).Scan(ctx)
	if err != nil {
		if d.client.ParseErr(err) == nil {
			return nil, nil
		}
		return nil, err
	}
	return m, err
}

func (d *subtaskDAO) DeleteByID(ctx context.Context, id string) (int64, error) {
	result, err := d.db(ctx).NewDelete().Table(d.tableName).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (d *subtaskDAO) SoftDeleteByID(ctx context.Context, id string) (int64, error) {
	result, err := d.db(ctx).NewUpdate().Table(d.tableName).Set("status = ?", -1).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (d *subtaskDAO) GetByTaskID(ctx context.Context, taskID string) ([]model.Subtask, error) {
	var subtasks []model.Subtask
	err := d.client.GetDB().NewSelect().Model(&subtasks).ModelTableExpr(d.tableName).ColumnExpr("*").Where("status>0").Where("task_id = ?", taskID).Scan(ctx, &subtasks)
	if err != nil {
		return nil, err
	}
	return subtasks, nil
}

func (d *subtaskDAO) CASWorkerAndState(ctx context.Context, id string, worker, state string, oldWorker string) (int64, error) {
	return d.client.GetRowsAffected(
		d.db(ctx).NewUpdate().Table(d.tableName).
			Set("worker = ?", worker).Set("state = ?", state).
			Where("id = ?", id).Where("worker = ?", oldWorker).Exec(ctx))
}

func (d *subtaskDAO) CASWorkerAndRollback(ctx context.Context, id string, worker, rollback string, oldWorker string) (int64, error) {
	return d.client.GetRowsAffected(
		d.db(ctx).NewUpdate().Table(d.tableName).
			Set("worker = ?", worker).Set("rollback = ?", rollback).
			Where("id = ?", id).Where("worker = ?", oldWorker).Exec(ctx))
}

func (d *subtaskDAO) GetByIDs(ctx context.Context, ids []string) ([]model.Subtask, error) {
	var subtasks []model.Subtask
	if len(ids) == 0 {
		return subtasks, nil
	}
	err := d.client.GetDB().NewSelect().Model(&subtasks).ModelTableExpr(d.tableName).ColumnExpr("*").Where("status>0").Where("id IN (?)", bun.In(ids)).Scan(ctx, &subtasks)
	if err != nil {
		return nil, err
	}
	return subtasks, nil
}

func (d *subtaskDAO) SetOutputAndState(ctx context.Context, id string, output, state string) error {
	_, err := d.client.GetRowsAffected(
		d.db(ctx).NewUpdate().Table(d.tableName).
			Set("output = ?", output).Set("state = ?", state).
			Set("last_run_time = ?", basic.NewFromTime(time.Now()).DBString()).
			Where("id = ?", id).Exec(ctx))
	return err
}

func (d *subtaskDAO) SetRollbackAndState(ctx context.Context, id, rollback, output string) error {
	_, err := d.client.GetRowsAffected(
		d.db(ctx).NewUpdate().Table(d.tableName).
			Set("rollback = ?", rollback).Set("output = ?", output).
			Set("last_run_time = ?", basic.NewFromTime(time.Now()).DBString()).
			Where("id = ?", id).Exec(ctx))
	return err
}

func (d *subtaskDAO) SetInput(ctx context.Context, id, input string) error {
	_, err := d.client.GetRowsAffected(
		d.db(ctx).NewUpdate().Table(d.tableName).Set("input = ?", input).Where("id = ?", id).Exec(ctx))
	return err
}

func (d *subtaskDAO) SetRetry(ctx context.Context, id string, retry int8) error {
	_, err := d.client.GetRowsAffected(
		d.db(ctx).NewUpdate().Table(d.tableName).
			Set("retry = ?", retry).Set("state = ?", "pending").
			Set("last_run_time = ?", basic.NewFromTime(time.Now()).DBString()).
			Where("id = ?", id).Exec(ctx))
	return err
}
