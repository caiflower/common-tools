package taskx

import (
	"context"

	dbv1 "github.com/caiflower/common-tools/db/v1"
	"github.com/caiflower/common-tools/pkg/logger"
	taskxdao "github.com/caiflower/common-tools/taskx/dao"
	"github.com/uptrace/bun"
)

func (t *taskDispatcher) backupTask() {
	var tasks []taskxdao.TaskBak
	var subtasks []taskxdao.SubtaskBak
	tx := dbv1.NewBatchTx(t.DBClient.GetDB())

	if err := t.DBClient.GetDB().NewSelect().Table("task").
		Where("task_state IN (?)", bun.In([]TaskState{TaskFailed, TaskSucceeded})).
		Order("id").Limit(100).
		Scan(context.TODO(), &tasks); err != nil {
		logger.Error("query task failed. err: %v", err)
	}

	taskIds := make([]string, 0)
	taskPrimaryKey := make([]int, 0)
	for _, task := range tasks {
		taskPrimaryKey = append(taskPrimaryKey, task.Id)
		taskIds = append(taskIds, task.TaskId)
	}
	if len(taskIds) == 0 {
		return
	}

	tx.Add(func(tx *bun.Tx) error {
		_, err := tx.NewInsert().Model(&tasks).Exec(context.Background())
		return err
	})

	tx.Add(func(tx *bun.Tx) error {
		_, err := tx.NewDelete().Table("task").Where("id IN (?)", bun.In(taskPrimaryKey)).Exec(context.Background())
		return err
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
