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

 package taskxdao

import (
	"context"

	dbv1 "github.com/caiflower/common-tools/db/v1"
	"github.com/caiflower/common-tools/pkg/basic"
	"github.com/caiflower/common-tools/pkg/tools"
	"github.com/uptrace/bun"
)

type TaskBak Task

type Task struct {
	Id            int
	RequestId     string
	TaskId        string
	TaskName      string
	Input         string
	Output        string
	Worker        string
	Retry         int
	RetryInterval int
	Urgent        bool
	TaskState     string
	Description   string
	CreateTime    basic.Time
	UpdateTime    basic.Time
	Status        int
}

type Output struct {
	Output         string `json:",omitempty"`
	Err            string `json:"err,omitempty"`
	Msg            string `json:"msg,omitempty"`
	RollbackErr    string `json:"rollbackErr,omitempty"`
	RollbackMsg    string `json:"rollbackMsg,omitempty"`
	RollbackOutput string `json:"rollbackOutput,omitempty"`
}

func (o Output) String() string {
	return tools.ToJson(o)
}

func (t *Task) String() string {
	return t.TaskId
}

func (t *Task) IsFinished() bool {
	return t.TaskState == "Succeeded" || t.TaskState == "Failed"
}

type TaskDao struct {
	dbv1.IDB `autowired:""`
}

func (d *TaskDao) GetByTaskState(taskState []string, id int) ([]*Task, error) {
	res := make([]*Task, 0)
	err := d.GetSelect(&res).Where("task_state IN (?)", bun.In(taskState)).Where("id >= ?", id).Order("id asc").Scan(context.TODO(), &res)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (d *TaskDao) SetTaskState(taskId, taskState string, tx *bun.Tx) (int64, error) {
	update := d.GetUpdate(&Task{}, tx).Where("task_id = ?", taskId).Set("task_state = ?", taskState)
	return d.GetRowsAffected(update.Exec(context.TODO()))
}

func (d *TaskDao) SetWorkerAndTaskState(taskId string, worker string, taskState string, tx *bun.Tx) error {
	update := d.GetUpdate(&Task{}, tx).Where("task_id = ?", taskId).Set("worker = ?", worker).Set("task_state = ?", taskState)
	_, err := d.GetRowsAffected(update.Exec(context.TODO()))
	return err
}

func (d *TaskDao) GetTasksByTaskIds(taskIds []string) ([]*Task, error) {
	var res []*Task
	err := d.GetSelect(&res).Where("task_id IN (?)", bun.In(taskIds)).Scan(context.TODO(), &res)
	return res, d.ParseErr(err)
}

func (d *TaskDao) SetOutputAndTaskState(taskId, output, taskState string, tx *bun.Tx) error {
	update := d.GetUpdate(&Task{}, tx).Where("task_id = (?)", taskId).Set("output = ?", output).Set("task_state = ?", taskState)
	_, err := d.GetRowsAffected(update.Exec(context.TODO()))
	return err
}

func (d *TaskDao) SetRetry(taskId string, retry int, tx *bun.Tx) (err error) {
	_, err = d.GetRowsAffected(d.GetUpdate(&Task{}, tx).Where("task_id = ?", taskId).Set("retry = ?", retry).Exec(context.TODO()))
	return
}
