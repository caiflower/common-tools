package v1

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"time"

	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/caiflower/common-tools/pkg/tools"
	_ "github.com/go-sql-driver/mysql"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/mysqldialect"
	"github.com/uptrace/bun/schema"
)

type IDB interface {
	GetDB() *bun.DB                                                          // 获取数据库连接，无事物
	GetTx(tx *bun.Tx) bun.IDB                                                // 获取数据库连接，如果tx=nil，那么获取的是无事物的连接，否者返回tx。
	Begin() (*bun.Tx, error)                                                 // 获取一个连接，并且开始事务
	Close()                                                                  // 关闭DB
	GetSelect(model interface{}) *bun.SelectQuery                            // 获得通用处理器：查询
	GetInsert(model interface{}, tx *bun.Tx) *bun.InsertQuery                // 获得通用处理器：写入
	GetUpdate(model interface{}, tx *bun.Tx) *bun.UpdateQuery                // 获得通用处理器：更新
	GetDelete(model interface{}, tx *bun.Tx) *bun.DeleteQuery                // 获得通用处理器：删除
	GetSoftDelete(model interface{}, tx *bun.Tx) *bun.UpdateQuery            // 获得通用处理器：逻辑删除
	Insert(data interface{}, tx *bun.Tx) (int64, error)                      // 通用处理：插入数据(单条及批量处理，批量太大时不要使用)
	SoftDelete(model interface{}, tx *bun.Tx, id interface{}) (int64, error) // 通用处理：逻辑删除(id可以是单个也可以是数组)
	Delete(model interface{}, tx *bun.Tx, id interface{}) (int64, error)     // 通用处理：物理删除(id可以是单个也可以是数组)
	QueryByCondition(result interface{}, filter Filter) (int, error)         // 通用处理：根据条件查询
	QueryAll(result interface{}) (int, error)                                // 通用处理：查询全量
	GetRowsAffected(result sql.Result, err error) (int64, error)             // 通用处理：获取执行结果影响的记录数量
}

type Filter interface {
	GetPage() (offset int, limit int, disable bool)
	Filter(db bun.IDB) *bun.SelectQuery
}

type Client struct {
	DB     *bun.DB
	config *Config
}

func NewDBClient(config *Config) (c *Client, err error) {
	if config == nil {
		return nil, fmt.Errorf("db Config is nil")
	}

	logger.Info(" *** db Config *** %s", tools.ToJson(config))

	switch config.Dialect {
	case "mysql":
		c, err = createMysqlClient(config)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported dialect %s", config.Dialect)
	}

	c.DB.AddQueryHook(c)

	// 关闭表名复数形式
	if !config.Plural {
		schema.SetTableNameInflector(func(tableName string) string {
			return tableName
		})
	}
	return c, nil
}

func createMysqlClient(config *Config) (*Client, error) {
	password := config.Password
	dns := fmt.Sprintf("%s:%s@tcp(%s)/%s?charset=%s", config.User, password, config.Url, config.DbName, config.Charset)
	db, err := sql.Open("mysql", dns)
	if err != nil {
		return nil, err
	}

	// See "Important settings" section.
	db.SetConnMaxLifetime(time.Second * time.Duration(config.ConnMaxLifetime))
	db.SetMaxOpenConns(config.MaxOpen)
	db.SetMaxIdleConns(config.MaxIdle)

	bunDB := bun.NewDB(db, mysqldialect.New())
	return &Client{DB: bunDB, config: config}, nil
}

func (c *Client) GetDB() *bun.DB {
	return c.DB
}

func (c *Client) GetTx(tx *bun.Tx) bun.IDB {
	if tx == nil {
		return c.DB
	}
	return tx
}

func (c *Client) Begin() (*bun.Tx, error) {
	tx, err := c.DB.Begin()
	if err != nil {
		return nil, err
	}
	return &tx, nil
}

func (c *Client) Close() {
	logger.Info(" *** db Client Close *** ")
	c.DB.Close()
}

func (c *Client) GetSelect(model interface{}) *bun.SelectQuery {
	return c.GetDB().NewSelect().Model(model).Where("status>0")
}

func (c *Client) GetInsert(model interface{}, tx *bun.Tx) *bun.InsertQuery {
	return c.GetTx(tx).NewInsert().Model(model)
}

func (c *Client) GetUpdate(model interface{}, tx *bun.Tx) *bun.UpdateQuery {
	return c.GetTx(tx).NewUpdate().Model(model).Set("update_time=current_timestamp")
}

func (c *Client) GetDelete(model interface{}, tx *bun.Tx) *bun.DeleteQuery {
	return c.GetTx(tx).NewDelete().Model(model)
}

func (c *Client) GetSoftDelete(model interface{}, tx *bun.Tx) *bun.UpdateQuery {
	return c.GetTx(tx).NewUpdate().Model(model).Set("update_time=current_timestamp").Set("status=-1")
}

func (c *Client) Insert(data interface{}, tx *bun.Tx) (int64, error) {
	return c.GetRowsAffected(c.GetTx(tx).NewInsert().Model(data).Exec(context.Background()))
}

func (c *Client) SoftDelete(model interface{}, tx *bun.Tx, id interface{}) (int64, error) {
	handler := c.GetSoftDelete(model, tx)
	if reflect.TypeOf(id).Kind() == reflect.Slice {
		handler.Where("id in (?)", bun.In(id))
	} else {
		handler.Where("id=?", id)
	}
	return c.GetRowsAffected(handler.Exec(context.Background()))
}

func (c *Client) Delete(model interface{}, tx *bun.Tx, id interface{}) (int64, error) {
	handler := c.GetDelete(model, tx)
	if reflect.TypeOf(id).Kind() == reflect.Slice {
		handler.Where("id in (?)", bun.In(id))
	} else {
		handler.Where("id=?", id)
	}
	return c.GetRowsAffected(handler.Exec(context.Background()))
}

func (c *Client) QueryAll(result interface{}) (int, error) {
	return c.GetSelect(result).Order("id desc").ScanAndCount(context.Background(), result)
}

func (c *Client) QueryByCondition(result interface{}, filter Filter) (int, error) {
	if filter != nil {
		offset, limit, disable := filter.GetPage()
		if !disable {
			return filter.Filter(c.GetDB()).Model(result).Offset(offset).Limit(limit).ScanAndCount(context.Background(), result)
		}

		return filter.Filter(c.GetDB()).Model(result).ScanAndCount(context.Background(), result)
	}

	return c.QueryAll(result)
}

func (c *Client) GetRowsAffected(result sql.Result, err error) (int64, error) {
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (c *Client) BeforeQuery(ctx context.Context, event *bun.QueryEvent) context.Context {
	return ctx
}

func (c *Client) AfterQuery(ctx context.Context, event *bun.QueryEvent) {
	if c.config.Debug {
		rows := ""
		if event.Result != nil {
			c, _ := event.Result.RowsAffected()
			rows = fmt.Sprintf(". rows_affected=%d.", c)
		}
		if event.Err == nil {
			logger.Debug("SqlTrace -> %v. cost=%v%s", event.Query, time.Since(event.StartTime), rows)
		} else {
			logger.Error("SqlTrace -> %v. cost=%v%s. err=%v", event.Query, time.Since(event.StartTime), rows, event.Err)
		}
	}
}
