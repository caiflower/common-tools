package sqld

import (
	"context"

	dbv1 "github.com/caiflower/common-tools/db/v1"
	"github.com/caiflower/common-tools/taskx/dao"
	"github.com/caiflower/common-tools/taskx/dao/model"
	"github.com/uptrace/bun"
)

const DefaultTableNameOfTaskBak = "task_bak"

type taskBakDAO struct {
	client    *dbv1.Client
	store     dao.Store
	tableName string
}

func (d *taskBakDAO) db(ctx context.Context) bun.IDB {
	if tx := dao.TxFromContext(ctx); tx != nil {
		if bunTx, ok := tx.(*bun.Tx); ok {
			return bunTx
		}
	}
	return d.client.GetDB()
}

func NewTaskBakDAOWithClient(client *dbv1.Client) dao.TaskBakDAO {
	return &taskBakDAO{client: client, store: NewStore(client), tableName: DefaultTableNameOfTaskBak}
}

func NewTaskBakDAO() dao.TaskBakDAO {
	return &taskBakDAO{tableName: DefaultTableNameOfTaskBak}
}

func NewTaskBakDAOWithConfig(client *dbv1.Client, tableName string) dao.TaskBakDAO {
	if tableName == "" {
		tableName = DefaultTableNameOfTaskBak
	}
	return &taskBakDAO{client: client, store: NewStore(client), tableName: tableName}
}

func (d *taskBakDAO) GetStore() dao.Store { return d.store }

func (d *taskBakDAO) Insert(ctx context.Context, data *model.TaskBak) (int64, error) {
	return d.client.GetRowsAffected(d.db(ctx).NewInsert().Model(data).ModelTableExpr(d.tableName).Exec(ctx))
}

func (d *taskBakDAO) QueryPage(ctx context.Context, filter *model.TaskBakFilter) (res []model.TaskBak, cnt int, err error) {
	res = make([]model.TaskBak, 0)
	cnt, err = d.client.QueryPage(ctx, &res, filter)
	return
}

func (d *taskBakDAO) GetByID(ctx context.Context, id string) (*model.TaskBak, error) {
	m := new(model.TaskBak)
	err := d.client.GetDB().NewSelect().Model(m).ModelTableExpr(d.tableName).ColumnExpr("*").Where("status>0").Where("id = ?", id).Limit(1).Scan(ctx)
	if err != nil {
		if d.client.ParseErr(err) == nil {
			return nil, nil
		}
		return nil, err
	}
	return m, err
}

func (d *taskBakDAO) DeleteByID(ctx context.Context, id string) (int64, error) {
	result, err := d.db(ctx).NewDelete().Table(d.tableName).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (d *taskBakDAO) SoftDeleteByID(ctx context.Context, id string) (int64, error) {
	result, err := d.db(ctx).NewUpdate().Table(d.tableName).Set("status = ?", -1).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
