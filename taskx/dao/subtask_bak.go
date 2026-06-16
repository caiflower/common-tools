package dao

import (
	"context"

	dbv1 "github.com/caiflower/common-tools/db/v1"
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

const DefaultTableNameOfSubtaskBak = "subtask_bak"

type subtaskBakDAO struct {
	Client    *dbv1.Client `autowired:""`
	tableName string
}

// NewSubtaskBakDAOWithClient new client with db client
func NewSubtaskBakDAOWithClient(db *dbv1.Client) SubtaskBakDAO {
	return &subtaskBakDAO{Client: db, tableName: DefaultTableNameOfSubtaskBak}
}

// NewSubtaskBakDAO new client
func NewSubtaskBakDAO() SubtaskBakDAO {
	return &subtaskBakDAO{tableName: DefaultTableNameOfSubtaskBak}
}

// NewSubtaskBakDAOWithConfig new client with custom table name. An empty
// tableName falls back to DefaultTableNameOfSubtaskBak.
func NewSubtaskBakDAOWithConfig(db *dbv1.Client, tableName string) SubtaskBakDAO {
	if tableName == "" {
		tableName = DefaultTableNameOfSubtaskBak
	}
	return &subtaskBakDAO{Client: db, tableName: tableName}
}

// GetClient get the db client
func (d *subtaskBakDAO) GetClient() dbv1.DB {
	return d.Client
}

// Insert create a new record
func (d *subtaskBakDAO) Insert(ctx context.Context, data *model.SubtaskBak, tx ...*bun.Tx) (int64, error) {
	if len(tx) > 0 && tx[0] != nil {
		return d.Client.GetRowsAffected(tx[0].NewInsert().Model(data).ModelTableExpr(d.tableName).Exec(ctx))
	}
	return d.Client.GetRowsAffected(d.Client.GetDB().NewInsert().Model(data).ModelTableExpr(d.tableName).Exec(ctx))
}

// QueryPage query by page
func (d *subtaskBakDAO) QueryPage(ctx context.Context, filter *model.SubtaskBakFilter) (res []model.SubtaskBak, cnt int, err error) {
	res = make([]model.SubtaskBak, 0)
	cnt, err = d.Client.QueryPage(ctx, &res, filter)
	return
}

// DeleteByID physically delete record by primaryKey
func (d *subtaskBakDAO) DeleteByID(ctx context.Context, id string, tx ...*bun.Tx) (int64, error) {
	result, err := d.Client.GetTx(tx...).NewDelete().Table(d.tableName).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// GetByID get by primaryKey, return nil if not found
func (d *subtaskBakDAO) GetByID(ctx context.Context, id string) (*model.SubtaskBak, error) {
	m := new(model.SubtaskBak)
	err := d.Client.GetDB().NewSelect().Model(m).ModelTableExpr(d.tableName).ColumnExpr("*").Where("status>0").Where("id = ?", id).Limit(1).Scan(ctx)
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
	result, err := d.Client.GetTx(tx...).NewUpdate().Table(d.tableName).Set("status = ?", -1).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (d *subtaskBakDAO) GetByTaskID(ctx context.Context, taskID string) ([]model.SubtaskBak, error) {
	var subtasks []model.SubtaskBak
	err := d.Client.GetDB().NewSelect().Model(&subtasks).ModelTableExpr(d.tableName).ColumnExpr("*").Where("status>0").Where("task_id = ?", taskID).Scan(ctx, &subtasks)
	if err != nil {
		return nil, err
	}
	return subtasks, nil
}
