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

import "encoding/base64"

var Base64Encoding = ToBase64

func ToBase64(str string) string {
	return base64.StdEncoding.EncodeToString([]byte(str))
}

func Base64Decoding(str string) (string, error) {
	if _tmp, err := base64.StdEncoding.DecodeString(str); err != nil {
		return "", err
	} else {
		return string(_tmp), nil
	}
}
