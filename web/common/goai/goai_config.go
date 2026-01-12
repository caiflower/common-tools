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

package goai

type Config struct {
	ReadContentTypes        []string
	WriteContentTypes       []string
	CommonRequest           any
	CommonRequestDataField  string
	CommonResponse          any
	CommonResponseDataField string
	IgnorePkgPath           bool
}

func (oai *OpenApiV3) fillWithDefaultValue() {
	if oai.OpenAPI == "" {
		oai.OpenAPI = `3.0.0`
	}
	if len(oai.Config.ReadContentTypes) == 0 {
		oai.Config.ReadContentTypes = defaultReadContentTypes
	}
	if len(oai.Config.WriteContentTypes) == 0 {
		oai.Config.WriteContentTypes = defaultWriteContentTypes
	}
}
