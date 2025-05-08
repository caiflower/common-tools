package dbv1

import (
	"database/sql"
	"errors"
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
		Url:          "127.0.0.1:3306",
		User:         "root",
		Password:     "admin",
		DbName:       "test",
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

func TestTransactionTimeout(t *testing.T) {
	transactionTimeout := time.Second * 5

	config := Config{
		Url:                "127.0.0.1:3306",
		User:               "root",
		Password:           "admin",
		DbName:             "test",
		Debug:              true,
		EnableMetric:       true,
		TransactionTimeout: transactionTimeout,
	}

	l := logger.Config{
		Level: logger.DebugLevel,
	}

	logger.InitLogger(&l)

	client, err := NewDBClient(config)
	if err != nil {
		panic(err)
	}

	tx, cancel, err := client.Begin()
	if err != nil {
		return
	}
	defer cancel()
	defer tx.Commit()

	var containerRegistry []ContainerRegistry
	containerRegistry = append(containerRegistry, ContainerRegistry{
		RegistryName: "Test",
		UserName:     "root",
		Pass:         "test",
		SecretName:   "test",
		ExpireTime:   "2024-10-31 20:17:42",
		Server:       "test",
		CreateTime:   time.Now(),
		UpdateTime:   time.Now(),
		Status:       1,
	})

	//// 超时
	time.Sleep(transactionTimeout + time.Second)

	_, err = client.Insert(&containerRegistry, tx)
	if err != nil && errors.Is(err, sql.ErrTxDone) {
		logger.Info("test transaction timeout successfully")
	} else {
		logger.Error("test failed. Error: %v", err)
	}
}
