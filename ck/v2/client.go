package v2

import (
	"database/sql"
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
