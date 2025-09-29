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
	"github.com/uptrace/bun"
)

type SubtaskBak Subtask

type Subtask struct {
	Id            int
	TaskId        string
	SubtaskId     string
	TaskName      string
	Input         string
	Output        string
	TaskState     string
	Worker        string
	Retry         int
	RetryInterval int
	PreSubtaskId  string
	Rollback      string
	UpdateTime    basic.Time
	Status        int
}

func (t *Subtask) String() string {
	return t.SubtaskId
}

func (t *Subtask) IsFinished() bool {
	return t.TaskState == "Succeeded" || t.TaskState == "Failed"
}

func (t *Subtask) RollbackFinished() bool {
	return t.Rollback == "RollbackFailed" || t.Rollback == "RollbackSucceeded"
}

type SubtaskDao struct {
	dbv1.IDB `autowired:""`
}

func (d *SubtaskDao) GetSubtasksByTaskId(taskId string) ([]*Subtask, map[string]*Subtask, error) {
	res := make([]*Subtask, 0)
	err := d.GetSelect(&res).Where("task_id = ?", taskId).Scan(context.TODO(), &res)
	if err != nil {
		return nil, nil, err
	}
	subtaskMap := make(map[string]*Subtask)
	for _, v := range res {
		subtaskMap[v.SubtaskId] = v
	}
	return res, subtaskMap, nil
}

func (d *SubtaskDao) SetOutputAndTaskState(subtaskId, output, taskState string, tx *bun.Tx) (int64, error) {
	update := d.GetUpdate(&Subtask{}, tx).Where("subtask_id = ?", subtaskId).Set("task_state = ?", taskState).Set("output = ?", output)
	return d.GetRowsAffected(update.Exec(context.TODO()))
}

func (d *SubtaskDao) SetWorkerAndTaskState(subtaskId string, worker string, taskState string, tx *bun.Tx) error {
	update := d.GetUpdate(&Subtask{}, tx).Where("subtask_id = ?", subtaskId).Set("worker = ?", worker).Set("task_state = ?", taskState)
	_, err := d.GetRowsAffected(update.Exec(context.TODO()))
	return err
}

func (d *SubtaskDao) SetWorkerAndRollback(subtaskId, worker, rollback string, tx *bun.Tx) error {
	update := d.GetUpdate(&Subtask{}, tx).Where("subtask_id = ?", subtaskId).Set("worker = ?", worker).Set("rollback = ?", rollback)
	_, err := d.GetRowsAffected(update.Exec(context.TODO()))
	return err
}

func (d *SubtaskDao) GetSubtasksBySubtaskIds(subtaskIds []string) ([]*Subtask, error) {
	var subTasks []*Subtask
	err := d.GetSelect(&subTasks).Where("subtask_id IN (?)", bun.In(subtaskIds)).Scan(context.TODO(), &subTasks)
	return subTasks, err
}

func (d *SubtaskDao) SetRetry(subtaskId string, retry int, tx *bun.Tx) (err error) {
	_, err = d.GetRowsAffected(d.GetUpdate(&Subtask{}, tx).Where("subtask_id = ?", subtaskId).Set("retry = ?", retry).Exec(context.TODO()))
	return
}

func (d *SubtaskDao) SetRollbackAndTaskState(subtaskId, output, rollback string, tx *bun.Tx) (int64, error) {
	update := d.GetUpdate(&Subtask{}, tx).Where("subtask_id = ?", subtaskId).Set("rollback = ?", rollback).Set("output = ?", output)
	return d.GetRowsAffected(update.Exec(context.TODO()))
}
