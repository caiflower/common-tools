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

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"github.com/caiflower/common-tools/pkg/basic"
	"github.com/caiflower/common-tools/pkg/tools"
	"github.com/caiflower/common-tools/taskx/dao/model"
	"github.com/dominikbraun/graph"
)

type TaskRollbackState string

const (
	TaskPending        = "Pending"
	TaskRunning        = "Running"
	TaskSubtaskRunning = "SubtaskRunning"
	TaskFailed         = "Failed"
	TaskSucceeded      = "Succeeded"

	RollingBack       TaskRollbackState = "RollingBack"
	RollbackSucceeded TaskRollbackState = "RollbackSucceeded"
	RollbackFailed    TaskRollbackState = "RollbackFailed"
	RollbackPending   TaskRollbackState = "RollbackPending"
	NoneRollback      TaskRollbackState = "NoneRollback"

	DefaultRetryCount    = 3
	DefaultRetryInterval = 5
)

var taskHash = func(c *Subtask) string {
	return c.GetID()
}

type Task struct {
	task          model.Task
	failedSubtask bool
	subTasks      []*Subtask
	subtaskMap    map[string]*Subtask
	g             graph.Graph[string, *Subtask]
	rollbacks     []*Subtask
}

type Subtask struct {
	subtask model.Subtask
}

func NewTask(taskName string) *Task {
	return &Task{
		task: model.Task{
			ID:            tools.GenerateId("t"),
			TaskName:      taskName,
			State:         TaskPending,
			Retry:         DefaultRetryCount,
			RetryInterval: DefaultRetryInterval,
		},
		subtaskMap: make(map[string]*Subtask),
		g:          graph.New(taskHash, graph.Directed(), graph.PreventCycles()),
	}
}

func NewSubtask(taskName string) *Subtask {
	return &Subtask{
		subtask: model.Subtask{
			ID:            tools.GenerateId("st"),
			TaskName:      taskName,
			State:         TaskPending,
			Retry:         DefaultRetryCount,
			RetryInterval: DefaultRetryInterval,
			Rollback:      string(RollbackPending),
		},
	}
}

func (t *Subtask) GetID() string {
	return t.subtask.ID
}

func (t *Subtask) GetState() string {
	return t.subtask.State
}

func (t *Subtask) GetName() string {
	return t.subtask.TaskName
}

func (t *Subtask) UnmarshalInput(v interface{}) error {
	return tools.DeByte([]byte(t.subtask.Input), v)
}

func (t *Subtask) GetInput() string {
	return t.subtask.Input
}

func (t *Subtask) SetInput(content interface{}) *Subtask {
	_tmp, _ := tools.ToByte(content)
	t.subtask.Input = string(_tmp)
	return t
}

func (t *Subtask) GetOutput() string {
	return t.subtask.Output
}

func (t *Subtask) UnmarshalOutput(v interface{}) error {
	return tools.DeByte([]byte(t.subtask.Output), v)
}

func (t *Subtask) IsRollbackFinished() bool {
	rollback := t.GetRollback()
	return TaskRollbackState(rollback) == RollbackFailed || TaskRollbackState(rollback) == RollbackSucceeded || TaskRollbackState(rollback) == NoneRollback
}

func (t *Subtask) IsFinished() bool {
	return t.subtask.State == TaskFailed || t.subtask.State == TaskSucceeded
}

func (t *Subtask) SetRetry(retry int8) *Subtask {
	t.subtask.Retry = retry
	return t
}

func (t *Subtask) SetRetryInterval(retryInterval int32) *Subtask {
	t.subtask.RetryInterval = retryInterval
	return t
}

func (t *Subtask) needRollback() bool {
	return (t.subtask.State == TaskRunning ||
		t.subtask.State == TaskSucceeded ||
		t.subtask.State == TaskFailed) &&
		(t.subtask.Rollback == string(RollbackPending) || t.subtask.Rollback == string(RollingBack))
}

func (t *Subtask) GetRollback() string {
	return t.subtask.Rollback
}

