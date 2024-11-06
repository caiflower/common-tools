package tools

func ToJson(v interface{}) string {
	bytes, _ := Marshal(v)
	return string(bytes)
}
