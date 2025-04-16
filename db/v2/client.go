package dbv2

import (
	"fmt"
	"reflect"

	"entgo.io/ent/entc/integration/ent"
	"github.com/caiflower/common-tools/global"
	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/caiflower/common-tools/pkg/tools"
)

type Client struct {
	client *ent.Client
}

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

	global.DefaultResourceManger.Add(c)
	return c, nil
}

func createMysqlClient(config *Config) (c *Client, err error) {
	client, err := ent.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s)/%s?charset=%s", config.User, config.Password, config.Url, config.DbName, config.Charset))

	if err != nil {
		logger.Error("ent Open db client failed. err: %v", err)
		return nil, err
	}

	c = &Client{}
	c.client = client
	return
}

func (c *Client) Close() {

}