func (t *Subtask) GetLastRunTime() *basic.Time {
	return &t.subtask.LastRunTime
}

func (t *Subtask) GetRetryInterval() int32 {
	return t.subtask.RetryInterval
}

func (t *Subtask) getModel() *model.Subtask {
	return &t.subtask
}

func (t *Task) GetID() string {
	return t.task.ID
}

func (t *Task) GetTaskName() string {
	return t.task.TaskName
}

func (t *Task) GetInput() string {
	return t.task.Input
}

func (t *Task) SetInput(content interface{}) *Task {
	_tmp, _ := tools.ToByte(content)
	t.task.Input = string(_tmp)
	return t
}

func (t *Task) GetState() string {
	return t.task.State
}

func (t *Task) SetDescription(description string) *Task {
	t.task.Description = description
	return t
}

func (t *Task) GetDescription() string {
	return t.task.Description
}

func (t *Task) SetExecuteTime(executeTime time.Time) *Task {
	t.task.ExecuteTime = basic.Time(executeTime)
	return t
}

func (t *Task) GetExecuteTime() time.Time {
	return t.task.ExecuteTime.Time()
}

func (t *Task) UnmarshalInput(v interface{}) error {
	return tools.DeByte([]byte(t.task.Input), v)
}

func (t *Task) AddSubtask(task *Subtask) error {
	t.subTasks = append(t.subTasks, task)
	t.subtaskMap[task.GetID()] = task
	return t.g.AddVertex(task)
}

func (t *Task) AddDirectedEdge(src, dst *Subtask) error {
	//t.isSort = false
	return t.g.AddEdge(src.GetID(), dst.GetID())
}

func (t *Task) SetRequestID(requestID string) *Task {
	t.task.RequestID = requestID
	return t
}

func (t *Task) SetRetry(retry int8) *Task {
	t.task.Retry = retry
	return t
}

func (t *Task) SetRetryInterval(retryInterval int32) *Task {
	t.task.RetryInterval = retryInterval
	return t
}

// SetUrgent handle task immediately
func (t *Task) SetUrgent() *Task {
	t.task.Urgent = true
	return t
}

func (t *Task) NextSubTasks() ([]*Subtask, bool) {
	var res []*Subtask
	// has failedSubtask
	if t.failedSubtask {
		return t.rollbacks, true
	}

	predecessorMap, _ := t.g.PredecessorMap()
	for k, v := range predecessorMap {
		if len(v) == 0 {
			res = append(res, t.subtaskMap[k])
		}
	}

	return res, false
}

func (t *Task) updateSubtaskState(taskId string, taskState string) error {
	subtask := t.subtaskMap[taskId]
	if subtask == nil {
		return fmt.Errorf("%s Task not found", taskId)
	}

	if taskState == TaskSucceeded {
		adjacencyMap, err := t.g.AdjacencyMap()
		if err != nil {
			return err
		}
		for _, v := range adjacencyMap[subtask.GetID()] {
			if err = t.g.RemoveEdge(v.Source, v.Target); err != nil {
				return err
			}
		}

		predecessorMap, err := t.g.PredecessorMap()
		if err != nil {
			return err
		}
		for _, v := range predecessorMap[subtask.GetID()] {
			if err = t.g.RemoveEdge(v.Source, v.Target); err != nil {
				return err
			}
		}

		if err = t.g.RemoveVertex(subtask.GetID()); err != nil {
			return err
		}
	} else if taskState == TaskFailed {
		t.failedSubtask = true
	}

	subtask.subtask.State = taskState

	return nil
}

func (t *Task) IsFinished() bool {
	return t.task.State == TaskFailed || t.task.State == TaskSucceeded
}

func (t *Task) Size() int {
	size, _ := t.g.Size()
	return size
}

func (t *Task) Order() int {
	order, _ := t.g.Order()
	return order
}

