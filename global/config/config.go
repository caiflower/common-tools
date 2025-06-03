package config

import (
	"github.com/caiflower/common-tools/cluster"
	dbv1 "github.com/caiflower/common-tools/db/v1"
	"github.com/caiflower/common-tools/global/env"
	kafkav1 "github.com/caiflower/common-tools/kafka/v1"
	"github.com/caiflower/common-tools/pkg/bean"
	"github.com/caiflower/common-tools/pkg/http"
	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/caiflower/common-tools/pkg/tools"
	redisv1 "github.com/caiflower/common-tools/redis/v1"
	"github.com/caiflower/common-tools/telemetry"
	webv1 "github.com/caiflower/common-tools/web/v1"
)

type DefaultConfig struct {
	LoggerConfig     logger.Config    `yaml:"logger" json:"logger"`
	ClusterConfig    cluster.Config   `yaml:"cluster" json:"cluster"`
	DatabaseConfig   []dbv1.Config    `yaml:"database" json:"database"`
	HttpClientConfig http.Config      `yaml:"http_client" json:"http_client"`
	WebConfig        []webv1.Config   `yaml:"web" json:"web"`
	RedisConfig      []redisv1.Config `yaml:"redis" json:"redis"`
	KafkaConfig      []kafkav1.Config `yaml:"kafka" json:"kafkaConfig"`
	TelemetryConfig  telemetry.Config `yaml:"telemetry" json:"telemetry"`
}

func LoadDefaultConfig(v *DefaultConfig) (err error) {
	err = tools.LoadConfig(env.ConfigPath+"/default.yaml", v)
	bean.SetBeanOverwrite("default", v)
	return
}

func LoadYamlFile(fileName string, v interface{}) error {
	return tools.LoadConfig(env.ConfigPath+"/"+fileName, v)
}
