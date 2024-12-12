package v2

import (
	"database/sql"
	"fmt"
	"reflect"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/caiflower/common-tools/pkg/tools"
)

type Config struct {
	User            string        `yaml:"user"`
	Password        string        `yaml:"password"`
	Urls            []string      `yaml:"urls"`
	DbName          string        `yaml:"dbName"`
	MaxIdleConns    int           `yaml:"max_idle_conns" default:"50"`
	MaxOpenConns    int           `yaml:"max_open_conns" default:"200"`
	MaxIdleTime     time.Duration `yaml:"maxIdleTime" default:"80s"`
	ConnMaxLifetime time.Duration `yaml:"connMaxLifetime" default:"500s"`
	ReadTimeout     time.Duration `yaml:"read_timeout" default:"5s"`
	Debug           bool          `yaml:"debug"`
	Plural          bool          `json:"plural"`
}

type IClickHouseDB interface {
	GetDB() *sql.DB

	NewInsert() *InsertQuery
}

type Client struct {
	db     *sql.DB
	config *Config
}

func NewClient(config Config) IClickHouseDB {
	if err := tools.DoTagFunc(&config, nil, []func(reflect.StructField, reflect.Value, interface{}) error{tools.SetDefaultValueIfNil}); err != nil {
		logger.Warn("Clickhouse set default config failed. err: %s", err.Error())
	}

	conn := clickhouse.OpenDB(&clickhouse.Options{
		Addr: config.Urls,
		Auth: clickhouse.Auth{
			Database: config.DbName,
			Username: config.User,
			Password: config.Password,
		},
		Settings: clickhouse.Settings{
			"max_execution_time": 60,
		},
		DialTimeout: 5 * time.Second,
		Compression: &clickhouse.Compression{
			Method: clickhouse.CompressionLZ4,
		},
		Debug: config.Debug,
	})
	conn.SetMaxIdleConns(config.MaxIdleConns)
	conn.SetMaxOpenConns(config.MaxOpenConns)
	conn.SetConnMaxLifetime(config.ConnMaxLifetime)

	if err := conn.Ping(); err != nil {
		logger.Warn("Clickhouse ping failed. err: %s", err.Error())
	}

	return &Client{db: conn, config: &config}
}

func (c *Client) GetDB() *sql.DB {
	return c.db
}

func (c *Client) NewInsert() *InsertQuery {
	return &InsertQuery{
		db: c.db,
	}
}

type InsertQuery struct {
	db    *sql.DB
	table string

	sqlErr error
}

func (q *InsertQuery) Model(v interface{}) *InsertQuery {
	of := reflect.TypeOf(v)
	switch of.Kind() {
	case reflect.Ptr:
		q.table = tools.ToUnderscore(of.Elem().Name())
	case reflect.Struct:
		q.table = tools.ToUnderscore(of.Name())
	case reflect.Slice:
		if of.Elem().Kind() == reflect.Ptr {
			q.table = tools.ToUnderscore(of.Elem().Elem().Name())
		} else if of.Elem().Kind() == reflect.Struct {
			q.table = tools.ToUnderscore(of.Elem().Name())
		}
	default:
		q.sqlErr = fmt.Errorf("invalid model type: %s", of.Kind().String())
	}

	return q
}

func (q *InsertQuery) Exec(v interface{}) (int64, error) {
	if q.sqlErr != nil {
		return 0, q.sqlErr
	}

	of := reflect.TypeOf(v)
	switch of.Kind() {
	case reflect.Ptr:
		//conn, err := q.db.Prepare("INSERT INTO " + q.table)
		//if err != nil {
		//	q.sqlErr = err
		//	return 0, q.sqlErr
		//}

		//value := reflect.ValueOf(v)
		//for i := 0; i < of.Elem().NumField(); i++ {
		//
		//}

		//if result, err := q.db.Exec(""); err != nil {
		//	q.sqlErr = err
		//	return 0, q.sqlErr
		//} else {
		//	return result.RowsAffected()
		//}
	case reflect.Struct:

	case reflect.Slice:
		//begin, err := q.db.Begin()
		//if err != nil {
		//	return 0, err
		//}
		//if of.Elem().Kind() == reflect.Ptr {
		//	q.table = tools.ToUnderscore(of.Elem().Elem().Name())
		//} else if of.Elem().Kind() == reflect.Struct {
		//	q.table = tools.ToUnderscore(of.Elem().Name())
		//}
	default:
		q.sqlErr = fmt.Errorf("invalid model type: %s", of.Kind().String())
	}

	return 0, q.sqlErr
}
