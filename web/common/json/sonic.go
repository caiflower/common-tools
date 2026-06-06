// Copyright 2022 CloudWeGo Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

package json

import (
	"github.com/caiflower/common-tools/pkg/json"
)

// Name is the name of the effective json package.
const Name = "sonic"

var (
	// Marshal delegates to tools.Marshal (sonic on amd64/arm64, encoding/json otherwise).
	Marshal = json.Marshal
	// Unmarshal delegates to tools.Unmarshal (sonic on amd64/arm64, encoding/json otherwise).
	Unmarshal = json.Unmarshal
	// MarshalIndent delegates to tools.MarshalIndent.
	MarshalIndent = json.MarshalIndent
	// NewDecoder delegates to tools.NewDecoder.
	NewDecoder = json.NewDecoder
	// NewEncoder delegates to tools.NewEncoder.
	NewEncoder = json.NewEncoder
)
