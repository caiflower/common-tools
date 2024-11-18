package tools

func LoadConfig(filename string, v interface{}) error {
	err := UnmarshalFileYaml(filename, v)
	if err != nil {
		return err
	}

	return nil
}
