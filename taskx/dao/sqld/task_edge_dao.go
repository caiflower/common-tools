package sqld

import (
	"context"

	dbv1 "github.com/caiflower/common-tools/db/v1"
	"github.com/caiflower/common-tools/taskx/dao"
	"github.com/caiflower/common-tools/taskx/dao/model"
	"github.com/uptrace/bun"
)

const DefaultTableNameOfTaskEdge = "task_edge"

type taskEdgeDAO struct {
	client    *dbv1.Client
	store     dao.Store
	tableName string
}

func (d *taskEdgeDAO) db(ctx context.Context) bun.IDB {
	if tx := dao.TxFromContext(ctx); tx != nil {
		if bunTx, ok := tx.(*bun.Tx); ok {
			return bunTx
		}
	}
	return d.client.GetDB()
}

func NewTaskEdgeDAOWithClient(client *dbv1.Client) dao.TaskEdgeDAO {
	return &taskEdgeDAO{client: client, store: NewStore(client), tableName: DefaultTableNameOfTaskEdge}
}

func NewTaskEdgeDAO() dao.TaskEdgeDAO {
	return &taskEdgeDAO{tableName: DefaultTableNameOfTaskEdge}
}

func NewTaskEdgeDAOWithConfig(client *dbv1.Client, tableName string) dao.TaskEdgeDAO {
	if tableName == "" {
		tableName = DefaultTableNameOfTaskEdge
	}
	return &taskEdgeDAO{client: client, store: NewStore(client), tableName: tableName}
}

func (d *taskEdgeDAO) GetStore() dao.Store { return d.store }

func (d *taskEdgeDAO) Insert(ctx context.Context, data *model.TaskEdge) (int64, error) {
	return d.client.GetRowsAffected(d.db(ctx).NewInsert().Model(data).ModelTableExpr(d.tableName).Exec(ctx))
}

func (d *taskEdgeDAO) BatchInsert(ctx context.Context, data []model.TaskEdge) (int64, error) {
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

func (d *taskEdgeDAO) QueryPage(ctx context.Context, filter *model.TaskEdgeFilter) (res []model.TaskEdge, cnt int, err error) {
	res = make([]model.TaskEdge, 0)
	cnt, err = d.client.QueryPage(ctx, &res, filter)
	return
}

func (d *taskEdgeDAO) DeleteByID(ctx context.Context, id string) (int64, error) {
	result, err := d.db(ctx).NewDelete().Table(d.tableName).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (d *taskEdgeDAO) GetByTaskID(ctx context.Context, taskID string) ([]model.TaskEdge, error) {
	var edges []model.TaskEdge
	err := d.client.GetDB().NewSelect().Model(&edges).ModelTableExpr(d.tableName).ColumnExpr("*").Where("task_id = ?", taskID).Scan(ctx, &edges)
	if err != nil {
		return nil, err
	}
	return edges, nil
}

func (d *taskEdgeDAO) DeleteByTaskID(ctx context.Context, taskID string) (int64, error) {
	result, err := d.db(ctx).NewDelete().Table(d.tableName).Where("task_id = ?", taskID).Exec(ctx)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
