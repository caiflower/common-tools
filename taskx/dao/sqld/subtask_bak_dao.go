package sqld

import (
	"context"

	dbv1 "github.com/caiflower/common-tools/db/v1"
	"github.com/caiflower/common-tools/taskx/dao"
	"github.com/caiflower/common-tools/taskx/dao/model"
	"github.com/uptrace/bun"
)

const DefaultTableNameOfSubtaskBak = "subtask_bak"

type subtaskBakDAO struct {
	client    *dbv1.Client
	store     dao.Store
	tableName string
}

func (d *subtaskBakDAO) db(ctx context.Context) bun.IDB {
	if tx := dao.TxFromContext(ctx); tx != nil {
		if bunTx, ok := tx.(*bun.Tx); ok {
			return bunTx
		}
	}
	return d.client.GetDB()
}

func NewSubtaskBakDAOWithClient(client *dbv1.Client) dao.SubtaskBakDAO {
	return &subtaskBakDAO{client: client, store: NewStore(client), tableName: DefaultTableNameOfSubtaskBak}
}

func NewSubtaskBakDAO() dao.SubtaskBakDAO {
	return &subtaskBakDAO{tableName: DefaultTableNameOfSubtaskBak}
}

func NewSubtaskBakDAOWithConfig(client *dbv1.Client, tableName string) dao.SubtaskBakDAO {
	if tableName == "" {
		tableName = DefaultTableNameOfSubtaskBak
	}
	return &subtaskBakDAO{client: client, store: NewStore(client), tableName: tableName}
}

func (d *subtaskBakDAO) GetStore() dao.Store { return d.store }

func (d *subtaskBakDAO) Insert(ctx context.Context, data *model.SubtaskBak) (int64, error) {
	return d.client.GetRowsAffected(d.db(ctx).NewInsert().Model(data).ModelTableExpr(d.tableName).Exec(ctx))
}

func (d *subtaskBakDAO) GetByID(ctx context.Context, id string) (*model.SubtaskBak, error) {
	m := new(model.SubtaskBak)
	err := d.client.GetDB().NewSelect().Model(m).ModelTableExpr(d.tableName).ColumnExpr("*").Where("status>0").Where("id = ?", id).Limit(1).Scan(ctx)
	if err != nil {
		if d.client.ParseErr(err) == nil {
			return nil, nil
		}
		return nil, err
	}
	return m, err
}

func (d *subtaskBakDAO) QueryPage(ctx context.Context, filter *model.SubtaskBakFilter) (res []model.SubtaskBak, cnt int, err error) {
	res = make([]model.SubtaskBak, 0)
	cnt, err = d.client.QueryPage(ctx, &res, filter)
	return
}

func (d *subtaskBakDAO) DeleteByID(ctx context.Context, id string) (int64, error) {
	result, err := d.db(ctx).NewDelete().Table(d.tableName).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (d *subtaskBakDAO) SoftDeleteByID(ctx context.Context, id string) (int64, error) {
	result, err := d.db(ctx).NewUpdate().Table(d.tableName).Set("status = ?", -1).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (d *subtaskBakDAO) GetByTaskID(ctx context.Context, taskID string) ([]model.SubtaskBak, error) {
	var subtasks []model.SubtaskBak
	err := d.client.GetDB().NewSelect().Model(&subtasks).ModelTableExpr(d.tableName).ColumnExpr("*").Where("status>0").Where("task_id = ?", taskID).Scan(ctx, &subtasks)
	if err != nil {
		return nil, err
	}
	return subtasks, nil
}
