/*
 * Copyright 2024 caiflower Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

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

// ToCamelByte converts snake_case to camelCase with high performance using bytes
func ToCamelByte(src []byte) []byte {
	l := len(src)
	if l == 0 {
		return []byte{}
	}

	dst := make([]byte, 0, l)
	upperNext := false

	disableNormal := true
	if len(src) <= 15 { // if src len less than 15, then try to disableNormal
		for _, c := range src {
			if c == '_' {
				disableNormal = false
				break
			}
		}
	} else {
		disableNormal = false
	}

	if !disableNormal {
		for _, c := range src {
			if c == '_' {
				upperNext = true
				continue
			}

			if upperNext {
				if c >= 'a' && c <= 'z' {
					c -= 32 // 'a' - 'A'
				}
				upperNext = false
			}

			dst = append(dst, c)
		}
	} else {
		dst = src
	}

	if len(dst) > 0 {
		c := dst[0]
		if c >= 'A' && c <= 'Z' {
			dst[0] = c + 32 // 'a' - 'A'
		}
	}

	return dst
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
