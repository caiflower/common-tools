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

package redis

import "time"

const (
	ClusterMode = "cluster"
)

// Config holds Redis client configuration shared across all versions.
type Config struct {
	Name                  string        `yaml:"name" json:"name"`
	Mode                  string        `yaml:"mode" json:"mode"`
	Addrs                 []string      `yaml:"addrs" json:"addrs"`
	Password              string        `yaml:"password" json:"password"`
	EnablePasswordEncrypt bool          `yaml:"enablePasswordEncrypt" json:"enablePasswordEncrypt"`
	DB                    int           `yaml:"db" json:"db"`
	ReadTimeout           time.Duration `yaml:"readTimeout" default:"10s" json:"readTimeout"`
	WriteTimeout          time.Duration `yaml:"writeTimeout" default:"20s" json:"writeTimeout"`
	PoolSize              int           `yaml:"poolSize" json:"poolSize"`
	MinIdleConns          int           `yaml:"minIdleConns" default:"20" json:"minIdleConns"`
	MaxConnAge            time.Duration `yaml:"maxConnAge" default:"1800s" json:"maxConnAge"`  // v8 name, use ConnMaxLifetime for new configs
	IdleTimeout           time.Duration `yaml:"idleTimeout" default:"300s" json:"idleTimeout"` // v8 name, use ConnMaxIdleTime for new configs
	ConnMaxLifetime       time.Duration `yaml:"connMaxLifetime" json:"connMaxLifetime"`        // v9 alias for MaxConnAge, no default: zero sentinel enables fallback to MaxConnAge
	ConnMaxIdleTime       time.Duration `yaml:"connMaxIdleTime" json:"connMaxIdleTime"`        // v9 alias for IdleTimeout, no default: zero sentinel enables fallback to IdleTimeout
	DisableIdentity       bool          `yaml:"disableIdentity" json:"disableIdentity"`        // v9 only
	KeyPrefix             string        `yaml:"keyPrefix" json:"keyPrefix"`
	EnableMetrics         string        `yaml:"enableMetrics" default:"true" json:"enableMetrics"`
}
