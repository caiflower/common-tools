package tools

import (
	"fmt"
	"reflect"
	"regexp"
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

func ToString(v interface{}) string {
	if v == nil {
		return ""
	}
	if t := reflect.ValueOf(v); t.Kind() == reflect.Ptr && t.IsNil() {
		return ""
	}
	return fmt.Sprint(v)
}
