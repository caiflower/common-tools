package config

import (
	"github.com/caiflower/common-tools/cluster"
	dbv1 "github.com/caiflower/common-tools/db/v1"
	"github.com/caiflower/common-tools/global/env"
	"github.com/caiflower/common-tools/pkg/http"
	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/caiflower/common-tools/pkg/tools"
	redisv1 "github.com/caiflower/common-tools/redis/v1"
	webv1 "github.com/caiflower/common-tools/web/v1"
)

type DefaultConfig struct {
	LoggerConfig     logger.Config    `yaml:"logger"`
	ClusterConfig    cluster.Config   `yaml:"cluster"`
	DatabaseConfig   []dbv1.Config    `yaml:"database"`
	HttpClientConfig http.Config      `yaml:"http_client"`
	WebConfig        []webv1.Config   `yaml:"web"`
	RedisConfig      []redisv1.Config `yaml:"redis"`
}

func LoadDefaultConfig(v *DefaultConfig) (err error) {
	err = tools.LoadConfig(env.ConfigPath+"/default.yaml", v)
	return
}

func LoadYamlFile(fileName string, v interface{}) error {
	return tools.LoadConfig(env.ConfigPath+"/"+fileName, v)
}
