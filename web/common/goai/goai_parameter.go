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

import "encoding/json"

type Parameter struct {
	Name            string     `json:"name,omitempty"`
	In              string     `json:"in,omitempty"`
	Description     string     `json:"description,omitempty"`
	Style           string     `json:"style,omitempty"`
	Explode         *bool      `json:"explode,omitempty"`
	AllowEmptyValue bool       `json:"allowEmptyValue,omitempty"`
	AllowReserved   bool       `json:"allowReserved,omitempty"`
	Deprecated      bool       `json:"deprecated,omitempty"`
	Required        bool       `json:"required,omitempty"`
	Schema          *SchemaRef `json:"schema,omitempty"`
	Example         any        `json:"example,omitempty"`
	Examples        *Examples  `json:"examples,omitempty"`
	Content         *Content   `json:"content,omitempty"`
	XExtensions     XExtension `json:"-"`
}

type ParameterRef struct {
	Ref   string     `json:"$ref,omitempty"`
	Value *Parameter `json:",omitempty"`
}

func (r ParameterRef) MarshalJSON() ([]byte, error) {
	if r.Ref != "" {
		return formatRefToBytes(r.Ref), nil
	}
	return json.Marshal(r.Value)
}
