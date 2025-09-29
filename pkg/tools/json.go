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
