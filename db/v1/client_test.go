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
	"testing"
	"time"

	golocalv1 "github.com/caiflower/common-tools/pkg/golocal/v1"
	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/caiflower/common-tools/pkg/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		Url:      "127.0.0.1:3306",
		User:     "root",
		Password: "admin",
		DbName:   "test",
		Debug:    true,
	}

	l := logger.Config{
		Level: logger.DebugLevel,
	}

	logger.InitLogger(&l)

	client, err := NewDBClient(config)
	if err != nil {
		fmt.Println("connect failed. Skip TestNewDBClient")
		t.Skip()
	}

	var containerRegistry []ContainerRegistry
	count, err := client.QueryAll(golocalv1.GetContext(), &containerRegistry)
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
		TransactionTimeout: transactionTimeout,
	}

	l := logger.Config{
		Level: logger.DebugLevel,
	}

	logger.InitLogger(&l)

	client, err := NewDBClient(config)
	if err != nil {
		fmt.Println("connect failed. Skip TestNewDBClient")
		t.Skip()
	}

	tx, cancel, err := client.Begin(golocalv1.GetContext())
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

	_, err = client.Insert(golocalv1.GetContext(), &containerRegistry, tx)
	if err != nil && errors.Is(err, sql.ErrTxDone) {
		logger.Info("test transaction timeout successfully")
	} else {
		logger.Error("test failed. Error: %v", err)
	}
}

func TestNewDBClient_UnsupportedDialect(t *testing.T) {
	cfg := Config{
		Dialect:            "oracle",
		Url:                "127.0.0.1:3306",
		DbName:             "test",
		MaxOpen:            200,
		MaxIdle:            20,
		TransactionTimeout: 30 * time.Second,
	}
	_, err := NewDBClient(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "dialect must be one of")
}

func TestNewDBClient_InvalidConfig(t *testing.T) {
	cfg := Config{}
	_, err := NewDBClient(cfg)
	assert.Error(t, err)
}

func TestClient_ParseErr(t *testing.T) {
	cfg := Config{
		Dialect:            "sqlite",
		Url:                ":memory:",
		MaxOpen:            1,
		MaxIdle:            1,
		TransactionTimeout: 30 * time.Second,
	}
	client, err := NewDBClient(cfg)
	require.NoError(t, err)
	defer client.Close()

	assert.Nil(t, client.ParseErr(sql.ErrNoRows))

	otherErr := errors.New("some db error")
	assert.Equal(t, otherErr, client.ParseErr(otherErr))
}

func TestClient_GetTx_NoTx(t *testing.T) {
	cfg := Config{
		Dialect:            "sqlite",
		Url:                ":memory:",
		MaxOpen:            1,
		MaxIdle:            1,
		TransactionTimeout: 30 * time.Second,
	}
	client, err := NewDBClient(cfg)
	require.NoError(t, err)
	defer client.Close()

	result := client.GetTx()
	assert.Equal(t, client.DB, result)
}

func TestClient_GetTx_WithTx(t *testing.T) {
	cfg := Config{
		Dialect:            "sqlite",
		Url:                ":memory:",
		MaxOpen:            1,
		MaxIdle:            1,
		TransactionTimeout: 30 * time.Second,
	}
	client, err := NewDBClient(cfg)
	require.NoError(t, err)
	defer client.Close()

	tx, err := client.DB.BeginTx(context.Background(), nil)
	require.NoError(t, err)
	defer tx.Rollback()

	result := client.GetTx(&tx)
	assert.Equal(t, &tx, result)
}

func TestClient_GetRowsAffected(t *testing.T) {
	cfg := Config{
		Dialect:            "sqlite",
		Url:                ":memory:",
		MaxOpen:            1,
		MaxIdle:            1,
		TransactionTimeout: 30 * time.Second,
	}
	client, err := NewDBClient(cfg)
	require.NoError(t, err)
	defer client.Close()

	result := &mockSQLResult{rowsAffected: 5}
	affected, err := client.GetRowsAffected(result, nil)
	assert.NoError(t, err)
	assert.Equal(t, int64(5), affected)

	dbErr := errors.New("exec error")
	_, err = client.GetRowsAffected(nil, dbErr)
	assert.Equal(t, dbErr, err)
}

func TestClient_Order(t *testing.T) {
	cfg := Config{
		Dialect:            "sqlite",
		Url:                ":memory:",
		MaxOpen:            1,
		MaxIdle:            1,
		TransactionTimeout: 30 * time.Second,
	}
	client, err := NewDBClient(cfg)
	require.NoError(t, err)
	defer client.Close()

	assert.Equal(t, 1000, client.Order())
}

func TestNewDBClient_Sqlite(t *testing.T) {
	cfg := Config{
		Dialect:            "sqlite",
		Url:                ":memory:",
		MaxOpen:            1,
		MaxIdle:            1,
		TransactionTimeout: 30 * time.Second,
	}
	client, err := NewDBClient(cfg)
	require.NoError(t, err)
	defer client.Close()

	assert.NotNil(t, client.DB)

	_, err = client.DB.NewCreateTable().Model((*testItem)(nil)).Exec(context.Background())
	require.NoError(t, err)

	_, err = client.Insert(context.Background(), &testItem{Name: "hello"})
	require.NoError(t, err)

	var items []testItem
	count, err := client.DB.NewSelect().Model(&items).ScanAndCount(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, count)
	assert.Equal(t, "hello", items[0].Name)
}

type mockSQLResult struct {
	rowsAffected int64
}

func (m *mockSQLResult) LastInsertId() (int64, error) { return 0, nil }
func (m *mockSQLResult) RowsAffected() (int64, error) { return m.rowsAffected, nil }
