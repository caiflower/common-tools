/*
 * Copyright 2024 caiflower Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package dbv1

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"time"

	golocalv1 "github.com/caiflower/common-tools/pkg/golocal/v1"
	"github.com/uptrace/bun/schema"

	"github.com/caiflower/common-tools/global"
	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/caiflower/common-tools/pkg/tools"
	_ "github.com/go-sql-driver/mysql"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/mysqldialect"
)

const traceId = "traceId"

type IDB interface {
	GetDB() *bun.DB                                                          // 获取数据库连接，无事物
	GetTx(tx *bun.Tx) bun.IDB                                                // 获取数据库连接，如果tx=nil，那么获取的是无事物的连接，否者返回tx。
	Begin() (*bun.Tx, context.CancelFunc, error)                             // 获取一个连接，并且开始事务
	Close()                                                                  // 关闭DB
	GetSelect(model interface{}) *bun.SelectQuery                            // 获得通用处理器：查询
	GetInsert(model interface{}, tx *bun.Tx) *bun.InsertQuery                // 获得通用处理器：写入
	GetUpdate(model interface{}, tx *bun.Tx) *bun.UpdateQuery                // 获得通用处理器：更新
	GetDelete(model interface{}, tx *bun.Tx) *bun.DeleteQuery                // 获得通用处理器：删除
	GetSoftDelete(model interface{}, tx *bun.Tx) *bun.UpdateQuery            // 获得通用处理器：逻辑删除
	Insert(data interface{}, tx *bun.Tx) (int64, error)                      // 通用处理：插入数据(单条及批量处理，批量太大时不要使用)
	SoftDelete(model interface{}, tx *bun.Tx, id interface{}) (int64, error) // 通用处理：逻辑删除(id可以是单个也可以是数组)
	Delete(model interface{}, tx *bun.Tx, id interface{}) (int64, error)     // 通用处理：物理删除(id可以是单个也可以是数组)
	QueryPage(result interface{}, filter Filter) (int, error)                // 通用处理：根据条件查询
	QueryAll(result interface{}) (int, error)                                // 通用处理：查询全量
	GetRowsAffected(result sql.Result, err error) (int64, error)             // 通用处理：获取执行结果影响的记录数量
	ParseErr(err error) error                                                // 单个数据操作，消化ErrNoRows
}

type Filter interface {
	GetPage() (offset int, limit int, disable bool)
	Filter(db bun.IDB) *bun.SelectQuery
}

type Client struct {
	DB     *bun.DB
	config *Config
	cancel context.CancelFunc
}

var once sync.Once

func NewDBClient(config Config) (c *Client, err error) {
	if err = tools.DoTagFunc(&config, nil, []func(reflect.StructField, reflect.Value, interface{}) error{tools.SetDefaultValueIfNil}); err != nil {
		return nil, err
	}

	logger.Info(" *** db Config *** %s", tools.ToJson(config))

	switch config.Dialect {
	case "mysql":
		c, err = createMysqlClient(&config)
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

	if config.EnableMetric {
		ctx, cancelFunc := context.WithCancel(GetContext())
		c.cancel = cancelFunc
		once.Do(func() {
			startMetric(ctx, c.DB, &config)
		})
	}

	if err = c.DB.Ping(); err != nil {
		return nil, errors.New("connect to db failed")
	}

	global.DefaultResourceManger.Add(c)
	return c, nil
}

func createMysqlClient(config *Config) (*Client, error) {
	password := config.Password
	if config.EnablePasswordEncrypt {
		_tmpPassword, err := tools.AesDecryptRawBase64(password)
		if err != nil {
			return nil, err
		}
		password = _tmpPassword
	}
	dns := fmt.Sprintf("%s:%s@tcp(%s)/%s?charset=%s", config.User, password, config.Url, config.DbName, config.Charset)
	db, err := sql.Open("mysql", dns)
	if err != nil {
		return nil, err
	}

	// See "Important settings" section.
	db.SetConnMaxLifetime(time.Second * time.Duration(config.ConnMaxLifetime))
	db.SetMaxOpenConns(config.MaxOpen)
	db.SetMaxIdleConns(config.MaxIdle)
	ctx, cancelFunc := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelFunc()
	if err = db.PingContext(ctx); err != nil {
		return nil, errors.New("connect to db timeout")
	}

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

func (c *Client) Begin() (*bun.Tx, context.CancelFunc, error) {
	ctx, cancel := context.WithTimeout(GetContext(), c.config.TransactionTimeout)
	tx, err := c.DB.BeginTx(ctx, nil)
	if err != nil {
		cancel()
		return nil, nil, err
	}
	return &tx, cancel, nil
}

func (c *Client) Close() {
	logger.Info(" *** db Client Close *** ")
	if c.cancel != nil {
		c.cancel()
	}
	err := c.DB.Close()
	if err != nil {
		logger.Warn(" *** db Client Close Failed *** \n err: %s", err)
	}
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
	return c.GetRowsAffected(c.GetTx(tx).NewInsert().Model(data).Exec(GetContext()))
}

func (c *Client) SoftDelete(model interface{}, tx *bun.Tx, id interface{}) (int64, error) {
	handler := c.GetSoftDelete(model, tx)
	if reflect.TypeOf(id).Kind() == reflect.Slice {
		handler.Where("id in (?)", bun.In(id))
	} else {
		handler.Where("id=?", id)
	}
	return c.GetRowsAffected(handler.Exec(GetContext()))
}

func (c *Client) Delete(model interface{}, tx *bun.Tx, id interface{}) (int64, error) {
	handler := c.GetDelete(model, tx)
	if reflect.TypeOf(id).Kind() == reflect.Slice {
		handler.Where("id in (?)", bun.In(id))
	} else {
		handler.Where("id=?", id)
	}
	return c.GetRowsAffected(handler.Exec(GetContext()))
}

func (c *Client) QueryAll(result interface{}) (int, error) {
	return c.GetSelect(result).Order("id desc").ScanAndCount(GetContext(), result)
}

func (c *Client) QueryPage(result interface{}, filter Filter) (int, error) {
	if filter != nil {
		offset, limit, disable := filter.GetPage()
		if !disable {
			return filter.Filter(c.GetDB()).Model(result).Offset(offset).Limit(limit).ScanAndCount(GetContext(), result)
		}

		return filter.Filter(c.GetDB()).Model(result).ScanAndCount(GetContext(), result)
	}

	return c.QueryAll(result)
}

func (c *Client) GetRowsAffected(result sql.Result, err error) (int64, error) {
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (c *Client) ParseErr(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}

	return err
}

func (c *Client) BeforeQuery(ctx context.Context, event *bun.QueryEvent) context.Context {
	return ctx
}

func (c *Client) AfterQuery(ctx context.Context, event *bun.QueryEvent) {
	if c.config.Debug {
		if requestId, trace := golocalv1.GetTraceID(), ctx.Value(traceId); requestId == "" && trace != nil {
			golocalv1.PutTraceID(trace.(string))
		}
		rows := ""
		if event.Result != nil {
			row, _ := event.Result.RowsAffected()
			rows = fmt.Sprintf(". rows_affected=%d.", row)
		}
		if event.Err == nil {
			logger.Info("SqlTrace -> %v. cost=%v%s", event.Query, time.Since(event.StartTime), rows)
		} else {
			logger.Error("SqlTrace -> %v. cost=%v%s. err=%v", event.Query, time.Since(event.StartTime), rows, event.Err)
		}
	}
}

// GetContext getContext with traceId
func GetContext() context.Context {
	return context.WithValue(golocalv1.GetContext(), traceId, golocalv1.GetTraceID())
}
