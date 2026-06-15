package dao

import (
	"context"
	"time"

	dbv1 "github.com/caiflower/common-tools/db/v1"
	"github.com/caiflower/common-tools/pkg/basic"
	"github.com/caiflower/common-tools/taskx/dao/model"
	"github.com/uptrace/bun"
)

type SubtaskDAO interface {
	GetClient() dbv1.DB
	Insert(ctx context.Context, data *model.Subtask, tx ...*bun.Tx) (int64, error)
	BatchInsert(ctx context.Context, data []model.Subtask, tx ...*bun.Tx) (int64, error)
	QueryPage(ctx context.Context, filter *model.SubtaskFilter) (res []model.Subtask, cnt int, err error)
	GetByID(ctx context.Context, id string) (*model.Subtask, error)
	DeleteByID(ctx context.Context, id string, tx ...*bun.Tx) (int64, error)
	SoftDeleteByID(ctx context.Context, id string, tx ...*bun.Tx) (int64, error)
	GetByTaskID(ctx context.Context, taskID string) ([]model.Subtask, error)
	GetByIDs(ctx context.Context, subtaskIDs []string) ([]model.Subtask, error)
	SetWorkerAndStateWithOldWorker(ctx context.Context, subtaskID string, worker, state string, oldWorker string, tx ...*bun.Tx) (int64, error)
	SetWorkerAndRollbackWithOldWorker(ctx context.Context, subtaskID string, worker, rollback string, oldWorker string, tx ...*bun.Tx) (int64, error)
	SetOutputAndState(ctx context.Context, id string, output, state string, tx ...*bun.Tx) error
	SetRollbackAndState(ctx context.Context, id string, rollback string, output string, tx ...*bun.Tx) error
	SetRetry(ctx context.Context, subtaskID string, retry int8, tx ...*bun.Tx) error
	SetInput(ctx context.Context, id, input string, tx ...*bun.Tx) error
}

const TableNameOfSubtask = "subtask"

type subtaskDAO struct {
	Client *dbv1.Client `autowired:""`
}

// NewSubtaskDAOWithClient new client with db client
func NewSubtaskDAOWithClient(db *dbv1.Client) SubtaskDAO {
	return &subtaskDAO{Client: db}
}

// NewSubtaskDAO new client
func NewSubtaskDAO() SubtaskDAO {
	return &subtaskDAO{}
}

// GetClient get the db client
func (d *subtaskDAO) GetClient() dbv1.DB {
	return d.Client
}

// Insert create a new record
func (d *subtaskDAO) Insert(ctx context.Context, data *model.Subtask, tx ...*bun.Tx) (int64, error) {
	return d.Client.Insert(ctx, data, tx...)
}

// BatchInsert batch createn
func (d *subtaskDAO) BatchInsert(ctx context.Context, data []model.Subtask, tx ...*bun.Tx) (int64, error) {
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
		cnt, err := d.Client.GetRowsAffected(d.Client.GetTx(tx...).NewInsert().Model(&batchList).Exec(ctx))
		if err != nil {
			return count, err
		}
		count += cnt
		pageNumber++
	}
	return count, nil
}

// QueryPage query by page
func (d *subtaskDAO) QueryPage(ctx context.Context, filter *model.SubtaskFilter) (res []model.Subtask, cnt int, err error) {
	res = make([]model.Subtask, 0)
	cnt, err = d.Client.QueryPage(ctx, &res, filter)
	return
}

