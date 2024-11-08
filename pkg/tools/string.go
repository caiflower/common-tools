package tools

import (
	"regexp"
	"strings"

	"github.com/google/uuid"
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

func RegFind(s, reg string) []string {
	pattern := regexp.MustCompile(reg)
	return pattern.FindAllString(s, -1)
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

func UUID() string {
	u, _ := uuid.NewUUID()
	return u.String()
}

func ToCamel(str string) (camel string) {
	parts := strings.Split(str, "_")
	for _, v := range parts {
		if v != "" {
			if len(v) > 1 {
				camel += strings.ToUpper(v[:1]) + v[1:]
			} else {
				camel += strings.ToTitle(v)
			}
		}
	}

	if len(camel) > 1 {
		camel = strings.ToLower(camel[:1]) + camel[1:]
	} else {
		camel = strings.ToLower(camel)
	}

	return
}
