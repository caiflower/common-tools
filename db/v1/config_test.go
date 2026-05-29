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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func validMySQLConfig() Config {
	return Config{
		Dialect:            "mysql",
		Url:                "127.0.0.1:3306",
		DbName:             "test_db",
		User:               "root",
		Password:           "secret",
		MaxOpen:            200,
		MaxIdle:            20,
		ConnMaxLifetime:    28800 * time.Second,
		ConnMaxIdleTime:    60 * time.Second,
		TransactionTimeout: 30 * time.Second,
	}
}

func validSQLiteConfig() Config {
	return Config{
		Dialect:            "sqlite",
		Url:                ":memory:",
		MaxOpen:            1,
		MaxIdle:            1,
		TransactionTimeout: 30 * time.Second,
	}
}

func TestConfig_Validate_ValidMySQL(t *testing.T) {
	cfg := validMySQLConfig()
	assert.NoError(t, cfg.Validate())
}

func TestConfig_Validate_ValidSQLite(t *testing.T) {
	cfg := validSQLiteConfig()
	assert.NoError(t, cfg.Validate())
}

func TestConfig_Validate_SQLiteEmptyDbNameAndUrl(t *testing.T) {
	cfg := Config{
		Dialect:            "sqlite",
		MaxOpen:            1,
		MaxIdle:            1,
		TransactionTimeout: 30 * time.Second,
	}
	assert.NoError(t, cfg.Validate())
}

func TestConfig_Validate_EmptyDialect(t *testing.T) {
	cfg := validMySQLConfig()
	cfg.Dialect = ""
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "dialect is required")
}

func TestConfig_Validate_UnsupportedDialect(t *testing.T) {
	cfg := validMySQLConfig()
	cfg.Dialect = "oracle"
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "dialect must be one of mysql, pgsql, sqlite")
}

func TestConfig_Validate_MySQLEmptyDbName(t *testing.T) {
	cfg := validMySQLConfig()
	cfg.DbName = ""
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "dbName is required when dialect is not sqlite")
}

func TestConfig_Validate_MySQLEmptyUrl(t *testing.T) {
	cfg := validMySQLConfig()
	cfg.Url = ""
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "url is required when dialect is not sqlite")
}

func TestConfig_Validate_PgsqlEmptyDbName(t *testing.T) {
	cfg := validMySQLConfig()
	cfg.Dialect = "pgsql"
	cfg.DbName = ""
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "dbName is required when dialect is not sqlite")
}

func TestConfig_Validate_PgsqlEmptyUrl(t *testing.T) {
	cfg := validMySQLConfig()
	cfg.Dialect = "pgsql"
	cfg.Url = ""
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "url is required when dialect is not sqlite")
}

func TestConfig_Validate_TransactionTimeoutZero(t *testing.T) {
	cfg := validMySQLConfig()
	cfg.TransactionTimeout = 0
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "transactionTimeout must be greater than 0")
}

func TestConfig_Validate_TransactionTimeoutNegative(t *testing.T) {
	cfg := validMySQLConfig()
	cfg.TransactionTimeout = -1 * time.Second
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "transactionTimeout must be greater than 0")
}

func TestConfig_Validate_MaxOpenZero(t *testing.T) {
	cfg := validMySQLConfig()
	cfg.MaxOpen = 0
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "maxOpen must be greater than 0")
}

func TestConfig_Validate_MaxOpenNegative(t *testing.T) {
	cfg := validMySQLConfig()
	cfg.MaxOpen = -1
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "maxOpen must be greater than 0")
}

func TestConfig_Validate_MaxIdleNegative(t *testing.T) {
	cfg := validMySQLConfig()
	cfg.MaxIdle = -1
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "maxIdle must not be negative")
}

func TestConfig_Validate_MaxIdleExceedsMaxOpen(t *testing.T) {
	cfg := validMySQLConfig()
	cfg.MaxOpen = 10
	cfg.MaxIdle = 20
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "maxIdle must not exceed maxOpen")
}

func TestConfig_Validate_ConnMaxLifetimeNegative(t *testing.T) {
	cfg := validMySQLConfig()
	cfg.ConnMaxLifetime = -1 * time.Second
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "connMaxLifetime must not be negative")
}

func TestConfig_Validate_ConnMaxIdleTimeNegative(t *testing.T) {
	cfg := validMySQLConfig()
	cfg.ConnMaxIdleTime = -1 * time.Second
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "connMaxIdleTime must not be negative")
}

func TestConfig_Validate_ZeroValuesAllowed(t *testing.T) {
	cfg := Config{
		Dialect:            "sqlite",
		MaxOpen:            1,
		MaxIdle:            0,
		ConnMaxLifetime:    0,
		ConnMaxIdleTime:    0,
		TransactionTimeout: 30 * time.Second,
	}
	assert.NoError(t, cfg.Validate())
}
