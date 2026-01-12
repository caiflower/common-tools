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

type Path struct {
	Ref         string     `json:"$ref,omitempty"`
	Summary     string     `json:"summary,omitempty"`
	Description string     `json:"description,omitempty"`
	Connect     *Operation `json:"connect,omitempty"`
	Delete      *Operation `json:"delete,omitempty"`
	Get         *Operation `json:"get,omitempty"`
	Head        *Operation `json:"head,omitempty"`
	Options     *Operation `json:"options,omitempty"`
	Patch       *Operation `json:"patch,omitempty"`
	Post        *Operation `json:"post,omitempty"`
	Put         *Operation `json:"put,omitempty"`
	Trace       *Operation `json:"trace,omitempty"`
	Servers     Servers    `json:"servers,omitempty"`
	Parameters  Parameters `json:"parameters,omitempty"`
	XExtensions XExtension `json:"-"`
}

type Paths map[string]Path

type Parameters []*ParameterRef
