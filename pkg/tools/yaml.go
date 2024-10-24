package tools

import (
	"os"

	"gopkg.in/yaml.v2"
)

func UnmarshalFileYaml(filename string, v interface{}) error {
	content, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	return yaml.Unmarshal(content, v)
}
