//go:build (amd64 || arm64) && !stdjson

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

import "github.com/bytedance/sonic"

// Name is the name of the effective json package.
const Name = "sonic"

var (
	json = sonic.ConfigStd
	// Marshal is sonic implementation exported by hertz which is used by rendering.
	Marshal = json.Marshal
	// Unmarshal is sonic implementation exported by hertz which is used by binding.
	Unmarshal = json.Unmarshal
	// MarshalIndent is sonic implementation exported by hertz.
	MarshalIndent = json.MarshalIndent
	// NewDecoder is sonic implementation exported by hertz.
	NewDecoder = json.NewDecoder
	// NewEncoder is sonic implementation exported by hertz.
	NewEncoder = json.NewEncoder
)

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
