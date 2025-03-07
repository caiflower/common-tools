package tools

import (
	"regexp"
	"strconv"
	"strings"
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

func ToString(value interface{}) string {
	var key string
	if value == nil {
		return key
	}

	switch value.(type) {
	case float64:
		ft := value.(float64)
		key = strconv.FormatFloat(ft, 'f', -1, 64)
	case float32:
		ft := value.(float32)
		key = strconv.FormatFloat(float64(ft), 'f', -1, 64)
	case int:
		it := value.(int)
		key = strconv.Itoa(it)
	case uint:
		it := value.(uint)
		key = strconv.Itoa(int(it))
	case int8:
		it := value.(int8)
		key = strconv.Itoa(int(it))
	case uint8:
		it := value.(uint8)
		key = strconv.Itoa(int(it))
	case int16:
		it := value.(int16)
		key = strconv.Itoa(int(it))
	case uint16:
		it := value.(uint16)
		key = strconv.Itoa(int(it))
	case int32:
		it := value.(int32)
		key = strconv.Itoa(int(it))
	case uint32:
		it := value.(uint32)
		key = strconv.Itoa(int(it))
	case int64:
		it := value.(int64)
		key = strconv.FormatInt(it, 10)
	case uint64:
		it := value.(uint64)
		key = strconv.FormatUint(it, 10)
	case string:
		key = value.(string)
	case []byte:
		key = string(value.([]byte))
	default:
		newValue, _ := Marshal(value)
		key = string(newValue)
	}

	return key
}
