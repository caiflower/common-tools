package tools

import jsoniter "github.com/json-iterator/go"

func ToJson(v interface{}) string {
	bytes, _ := Marshal(v)
	return string(bytes)
}

func ToByte(v interface{}) (bytes []byte, err error) {
	switch t := v.(type) {
	case string:
		bytes = []byte(t)
		return
	case []byte:
		bytes = v.([]byte)
		return
	}

	return Marshal(v)
}

func DeByte(bytes []byte, v interface{}) (err error) {
	switch v.(type) {
	case *string:
		s := v.(*string)
		*s = string(bytes)
		return
	case []byte:
		v = bytes
		return
	}
	return Unmarshal(bytes, v)
}

func Marshal(v interface{}) (bytes []byte, err error) {
	bytes, err = jsoniter.ConfigFastest.Marshal(v)
	return
}

func Unmarshal(bytes []byte, v interface{}) (err error) {
	return jsoniter.ConfigFastest.Unmarshal(bytes, v)
}
