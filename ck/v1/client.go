package v1

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/caiflower/common-tools/pkg/tools"
	"github.com/uptrace/go-clickhouse/ch"
	"github.com/uptrace/go-clickhouse/ch/chschema"
	"github.com/uptrace/go-clickhouse/chdebug"
)

type Config struct {
	User            string        `yaml:"user"`
	Password        string        `yaml:"password"`
	Url             string        `yaml:"url"`
	DbName          string        `yaml:"dbName"`
	MaxIdleTime     time.Duration `yaml:"maxIdleTime" default:"80s"`
	ConnMaxLifetime time.Duration `yaml:"connMaxLifetime" default:"500s"`
	Timeout         time.Duration `yaml:"timeout" default:"5s"`
	PoolSize        int           `yaml:"pool_size" default:"200"`
	Debug           bool          `yaml:"debug"`
	Plural          bool          `json:"plural"`
}

type Client struct {
	db *ch.DB
}

func NewClient(config Config) *Client {
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
	db.AddQueryHook(chdebug.NewQueryHook(chdebug.WithVerbose(true)))
	if err := db.Ping(context.Background()); err != nil {
		logger.Error("Clickhouse ping failed. err: %s", err.Error())
	}

	if !config.Plural {
		chschema.SetTableNameInflector(func(tableName string) string {
			return tableName
		})
	}

	return &Client{
		db: db,
	}
}
