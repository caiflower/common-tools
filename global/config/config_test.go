package config

import (
	"fmt"
	"testing"
)

func TestLoadDefaultConfig(t *testing.T) {
	defaultConfig := DefaultConfig{}
	err := LoadDefaultConfig(&defaultConfig)
	if err != nil {
		panic(err)
	}

	fmt.Printf("%+v\n", defaultConfig)
}
