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
	"regexp"
	"time"

	golocalv1 "github.com/caiflower/common-tools/pkg/golocal/v1"
	"github.com/uptrace/bun/driver/pgdriver"
	"github.com/uptrace/bun/schema"

	"github.com/caiflower/common-tools/global"
	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/caiflower/common-tools/pkg/tools"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/mattn/go-sqlite3"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/mysqldialect"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/dialect/sqlitedialect"
)

const traceId = "traceId"

type DB interface {
	GetDB() *bun.DB                                                                                  // 获取数据库连接，无事物
	GetTx(tx ...*bun.Tx) bun.IDB                                                                     // 获取数据库连接，如果tx=nil，那么获取的是无事物的连接，否者返回tx。
	Begin(ctx context.Context) (*bun.Tx, context.CancelFunc, error)                                  // 获取一个连接，并且开始事务
	Close()                                                                                          // 关闭DB
	GetSelect(model interface{}) *bun.SelectQuery                                                    // 获得通用处理器：查询
	GetInsert(model interface{}, tx ...*bun.Tx) *bun.InsertQuery                                     // 获得通用处理器：写入
	GetUpdate(model interface{}, tx ...*bun.Tx) *bun.UpdateQuery                                     // 获得通用处理器：更新
	GetDelete(model interface{}, tx ...*bun.Tx) *bun.DeleteQuery                                     // 获得通用处理器：删除
	GetSoftDelete(model interface{}, tx ...*bun.Tx) *bun.UpdateQuery                                 // 获得通用处理器：逻辑删除
	Insert(ctx context.Context, data interface{}, tx ...*bun.Tx) (int64, error)                      // 通用处理：插入数据(单条及批量处理，批量太大时不要使用)
	SoftDelete(ctx context.Context, model interface{}, id interface{}, tx ...*bun.Tx) (int64, error) // 通用处理：逻辑删除(id可以是单个也可以是数组)
	Delete(ctx context.Context, model interface{}, id interface{}, tx ...*bun.Tx) (int64, error)     // 通用处理：物理删除(id可以是单个也可以是数组)
	QueryPage(ctx context.Context, result interface{}, filter Filter) (int, error)                   // 通用处理：根据条件查询
	QueryAll(ctx context.Context, result interface{}) (int, error)                                   // 通用处理：查询全量
	GetRowsAffected(result sql.Result, err error) (int64, error)                                     // 通用处理：获取执行结果影响的记录数量
	ParseErr(err error) error                                                                        // 单个数据操作，消化ErrNoRows
}

type Filter interface {
	GetPage() (offset int, limit int, disable bool)
	Filter(db bun.IDB) *bun.SelectQuery
}

type Client struct {
	DB      *bun.DB
	config  *Config
	cancel  context.CancelFunc
	metrics *metricsCollector
}

func NewDBClient(config Config) (c *Client, err error) {
	_ = tools.DoTagFunc(&config, []tools.FnObj{{Fn: tools.SetDefaultValueIfNil}})
	config.Password = tools.ResolvePasswordFromEnv("DB", config.Name, config.Password)

	if err := config.Validate(); err != nil {
		return nil, err
	}

	logger.Info(" *** db Config *** %s", maskConfigPassword(tools.ToJson(config)))

	switch config.Dialect {
	case "mysql":
		c, err = createMysqlClient(&config)
		if err != nil {
			return nil, err
		}
	case "pgsql":
		c, err = createPgsqlClient(&config)
		if err != nil {
			return nil, err
		}
	case "sqlite":
		c, err = createSqliteClient(&config)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported dialect %s", config.Dialect)
	}

	c.DB.AddQueryHook(c)

	// WARNING: SetTableNameInflector modifies process-level global state.
	// If multiple Client instances with different Plural settings exist in the
	// same process, the last one wins. Consider using bun.BaseModel table tag
	// on individual models instead of relying on this global setting.
	if !config.Plural {
		logger.Warn("WARNING: schema.SetTableNameInflector modifies process-level global state. Multiple Client instances with different Plural settings will override each other. Consider using bun.BaseModel table tag on individual models instead.")
		schema.SetTableNameInflector(func(tableName string) string {
			return tableName
		})
	}

	if config.EnableMetric != nil && *config.EnableMetric {
		ensureMetricsRegistered()
		ctx, cancelFunc := context.WithCancel(context.Background())
		c.cancel = cancelFunc
		c.metrics = newMetricsCollector(&config)
		c.metrics.startPoolStats(ctx, c.DB)
	}

	global.DefaultResourceManger.AddWithOrder(c, 1000)
	return c, nil
}

