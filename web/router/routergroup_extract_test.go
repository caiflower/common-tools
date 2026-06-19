/*
 * Copyright 2026 caiflower Authors
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

import "testing"

func TestExtractGRPCMethodName(t *testing.T) {
	tests := []struct {
		name     string
		funcName string
		want     string
	}{
		// Pattern 1: Protoc-generated handlers with underscores
		{
			name:     "protoc standard",
			funcName: "github.com/caiflower/common-tools/web/test/proto._IService_Search_Handler",
			want:     "Search",
		},
		{
			name:     "protoc multi-word method",
			funcName: "github.com/example/proto._TaskService_GetTodoTask_Handler",
			want:     "GetTodoTask",
		},
		{
			name:     "protoc short name",
			funcName: "_IService_Run_Handler",
			want:     "Run",
		},

		// Pattern 2: Wrapper functions (XxxServiceYyyHandler)
		{
			name:     "wrapper ExecutionServiceGetHandler",
			funcName: "github.com/caiflower/dagflow/backend/internal/proto.ExecutionServiceGetHandler",
			want:     "Get",
		},
		{
			name:     "wrapper ExecutionServiceRunHandler",
			funcName: "github.com/caiflower/dagflow/backend/internal/proto.ExecutionServiceRunHandler",
			want:     "Run",
		},
		{
			name:     "wrapper ExecutionServiceListHandler",
			funcName: "github.com/caiflower/dagflow/backend/internal/proto.ExecutionServiceListHandler",
			want:     "List",
		},
		{
			name:     "wrapper FlowServiceCreateHandler",
			funcName: "github.com/caiflower/dagflow/backend/internal/proto.FlowServiceCreateHandler",
			want:     "Create",
		},
		{
			name:     "wrapper FlowServiceValidateHandler",
			funcName: "github.com/caiflower/dagflow/backend/internal/proto.FlowServiceValidateHandler",
			want:     "Validate",
		},
		{
			name:     "wrapper ProtocolServiceListHandler",
			funcName: "github.com/caiflower/dagflow/backend/internal/proto.ProtocolServiceListHandler",
			want:     "List",
		},
		{
			name:     "wrapper short name",
			funcName: "FlowServiceGetHandler",
			want:     "Get",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractGRPCMethodName(tt.funcName)
			if got != tt.want {
				t.Errorf("extractGRPCMethodName(%q) = %q, want %q", tt.funcName, got, tt.want)
			}
		})
	}
}
