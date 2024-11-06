package config

import (
	"github.com/caiflower/common-tools/cluster"
	dbv1 "github.com/caiflower/common-tools/db/v1"
	"github.com/caiflower/common-tools/global/env"
	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/caiflower/common-tools/pkg/tools"
)

type DefaultConfig struct {
	LoggerConfig   logger.Config  `yaml:"logger"`
	ClusterConfig  cluster.Config `yaml:"cluster"`
	DatabaseConfig dbv1.Config    `yaml:"database"`
}

func LoadDefaultConfig(v *DefaultConfig) (err error) {
	err = tools.LoadConfig(env.ConfigPath+"/default.yaml", v)
	return
}
