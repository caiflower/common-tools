package global

import (
	"github.com/caiflower/common-tools/pkg/global/env"
	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/caiflower/common-tools/pkg/tools"
)

type DefaultConfig struct {
	LoggerConfig logger.Config `yaml:"logger"`
}

func LoadDefaultConfig(v *DefaultConfig) (err error) {
	err = tools.LoadConfig(env.ConfigPath+"/default.yaml", v)
	return
}
