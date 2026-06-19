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

package json

import (
	"encoding/json"

	"github.com/caiflower/common-tools/pkg/tools"
)

// Name is the name of the effective json package.
var (
	// Marshal delegates to tools.Marshal (sonic on amd64/arm64, encoding/json otherwise).
	Marshal = tools.Marshal
	// Unmarshal delegates to tools.Unmarshal (sonic on amd64/arm64, encoding/json otherwise).
	Unmarshal = tools.Unmarshal
	// MarshalIndent delegates to tools.MarshalIndent.
	MarshalIndent = tools.MarshalIndent
	// NewDecoder delegates to tools.NewDecoder.
	NewDecoder = tools.NewDecoder
	// NewEncoder delegates to tools.NewEncoder.
	NewEncoder = tools.NewEncoder
	// Valid delegates to tools.Valid.
	Valid = tools.Valid
)

type RawMessage = json.RawMessage
