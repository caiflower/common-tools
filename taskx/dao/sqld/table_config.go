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

package sqld

// TableConfig configures the physical table names for taskx DAO models.
type TableConfig struct {
	Task       string `yaml:"task" json:"task" default:"task"`
	Subtask    string `yaml:"subtask" json:"subtask" default:"subtask"`
	TaskBak    string `yaml:"taskBak" json:"taskBak" default:"task_bak"`
	SubtaskBak string `yaml:"subtaskBak" json:"subtaskBak" default:"subtask_bak"`
	TaskEdge   string `yaml:"taskEdge" json:"taskEdge" default:"task_edge"`
}

// DefaultTableConfig returns a TableConfig with default table names.
func DefaultTableConfig() *TableConfig {
	return &TableConfig{
		Task:       "task",
		Subtask:    "subtask",
		TaskBak:    "task_bak",
		SubtaskBak: "subtask_bak",
		TaskEdge:   "task_edge",
	}
}

// Normalize returns a copy of cfg with empty fields filled from defaults.
func (c *TableConfig) Normalize() *TableConfig {
	if c == nil {
		return DefaultTableConfig()
	}
	cp := *c
	if cp.Task == "" {
		cp.Task = "task"
	}
	if cp.Subtask == "" {
		cp.Subtask = "subtask"
	}
	if cp.TaskBak == "" {
		cp.TaskBak = "task_bak"
	}
	if cp.SubtaskBak == "" {
		cp.SubtaskBak = "subtask_bak"
	}
	if cp.TaskEdge == "" {
		cp.TaskEdge = "task_edge"
	}
	return &cp
}
