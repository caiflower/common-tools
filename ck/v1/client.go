// +build go1.18

package v1

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"time"

	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/caiflower/common-tools/pkg/tools"
	"github.com/uptrace/go-clickhouse/ch"
	"github.com/uptrace/go-clickhouse/ch/chschema"
)

/**
 * 基于tcp原生接口实现的ck客户端，go > 1.18
 * ck client use tcp native protocol
 * https://clickhouse.com/docs/zh/interfaces/tcp
 */

type Config struct {
	User            string        `yaml:"user"`
	Password        string        `yaml:"password"`
	Url             string        `yaml:"url"`
	DbName          string        `yaml:"dbName"`
	MaxIdleTime     time.Duration `yaml:"maxIdleTime" default:"80s"`
	ConnMaxLifetime time.Duration `yaml:"connMaxLifetime" default:"500s"`
	Timeout         time.Duration `yaml:"timeout" default:"5s"`
	PoolSize        int           `yaml:"pool_size" default:"200"`
	Compress        bool          `yaml:"compress"`
	Debug           bool          `yaml:"debug"`
	Plural          bool          `json:"plural"`
}

type IClickHouseDB interface {
	GetDB() *ch.DB
	GetSelect(model interface{}) *ch.SelectQuery // 获得通用处理器：查询
	GetInsert(model interface{}) *ch.InsertQuery // 获得通用处理器：写入

	Insert(data interface{}) (int64, error)                       // 新增
	QueryAll(result interface{}) (int, error)                     // 查询所有
	QueryPage(result interface{}, filter DataFilter) (int, error) //通用处理：分页查询

	TruncateTable(model interface{}) error                       // 清空表
	DropTable(model interface{}) error                           // 删除表
	GetCreateTable(model interface{}) *ch.CreateTableQuery       // 创建表
	GetRowsAffected(result sql.Result, err error) (int64, error) // 获取受影响的行数
	Close()                                                      // 关闭
}

type DataFilter interface {
	GetPage() (offset int, limit int, disable bool) //分页偏移量计算，disable=true表示禁用分页
	Filter(db *ch.DB) *ch.SelectQuery               //查询时获取过滤器
	GetOrders() []string                            //获得排序规则
}

type Client struct {
	db     *ch.DB
	config *Config
}

func NewClient(config Config) IClickHouseDB {
	if err := tools.DoTagFunc(&config, nil, []func(reflect.StructField, reflect.Value, interface{}) error{tools.SetDefaultValueIfNil}); err != nil {
		logger.Warn("Clickhouse set default config failed. err: %s", err.Error())
	}

	db := ch.Connect(
		// clickhouse://<user>:<password>@<host>:<port>/<database>?sslmode=disable
		ch.WithDSN(fmt.Sprintf("clickhouse://%s:%s@%s/%s?sslmode=disable", config.User, config.Password, config.Url, config.DbName)),
		ch.WithTimeout(config.Timeout),
		ch.WithConnMaxIdleTime(config.MaxIdleTime),
		ch.WithConnMaxLifetime(config.MaxIdleTime),
		ch.WithPoolSize(config.PoolSize),
		ch.WithCompression(config.Compress),
	)
	//db := ch.Connect(
	//	ch.WithUser(config.User),
	//	ch.WithPassword(config.Password),
	//	ch.WithDatabase(config.DbName),
	//	ch.WithTimeout(config.Timeout),
	//	ch.WithConnMaxIdleTime(config.MaxIdleTime),
	//	ch.WithConnMaxLifetime(config.MaxIdleTime),
	//	ch.WithPoolSize(config.PoolSize),
	//	ch.WithTLSConfig(&tls.Config{InsecureSkipVerify: true}),
	//)
	c := &Client{
		db:     db,
		config: &config,
	}
	if config.Debug {
		db.AddQueryHook(c)
	}
	if !config.Plural {
		chschema.SetTableNameInflector(func(tableName string) string {
			return tableName
		})
	}

	// ping
	if err := db.Ping(context.Background()); err != nil {
		logger.Error("Clickhouse ping failed. err: %s", err.Error())
	}

	return c
}

func (c *Client) BeforeQuery(ctx context.Context, event *ch.QueryEvent) context.Context {
	return ctx
}

func (c *Client) AfterQuery(ctx context.Context, event *ch.QueryEvent) {
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

func (c *Client) GetDB() *ch.DB {
	return c.db
}

func (c *Client) GetSelect(model interface{}) *ch.SelectQuery {
	return c.db.NewSelect().Model(model)
}

func (c *Client) GetInsert(model interface{}) *ch.InsertQuery {
	return c.db.NewInsert().Model(model)
}

func (c *Client) TruncateTable(model interface{}) error {
	_, err := c.db.NewTruncateTable().Model(model).Exec(context.TODO())
	return err
}

func (c *Client) DropTable(model interface{}) error {
	_, err := c.db.NewDropTable().Model(model).Exec(context.TODO())
	return err
}

func (c *Client) GetCreateTable(model interface{}) *ch.CreateTableQuery {
	return c.db.NewCreateTable().Model(model)
}

func (c *Client) QueryAll(result interface{}) (int, error) {
	return c.GetSelect(result).ScanAndCount(context.TODO(), result)
}

func (c *Client) QueryPage(result interface{}, filter DataFilter) (int, error) {
	offset, limit, disable := filter.GetPage()

	// 禁用分页
	if disable {
		count := 0
		err := filter.Filter(c.GetDB()).Order(filter.GetOrders()...).Scan(context.Background(), result)
		v := reflect.ValueOf(result)
		if v.Kind() == reflect.Ptr {
			v = v.Elem()
		}
		if v.Kind() == reflect.Slice {
			count = v.Len()
		}
		return count, err
	}

	// 分页
	return filter.Filter(c.GetDB()).Order(filter.GetOrders()...).Offset(offset).Limit(limit).ScanAndCount(context.Background(), result)
}

func (c *Client) Insert(data interface{}) (int64, error) {
	return c.GetRowsAffected(c.GetInsert(data).Exec(context.TODO()))
}

func (c *Client) GetRowsAffected(result sql.Result, err error) (int64, error) {
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (c *Client) Close() {
	c.db.Close()
}