// DeleteByID physically delete record by primaryKey
func (d *subtaskDAO) DeleteByID(ctx context.Context, id string, tx ...*bun.Tx) (int64, error) {
	result, err := d.Client.DB.NewDelete().Table(TableNameOfSubtask).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// GetByID get by primaryKey, return nil if not found
func (d *subtaskDAO) GetByID(ctx context.Context, id string) (*model.Subtask, error) {
	m := new(model.Subtask)
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
func (d *subtaskDAO) SoftDeleteByID(ctx context.Context, id string, tx ...*bun.Tx) (int64, error) {
	result, err := d.Client.DB.NewUpdate().Table(TableNameOfSubtask).Set("status = ?", -1).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (d *subtaskDAO) GetByTaskID(ctx context.Context, taskID string) ([]model.Subtask, error) {
	var subtasks []model.Subtask
	err := d.Client.GetSelect(&subtasks).Where("task_id = ?", taskID).Scan(ctx, &subtasks)
	if err != nil {
		return nil, err
	}
	return subtasks, nil
}

func (d *subtaskDAO) SetWorkerAndStateWithOldWorker(ctx context.Context, id string, worker, state string, oldWorker string, tx ...*bun.Tx) (int64, error) {
	return d.Client.GetRowsAffected(
		d.Client.GetTx(tx...).NewUpdate().
			Table(TableNameOfSubtask).
			Set("worker = ?", worker).
			Set("state = ?", state).
			Where("id = ?", id).
			Where("worker = ?", oldWorker).
			Exec(ctx))
}

func (d *subtaskDAO) SetWorkerAndRollbackWithOldWorker(ctx context.Context, id string, worker, rollback string, oldWorker string, tx ...*bun.Tx) (int64, error) {
	return d.Client.GetRowsAffected(
		d.Client.GetTx(tx...).NewUpdate().
			Table(TableNameOfSubtask).
			Set("worker = ?", worker).
			Set("rollback = ?", rollback).
			Where("id = ?", id).
			Where("worker = ?", oldWorker).
			Exec(ctx))
}

func (d *subtaskDAO) GetByIDs(ctx context.Context, ids []string) ([]model.Subtask, error) {
	var subtasks []model.Subtask
	if len(ids) == 0 {
		return subtasks, nil
	}
	err := d.Client.GetSelect(&subtasks).Where("id IN (?)", bun.In(ids)).Scan(ctx, &subtasks)
	if err != nil {
		return nil, err
	}
	return subtasks, nil
}

func (d *subtaskDAO) SetOutputAndState(ctx context.Context, id string, output, state string, tx ...*bun.Tx) error {
	_, err := d.Client.GetRowsAffected(
		d.Client.GetTx(tx...).NewUpdate().
			Table(TableNameOfSubtask).
			Set("output = ?", output).
			Set("state = ?", state).
			Set("last_run_time = ?", basic.NewFromTime(time.Now()).DBString()).
			Where("id = ?", id).
			Exec(ctx))
	if err != nil {
		return err
	}
	return nil
}

func (d *subtaskDAO) SetRollbackAndState(ctx context.Context, id, rollback, output string, tx ...*bun.Tx) error {
	_, err := d.Client.GetRowsAffected(
		d.Client.GetTx(tx...).NewUpdate().
			Table(TableNameOfSubtask).
			Set("rollback = ?", rollback).
			Set("output = ?", output).
			Set("last_run_time = ?", basic.NewFromTime(time.Now()).DBString()).
			Where("id = ?", id).
			Exec(ctx))
	if err != nil {
		return err
	}
	return nil
}

func (d *subtaskDAO) SetInput(ctx context.Context, id, input string, tx ...*bun.Tx) error {
	_, err := d.Client.GetRowsAffected(
		d.Client.GetTx(tx...).NewUpdate().
			Table(TableNameOfSubtask).
			Set("input = ?", input).
			Where("id = ?", id).
			Exec(ctx))
	return err
}

func (d *subtaskDAO) SetRetry(ctx context.Context, id string, retry int8, tx ...*bun.Tx) error {
	_, err := d.Client.GetRowsAffected(
		d.Client.GetTx(tx...).NewUpdate().
			Table(TableNameOfSubtask).
			Set("retry = ?", retry).
			Set("state = ?", "pending").
			Set("last_run_time = ?", basic.NewFromTime(time.Now()).DBString()).
			Where("id = ?", id).
			Exec(ctx))
	if err != nil {
		return err
	}
	return nil
}
