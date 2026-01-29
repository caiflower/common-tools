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
	"context"

	dbv1 "github.com/caiflower/common-tools/db/v1"
	"github.com/caiflower/common-tools/pkg/logger"
	taskmodel "github.com/caiflower/common-tools/taskx/dao/model"
	"github.com/uptrace/bun"
)

func (t *taskDispatcher) backupTask() {
	var tasks []taskmodel.TaskBak
	var subtasks []taskmodel.SubtaskBak
	tx := dbv1.NewBatchTx(t.DBClient.GetDB())

	if err := t.DBClient.GetDB().NewSelect().Table("task").
		Where("state IN (?) AND create_time <= DATE_SUB(NOW(), interval ? second)", bun.In([]string{TaskFailed, TaskSucceeded}), t.cfg.BackupTaskAge.Seconds()).
		Order("id").Limit(100).
		Scan(context.TODO(), &tasks); err != nil {
		logger.Error("query task failed. err: %v", err)
	}

	taskIds := make([]string, 0)
	for _, task := range tasks {
		taskIds = append(taskIds, task.ID)
	}
	if len(taskIds) == 0 {
		return
	}

	tx.Add(func(tx *bun.Tx) error {
		_, err := tx.NewInsert().Model(&tasks).Exec(context.Background())
		return err
	})

	tx.Add(func(tx *bun.Tx) error {
		//_, err := tx.NewDelete().Table("task").Where("id IN (?)", bun.In(taskPrimaryKey)).Exec(context.Background())
		//return err
		return nil
	})

	tx.Add(func(tx *bun.Tx) error {
		return tx.NewSelect().Table("subtask").
			Where("task_id IN (?)", bun.In(taskIds)).
			Order("id").Limit(100).
			Scan(context.TODO(), &subtasks)
	})

	tx.Add(func(tx *bun.Tx) error {
		_, err := tx.NewInsert().Model(&subtasks).Exec(context.Background())
		return err
	})

	tx.Add(func(tx *bun.Tx) error {
		_, err := tx.NewDelete().Table("subtask").Where("task_id IN (?)", bun.In(taskIds)).Exec(context.Background())
		return err
	})

	if err := tx.Submit(); err != nil {
		logger.Error("backupTask tx submit failed. err: %v", err)
	}
}
