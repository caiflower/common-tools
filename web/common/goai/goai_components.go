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

type Components struct {
	Schemas         Schemas         `json:"schemas,omitempty"`
	Parameters      ParametersMap   `json:"parameters,omitempty"`
	Headers         Headers         `json:"headers,omitempty"`
	RequestBodies   RequestBodies   `json:"requestBodies,omitempty"`
	Responses       Responses       `json:"responses,omitempty"`
	SecuritySchemes SecuritySchemes `json:"securitySchemes,omitempty"`
	Examples        Examples        `json:"examples,omitempty"`
	Links           Links           `json:"links,omitempty"`
	Callbacks       Callbacks       `json:"callbacks,omitempty"`
}

type ParametersMap map[string]*ParameterRef

type RequestBodies map[string]*RequestBodyRef
