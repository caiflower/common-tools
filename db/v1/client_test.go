package v1

import (
	"fmt"
	"testing"
	"time"

	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/caiflower/common-tools/pkg/tools"
)

type ContainerRegistry struct {
	RegistryName string
	UserName     string
	Pass         string
	SecretName   string
	ExpireTime   string
	Server       string
	Id           int       //主键
	CreateTime   time.Time //创建时间
	UpdateTime   time.Time //更新时间
	Status       int       //状态
}

func TestNewDBClient(t *testing.T) {
	config := Config{
		Url:          "10.226.138.71:3306",
		User:         "root",
		Password:     "admin",
		DbName:       "fc-placement",
		Debug:        true,
		EnableMetric: true,
	}

	l := logger.Config{
		Level: logger.DebugLevel,
	}

	logger.InitLogger(&l)

	client, err := NewDBClient(config)
	if err != nil {
		panic(err)
	}

	var containerRegistry []ContainerRegistry
	count, err := client.QueryAll(&containerRegistry)
	if err != nil {
		panic(err)
	}

	for _, v := range containerRegistry {
		fmt.Println("v = " + tools.ToJson(v))
	}

	fmt.Println(count)

	time.Sleep(100 * time.Second)
}
