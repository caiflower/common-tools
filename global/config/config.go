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

package config

import (
	"github.com/caiflower/common-tools/cluster"
	dbv1 "github.com/caiflower/common-tools/db/v1"
	"github.com/caiflower/common-tools/global/env"
	xkafka "github.com/caiflower/common-tools/kafka"
	"github.com/caiflower/common-tools/pkg/bean"
	"github.com/caiflower/common-tools/pkg/http"
	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/caiflower/common-tools/pkg/tools"
	redisv1 "github.com/caiflower/common-tools/redis/v1"
	"github.com/caiflower/common-tools/telemetry"
)

type DefaultConfig struct {
	LoggerConfig     logger.Config    `yaml:"logger" json:"logger"`
	ClusterConfig    cluster.Config   `yaml:"cluster" json:"cluster"`
	DatabaseConfig   []dbv1.Config    `yaml:"database" json:"database"`
	HttpClientConfig http.Config      `yaml:"http_client" json:"http_client"`
	RedisConfig      []redisv1.Config `yaml:"redis" json:"redis"`
	TelemetryConfig  telemetry.Config `yaml:"telemetry" json:"telemetry"`
	KafkaConfig      []xkafka.Config  `yaml:"kafka" json:"kafka"`
}

func LoadDefaultConfig(v *DefaultConfig) (err error) {
	err = tools.LoadConfig(env.ConfigPath+"/default.yaml", v)
	bean.SetBeanOverwrite("default", v)
	return
}

func LoadYamlFile(fileName string, v interface{}) error {
	return tools.LoadConfig(env.ConfigPath+"/"+fileName, v)
}
