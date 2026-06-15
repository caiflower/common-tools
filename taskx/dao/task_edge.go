package dao

import (
	"context"

	"github.com/caiflower/common-tools/db/v1"
	"github.com/caiflower/common-tools/taskx/dao/model"
	"github.com/uptrace/bun"
)

type TaskEdgeDAO interface {
	GetClient() dbv1.DB
	Insert(ctx context.Context, data *model.TaskEdge, tx ...*bun.Tx) (int64, error)
	BatchInsert(ctx context.Context, data []model.TaskEdge, tx ...*bun.Tx) (int64, error)
	QueryPage(ctx context.Context, filter *model.TaskEdgeFilter) (res []model.TaskEdge, cnt int, err error)
	DeleteByID(ctx context.Context, id string, tx ...*bun.Tx) (int64, error)
	GetByTaskID(ctx context.Context, taskID string) ([]model.TaskEdge, error)
	DeleteByTaskID(ctx context.Context, taskID string, tx ...*bun.Tx) (int64, error)
}

const TableNameOfTaskEdge = "task_edge"

type taskEdgeDAO struct {
	Client *dbv1.Client `autowired:""`
}

func NewTaskEdgeDAOWithClient(db *dbv1.Client) TaskEdgeDAO {
	return &taskEdgeDAO{Client: db}
}

func NewTaskEdgeDAO() TaskEdgeDAO {
	return &taskEdgeDAO{}
}

func (d *taskEdgeDAO) GetClient() dbv1.DB {
	return d.Client
}

func (d *taskEdgeDAO) Insert(ctx context.Context, data *model.TaskEdge, tx ...*bun.Tx) (int64, error) {
	return d.Client.Insert(ctx, data, tx...)
}

func (d *taskEdgeDAO) BatchInsert(ctx context.Context, data []model.TaskEdge, tx ...*bun.Tx) (int64, error) {
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

func (d *taskEdgeDAO) QueryPage(ctx context.Context, filter *model.TaskEdgeFilter) (res []model.TaskEdge, cnt int, err error) {
	res = make([]model.TaskEdge, 0)
	cnt, err = d.Client.QueryPage(ctx, &res, filter)
	return
}

func (d *taskEdgeDAO) DeleteByID(ctx context.Context, id string, tx ...*bun.Tx) (int64, error) {
	result, err := d.Client.DB.NewDelete().Table(TableNameOfTaskEdge).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (d *taskEdgeDAO) GetByTaskID(ctx context.Context, taskID string) ([]model.TaskEdge, error) {
	var edges []model.TaskEdge
	// 注意：GetSelect 会自动拼接 WHERE status>0，但 task_edge 表没有 status 列，所以要直接用 NewSelect
	err := d.Client.GetDB().NewSelect().Model(&edges).Where("task_id = ?", taskID).Scan(ctx, &edges)
	if err != nil {
		return nil, err
	}
	return edges, nil
}

func (d *taskEdgeDAO) DeleteByTaskID(ctx context.Context, taskID string, tx ...*bun.Tx) (int64, error) {
	result, err := d.Client.GetTx(tx...).NewDelete().Table(TableNameOfTaskEdge).Where("task_id = ?", taskID).Exec(ctx)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