func configureConnectionPool(db *sql.DB, config *Config) {
	db.SetConnMaxLifetime(config.ConnMaxLifetime)
	db.SetConnMaxIdleTime(config.ConnMaxIdleTime)
	db.SetMaxOpenConns(config.MaxOpen)
	db.SetMaxIdleConns(config.MaxIdle)
}

func createMysqlClient(config *Config) (*Client, error) {
	password := config.Password
	if config.EnablePasswordEncrypt {
		_tmpPassword, err := tools.AesDecryptRawBase64(password)
		if err != nil {
			return nil, fmt.Errorf("mysql: decrypt password failed: %w", err)
		}
		password = _tmpPassword
	}
	dns := fmt.Sprintf("%s:%s@tcp(%s)/%s?charset=%s", config.User, password, config.Url, config.DbName, config.Charset)
	db, err := sql.Open("mysql", dns)
	if err != nil {
		return nil, fmt.Errorf("mysql: open database failed: %w", err)
	}

	configureConnectionPool(db, config)
	ctx, cancelFunc := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelFunc()
	if err = db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("connect to db timeout: %w", err)
	}

	bunDB := bun.NewDB(db, mysqldialect.New())
	return &Client{DB: bunDB, config: config}, nil
}

func createPgsqlClient(config *Config) (*Client, error) {
	password := config.Password
	if config.EnablePasswordEncrypt {
		_tmpPassword, err := tools.AesDecryptRawBase64(password)
		if err != nil {
			return nil, fmt.Errorf("pgsql: decrypt password failed: %w", err)
		}
		password = _tmpPassword
	}
	dsn := fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=disable",
		config.User, password, config.Url, config.DbName)
	db := sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(dsn)))

	configureConnectionPool(db, config)
	ctx, cancelFunc := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelFunc()
	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("connect to db timeout: %w", err)
	}

	bunDB := bun.NewDB(db, pgdialect.New())
	return &Client{DB: bunDB, config: config}, nil
}

func createSqliteClient(config *Config) (*Client, error) {
	db, err := sql.Open("sqlite3", config.Url)
	if err != nil {
		return nil, fmt.Errorf("sqlite: open database failed: %w", err)
	}

	logger.Info("sqlite: overriding connection pool settings for single-writer model (MaxOpen=1, MaxIdle=1, ConnMaxLifetime=0, ConnMaxIdleTime=0)")
	db.SetConnMaxLifetime(0)
	db.SetConnMaxIdleTime(0)
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	bunDB := bun.NewDB(db, sqlitedialect.New())
	return &Client{DB: bunDB, config: config}, nil
}

func (c *Client) GetDB() *bun.DB {
	return c.DB
}

func (c *Client) GetTx(tx ...*bun.Tx) bun.IDB {
	if len(tx) == 0 || tx[0] == nil {
		return c.DB
	}
	return tx[0]
}

