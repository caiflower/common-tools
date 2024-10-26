package config

import (
	"github.com/caiflower/common-tools/cluster"
	"github.com/caiflower/common-tools/global/env"
	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/caiflower/common-tools/pkg/tools"
)

type DefaultConfig struct {
	LoggerConfig  logger.Config  `yaml:"logger"`
	ClusterConfig cluster.Config `yaml:"cluster"`
}

func LoadDefaultConfig(v *DefaultConfig) (err error) {
	err = tools.LoadConfig(env.ConfigPath+"/default.yaml", v)
	return
}
