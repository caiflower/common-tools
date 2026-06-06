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
	"fmt"
	"time"
)

type Config struct {
	Name                  string        `yaml:"name" json:"name"`
	Dialect               string        `yaml:"dialect" default:"mysql" json:"dialect"`
	Url                   string        `yaml:"url" default:"127.0.0.1:3306" json:"url"`
	DbName                string        `yaml:"dbName" json:"dbName"`
	User                  string        `yaml:"user" json:"user"`
	Password              string        `yaml:"password" json:"password"`
	EnablePasswordEncrypt bool          `yaml:"enablePasswordEncrypt" json:"enablePasswordEncrypt"`
	Charset               string        `yaml:"charset" default:"utf8mb4" json:"charset"`
	MaxOpen               int           `yaml:"maxOpen" default:"200" json:"maxOpen"`
	MaxIdle               int           `yaml:"maxIdle" default:"20" json:"maxIdle"`
	ConnMaxLifetime       time.Duration `yaml:"connMaxLifetime" json:"connMaxLifetime" default:"28800s"`
	ConnMaxIdleTime       time.Duration `yaml:"connMaxIdleTime" json:"connMaxIdleTime" default:"60s"`
	Plural                bool          `yaml:"plural" json:"plural"`
	Debug                 bool          `yaml:"debug" json:"debug"`
	EnableMetric          *bool         `yaml:"enableMetric" json:"enableMetric" default:"true"`
	TransactionTimeout    time.Duration `yaml:"transactionTimeout" json:"transactionTimeout" default:"30s"`
}

func (c *Config) Validate() error {
	validDialects := map[string]bool{"mysql": true, "pgsql": true, "sqlite": true}
	if c.Dialect == "" {
		return fmt.Errorf("dialect is required")
	}
	if !validDialects[c.Dialect] {
		return fmt.Errorf("dialect must be one of mysql, pgsql, sqlite, got %s", c.Dialect)
	}
	if c.Dialect != "sqlite" {
		if c.DbName == "" {
			return fmt.Errorf("dbName is required when dialect is not sqlite")
		}
		if c.Url == "" {
			return fmt.Errorf("url is required when dialect is not sqlite")
		}
	}
	if c.TransactionTimeout <= 0 {
		return fmt.Errorf("transactionTimeout must be greater than 0")
	}
	if c.MaxOpen <= 0 {
		return fmt.Errorf("maxOpen must be greater than 0")
	}
	if c.MaxIdle < 0 {
		return fmt.Errorf("maxIdle must not be negative")
	}
	if c.MaxIdle > c.MaxOpen {
		return fmt.Errorf("maxIdle must not exceed maxOpen")
	}
	if c.ConnMaxLifetime < 0 {
		return fmt.Errorf("connMaxLifetime must not be negative")
	}
	if c.ConnMaxIdleTime < 0 {
		return fmt.Errorf("connMaxIdleTime must not be negative")
	}
	return nil
}
