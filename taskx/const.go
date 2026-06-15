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

package taskx

// TaskState 任务状态
type TaskState string

const (
	TaskPending        TaskState = "pending"
	TaskRunning        TaskState = "running"
	TaskSucceeded      TaskState = "succeeded"
	TaskFailed         TaskState = "failed"
	TaskSubtaskRunning TaskState = "subtask_running"
	TaskSkipped        TaskState = "skipped"
)

// TaskRollbackState 任务回滚状态
type TaskRollbackState string

const (
	RollbackPending   TaskRollbackState = "rollback_pending"
	RollingBack       TaskRollbackState = "rolling_back"
	RollbackSucceeded TaskRollbackState = "rollback_succeeded"
	RollbackFailed    TaskRollbackState = "rollback_failed"
	NoneRollback      TaskRollbackState = "none_rollback"
)

// TaskAffinityType 任务亲和性类型
type TaskAffinityType string

const (
	AffinityRandom         TaskAffinityType = "random"
	AffinityForceSameNode  TaskAffinityType = "force_same_node"
	AffinityPreferSameNode TaskAffinityType = "prefer_same_node"
)

// DefaultRetryCount 默认重试次数
const DefaultRetryCount int8 = 3

// DefaultRetryInterval 默认重试间隔（秒）
const DefaultRetryInterval int32 = 0

// TriggerMode 触发模式（DB 持久化用）
const (
	TriggerModeAllPredecessor = "all_predecessor"
	TriggerModeAnyPredecessor = "any_predecessor"
)

// EdgeType 边类型（DB 持久化用）
const (
	EdgeTypeControl        = "control"
	EdgeTypeData           = "data"
	EdgeTypeControlAndData = "control+data"
)

// RollbackStrategy 回滚策略（DB 持久化用）
const (
	RollbackStrategyAll    = "rollback_all"
	RollbackStrategyFailed = "rollback_failed"
	RollbackStrategyCustom = "rollback_custom"
)
