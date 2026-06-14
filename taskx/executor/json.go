package executor

import "github.com/caiflower/common-tools/web/common/json"

// unmarshalJSON JSON 反序列化辅助函数
func unmarshalJSON(data []byte, target any) error {
	return json.Unmarshal(data, target)
}

// marshalJSON JSON 序列化辅助函数
func marshalJSON(v any) ([]byte, error) {
	return json.Marshal(v)
}
