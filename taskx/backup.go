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
	var tasks []taskmodel.Task
	var subtasks []taskmodel.SubtaskBak
	tx := dbv1.NewBatchTx(t.DBClient.GetDB())

	// 查询需要备份的任务
	if err := t.DBClient.GetDB().NewSelect().Table("task").
		Where("state IN (?) AND create_time <= DATE_SUB(NOW(), interval ? second)", bun.In([]string{string(TaskFailed), string(TaskSucceeded)}), t.cfg.BackupTaskAge.Seconds()).
		Order("id").Limit(100).
		Scan(context.TODO(), &tasks); err != nil {
		logger.Error("[backupTask] query tasks for backup failed. err: %v", err)
		return
	}

	taskIds := make([]string, 0)
	for _, task := range tasks {
		taskIds = append(taskIds, task.ID)
	}
	if len(taskIds) == 0 {
		return
	}

	// 转换任务模型为备份模型
	taskBaks := make([]taskmodel.TaskBak, len(tasks))
	for i, task := range tasks {
		taskBaks[i] = taskmodel.TaskBak{
			ID:               task.ID,
			RequestID:        task.RequestID,
			TaskName:         task.TaskName,
			Input:            task.Input,
			Output:           task.Output,
			Worker:           task.Worker,
			Retry:            task.Retry,
			RetryInterval:    task.RetryInterval,
			Urgent:           task.Urgent,
			State:            task.State,
			Description:      task.Description,
			CreateTime:       task.CreateTime,
			LastRunTime:      task.LastRunTime,
			ExecuteTime:      task.ExecuteTime,
			Status:           task.Status,
			AffinityType:     task.AffinityType,
			PrimaryWorker:    task.PrimaryWorker,
			RollbackStrategy: task.RollbackStrategy,
		}
	}

	// 插入备份任务数据
	tx.Add(func(tx *bun.Tx) error {
		_, err := tx.NewInsert().Model(&taskBaks).Exec(context.Background())
		if err != nil {
			logger.Error("[backupTask] insert task backup failed. err: %v", err)
			return err
		}
		return nil
	})

	// 查询对应的子任务
	tx.Add(func(tx *bun.Tx) error {
		return tx.NewSelect().Table("subtask").
			Where("task_id IN (?)", bun.In(taskIds)).
			Order("id").Limit(100).
			Scan(context.TODO(), &subtasks)
	})

	// 准备子任务备份数据
	var subtaskBaks []taskmodel.SubtaskBak
	tx.Add(func(tx *bun.Tx) error {
		// 这里我们已经获取了subtasks，需要将它们转换为SubtaskBak模型
		subtaskBaks = make([]taskmodel.SubtaskBak, len(subtasks))
		for i, subtask := range subtasks {
			subtaskBaks[i] = taskmodel.SubtaskBak{
				ID:            subtask.ID,
				TaskID:        subtask.TaskID,
				PreSubtaskID:  subtask.PreSubtaskID,
				TaskName:      subtask.TaskName,
				Input:         subtask.Input,
				Output:        subtask.Output,
				State:         subtask.State,
				Worker:        subtask.Worker,
				Retry:         subtask.Retry,
				RetryInterval: subtask.RetryInterval,
				Rollback:      subtask.Rollback,
				LastRunTime:   subtask.LastRunTime,
				Status:        subtask.Status,
			}
		}
		return nil
	})

	// 插入备份子任务数据
	tx.Add(func(tx *bun.Tx) error {
		_, err := tx.NewInsert().Model(&subtaskBaks).Exec(context.Background())
		if err != nil {
			logger.Error("[backupTask] insert subtask backup failed. err: %v", err)
			return err
		}
		return nil
	})

	// 删除原始子任务数据
	tx.Add(func(tx *bun.Tx) error {
		result, err := tx.NewDelete().Table("subtask").Where("task_id IN (?)", bun.In(taskIds)).Exec(context.Background())
		if err != nil {
			logger.Error("[backupTask] delete subtasks during backup failed. err: %v", err)
			return err
		}
		rowsAffected, _ := result.RowsAffected()
		logger.Info("[backupTask] deleted %d subtasks for tasks: %v", rowsAffected, taskIds)
		return nil
	})

	// 删除任务对应的边数据
	tx.Add(func(tx *bun.Tx) error {
		result, err := tx.NewDelete().Table("task_edge").Where("task_id IN (?)", bun.In(taskIds)).Exec(context.Background())
		if err != nil {
			logger.Error("[backupTask] delete task_edges during backup failed. err: %v", err)
			return err
		}
		rowsAffected, _ := result.RowsAffected()
		logger.Trace("[backupTask] deleted %d task_edges for tasks: %v", rowsAffected, taskIds)
		return nil
	})

	// 删除原始任务数据
	tx.Add(func(tx *bun.Tx) error {
		result, err := tx.NewDelete().Table("task").Where("id IN (?)", bun.In(taskIds)).Exec(context.Background())
		if err != nil {
			logger.Error("[backupTask] delete tasks during backup failed. err: %v", err)
			return err
		}
		rowsAffected, _ := result.RowsAffected()
		logger.Info("[backupTask] deleted %d tasks: %v", rowsAffected, taskIds)
		return nil
	})

	if err := tx.Submit(); err != nil {
		logger.Error("[backupTask] tx submit failed. err: %v", err)
		logger.Warn("[backupTask] backup operation failed for task IDs: %v", taskIds)
		return
	}

	logger.Info("[backupTask] successfully backed up and deleted %d tasks: %v", len(taskIds), taskIds)
}