func (t *Task) Graph() string {
	var buf bytes.Buffer
	adjacencyMap, _ := t.g.AdjacencyMap()
	sorts, _ := graph.TopologicalSort(t.g)
	for _, v := range sorts {
		buf.WriteString(fmt.Sprintf("[%s]", t.subtaskMap[v].subtask.TaskName))
		var edgeStr string
		for _, edge := range adjacencyMap[v] {
			if edgeStr == "" {
				edgeStr += " => ["
			} else {
				edgeStr += ","
			}
			edgeStr += t.subtaskMap[edge.Target].subtask.TaskName
		}
		if edgeStr != "" {
			edgeStr += "]"
		} else {
			edgeStr = " => []"
		}

		buf.WriteString(edgeStr + "\n")
	}
	return buf.String()
}

func (t *Task) convert2Bean() (*model.Task, []model.Subtask) {
	now := time.Now()
	task := &t.task
	task.CreateTime = basic.Time(now)
	task.Status = 1

	predecessorMap, _ := t.g.PredecessorMap()
	subtasks := make([]model.Subtask, 0, len(t.subTasks))
	for _, v := range t.subTasks {
		m := predecessorMap[v.GetID()]
		preSubtaskId := ""
		for k := range m {
			if preSubtaskId != "" {
				preSubtaskId += ","
			}
			preSubtaskId += k
		}
		subtask := v.subtask
		subtask.TaskID = t.GetID()
		subtask.PreSubtaskID = preSubtaskId
		subtask.Status = 1
		subtasks = append(subtasks, subtask)
	}

	return task, subtasks
}

func (t *Task) initByBean(task *model.Task, subtasks []model.Subtask) (*Task, error) {
	t.task = *task
	t.subtaskMap = make(map[string]*Subtask)
	t.g = graph.New(taskHash, graph.Directed(), graph.PreventCycles())
	for _, subtask := range subtasks {
		st := &Subtask{
			subtask: subtask,
		}
		err := t.AddSubtask(st)
		if err != nil {
			return nil, err
		}
	}

	for _, subtask := range subtasks {
		if subtask.PreSubtaskID == "" {
			continue
		}
		for _, v := range strings.Split(subtask.PreSubtaskID, ",") {
			subTask := t.subtaskMap[v]
			subTask2 := t.subtaskMap[subtask.ID]
			err := t.AddDirectedEdge(subTask, subTask2)
			if err != nil {
				return nil, err
			}
		}
		if subtask.State == TaskFailed {
			t.failedSubtask = true
		}
	}

	if t.failedSubtask {
		if err := t.rollbackSubtasks(); err != nil {
			return nil, err
		}
	} else {
		for _, subtask := range subtasks {
			if err := t.updateSubtaskState(subtask.ID, subtask.State); err != nil {
				return nil, err
			}
		}
	}

	return t, nil
}

func (t *Task) rollbackSubtasks() error {
	var res [][]*Subtask
	predecessorMap, err := t.g.PredecessorMap()
	if err != nil {
		return err
	}

	for len(predecessorMap) != 0 {
		var tmp, removeTmp []*Subtask

		for k, v := range predecessorMap {
			if len(v) == 0 {
				if t.subtaskMap[k].needRollback() {
					tmp = append(tmp, t.subtaskMap[k])
				}

				removeTmp = append(removeTmp, t.subtaskMap[k])
			}
		}

		adjacencyMap, err1 := t.g.AdjacencyMap()
		if err1 != nil {
			return err1
		}
		for _, v := range removeTmp {
			for _, v1 := range adjacencyMap[v.GetID()] {
				if err = t.g.RemoveEdge(v1.Source, v1.Target); err != nil {
					return err
				}
			}

			if err = t.g.RemoveVertex(v.GetID()); err != nil {
				return err
			}
		}

		if len(tmp) > 0 {
			res = append(res, tmp)
		}

		predecessorMap, err = t.g.PredecessorMap()
		if err != nil {
			return err
		}
	}

	// rollback subtask
	if len(res) > 0 {
		t.rollbacks = res[len(res)-1]
	}

	return nil
}
