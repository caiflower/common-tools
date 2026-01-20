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

package router

import (
	"strings"
)

// normalizeRestfulPath 将 /{param} 形式转换为 /:param，兼容原有语法
func normalizeRestfulPath(path string) string {
	if !strings.Contains(path, "{") {
		return path
	}
	var b strings.Builder
	b.Grow(len(path))
	for i := 0; i < len(path); i++ {
		c := path[i]
		if c == '{' {
			j := strings.IndexByte(path[i:], '}')
			if j == -1 || j == 1 { // 未闭合或空参数名，保持原样
				b.WriteByte(c)
				continue
			}
			name := path[i+1 : i+j]
			b.WriteByte(':')
			b.WriteString(name)
			i += j
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
}

// toSwaggerPath 将 /:id 转为 /{id} 以符合 swagger 规范
func toSwaggerPath(path string) string {
	if !strings.ContainsAny(path, ":") {
		return path
	}
	var b strings.Builder
	b.Grow(len(path) + 4)
	for i := 0; i < len(path); i++ {
		c := path[i]
		if c == ':' {
			b.WriteByte('{')
			j := i + 1
			for ; j < len(path) && path[j] != '/'; j++ {
			}
			b.WriteString(path[i+1 : j])
			b.WriteByte('}')
			i = j - 1
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
}