func (c *Client) Begin(ctx context.Context) (*bun.Tx, context.CancelFunc, error) {
	ctx, cancel := context.WithTimeout(ctx, c.config.TransactionTimeout)
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

// Order returns the close order for graceful shutdown.
// DB clients should close after Kafka consumers (lower order) to ensure
// consumers can finish processing messages that may need DB access.
func (c *Client) Order() int {
	return 1000
}

// GetSelect returns a select query with a hardcoded "status>0" filter.
// NOTE: This assumes all models have a "status" column. For models without
// this column, use c.GetDB().NewSelect().Model(model) directly.
func (c *Client) GetSelect(model interface{}) *bun.SelectQuery {
	return c.GetDB().NewSelect().Model(model).Where("status>0")
}

func (c *Client) GetInsert(model interface{}, tx ...*bun.Tx) *bun.InsertQuery {
	return c.GetTx(tx...).NewInsert().Model(model)
}

func (c *Client) GetUpdate(model interface{}, tx ...*bun.Tx) *bun.UpdateQuery {
	return c.GetTx(tx...).NewUpdate().Model(model)
}

func (c *Client) GetDelete(model interface{}, tx ...*bun.Tx) *bun.DeleteQuery {
	return c.GetTx(tx...).NewDelete().Model(model)
}

// GetSoftDelete returns an update query that sets "status=-1" for soft deletion.
// NOTE: This assumes all models have a "status" column. For models without
// this column, use c.GetTx(tx...).NewUpdate().Model(model) directly.
func (c *Client) GetSoftDelete(model interface{}, tx ...*bun.Tx) *bun.UpdateQuery {
	return c.GetTx(tx...).NewUpdate().Model(model).Set("status=-1")
}

func (c *Client) Insert(ctx context.Context, data interface{}, tx ...*bun.Tx) (int64, error) {
	return c.GetRowsAffected(c.GetTx(tx...).NewInsert().Model(data).Exec(ctx))
}

func (c *Client) SoftDelete(ctx context.Context, model interface{}, id interface{}, tx ...*bun.Tx) (int64, error) {
	if id == nil {
		return 0, fmt.Errorf("SoftDelete: id must not be nil")
	}
	handler := c.GetSoftDelete(model, tx...)
	if reflect.TypeOf(id).Kind() == reflect.Slice {
		handler.Where("id in (?)", bun.In(id))
	} else {
		handler.Where("id=?", id)
	}
	return c.GetRowsAffected(handler.Exec(ctx))
}

func (c *Client) Delete(ctx context.Context, model interface{}, id interface{}, tx ...*bun.Tx) (int64, error) {
	if id == nil {
		return 0, fmt.Errorf("Delete: id must not be nil")
	}
	handler := c.GetDelete(model, tx...)
	if reflect.TypeOf(id).Kind() == reflect.Slice {
		handler.Where("id in (?)", bun.In(id))
	} else {
		handler.Where("id=?", id)
	}
	return c.GetRowsAffected(handler.Exec(ctx))
}

// QueryAll queries all records with default ordering by "id desc".
// NOTE: This assumes all models have an "id" column. For models without
// this column, use GetSelect(result) or a custom query directly.
func (c *Client) QueryAll(ctx context.Context, result interface{}) (int, error) {
	return c.GetSelect(result).Order("id desc").ScanAndCount(GetContextWithTraceID(ctx), result)
}

func (c *Client) QueryPage(ctx context.Context, result interface{}, filter Filter) (int, error) {
	if filter != nil {
		offset, limit, disable := filter.GetPage()
		if !disable {
			return filter.Filter(c.GetDB()).Model(result).Offset(offset).Limit(limit).ScanAndCount(GetContextWithTraceID(ctx), result)
		}

		return filter.Filter(c.GetDB()).Model(result).ScanAndCount(GetContextWithTraceID(ctx), result)
	}

	return c.QueryAll(ctx, result)
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
	duration := time.Since(event.StartTime)

	if c.metrics != nil && !isMetricsQuery(event.Query) {
		c.metrics.recordQuery(event.Err, duration)
	}

	if c.config.Debug {
		if requestId, trace := golocalv1.GetTraceID(), ctx.Value(traceId); requestId == "" && trace != nil {
			if tid, ok := trace.(string); ok {
				golocalv1.PutTraceID(tid)
			}
		}
		rows := ""
		if event.Result != nil {
			row, _ := event.Result.RowsAffected()
			rows = fmt.Sprintf(". rows_affected=%d.", row)
		}
		if event.Err == nil {
			logger.Debug("SqlTrace -> %v. cost=%v%s", event.Query, duration, rows)
		} else {
			logger.Error("SqlTrace -> %v. cost=%v%s. err=%v", event.Query, duration, rows, event.Err)
		}
	}
}

// GetContext getContext with traceId
func GetContextWithTraceID(ctx context.Context) context.Context {
	return context.WithValue(ctx, traceId, golocalv1.GetTraceID())
}

var passwordRe = regexp.MustCompile(`(?i)"(password|pass|secret|token)"\s*:\s*"[^"]*"`)

func maskConfigPassword(jsonStr string) string {
	return passwordRe.ReplaceAllString(jsonStr, `"$1":"******"`)
}
