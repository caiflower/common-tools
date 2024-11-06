package tools

import (
	"regexp"
	"strings"

	jsoniter "github.com/json-iterator/go"
)

// RegReplace 正则表达式替换
func RegReplace(str string, reg string, newStr string) string {
	pattern, err := regexp.Compile(reg)
	if err != nil {
		return str
	}
	return pattern.ReplaceAllString(str, newStr)
}

func MatchReg(s, reg string) bool {
	pattern := regexp.MustCompile(reg)
	return pattern.Match([]byte(s))
}

func StringSliceContains(slice []string, str string) bool {
	for _, v := range slice {
		if v == str {
			return true
		}
	}
	return false
}

func StringSliceLike(slice []string, str string) bool {
	for _, v := range slice {
		if strings.Contains(v, str) {
			return true
		}
	}
	return false
}

func Marshal(v interface{}) (bytes []byte, err error) {
	switch t := v.(type) {
	case string:
		bytes = []byte(t)
		return
	case []byte:
		bytes = v.([]byte)
		return
	}

	bytes, err = jsoniter.ConfigFastest.Marshal(v)

	return
}

func Unmarshal(bytes []byte, v interface{}) (err error) {
	switch v.(type) {
	case string:
		v = string(bytes)
		return
	case []byte:
		v = bytes
		return
	}
	return jsoniter.ConfigFastest.Unmarshal(bytes, v)
}
