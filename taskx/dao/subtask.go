package taskxdao

import (
	"context"

	dbv1 "github.com/caiflower/common-tools/db/v1"
	"github.com/caiflower/common-tools/pkg/basic"
	"github.com/uptrace/bun"
)

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
	UpdateTime    basic.Time
	Status        int
}

func (t *Subtask) String() string {
	return t.SubtaskId
}

func (t *Subtask) IsFinished() bool {
	return t.TaskState == "Succeeded" || t.TaskState == "Failed"
}

type SubTaskDao struct {
	dbv1.IDB `autowired:""`
}

func (d *SubTaskDao) GetSubTasksByTaskId(taskId string) ([]*Subtask, map[string]*Subtask, error) {
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

func (d *SubTaskDao) SetOutputAndTaskState(subtaskId, output, taskState string, tx *bun.Tx) (int64, error) {
	update := d.GetUpdate(&Subtask{}, tx).Where("subtask_id = ?", subtaskId).Set("task_state = ?", taskState).Set("output = ?", output)
	return d.GetRowsAffected(update.Exec(context.TODO()))
}

func (d *SubTaskDao) SetWorkerAndTaskState(subtaskId string, worker string, taskState string, tx *bun.Tx) error {
	update := d.GetUpdate(&Subtask{}, tx).Where("subtask_id = ?", subtaskId).Set("worker = ?", worker).Set("task_state = ?", taskState)
	_, err := d.GetRowsAffected(update.Exec(context.TODO()))
	return err
}

func (d *SubTaskDao) GetSubtasksBySubtaskIds(subtaskIds []string) ([]*Subtask, error) {
	var subTasks []*Subtask
	err := d.GetSelect(&subTasks).Where("subtask_id IN (?)", bun.In(subtaskIds)).Scan(context.TODO(), &subTasks)
	return subTasks, err
}

func (d *SubTaskDao) SetRetry(subtaskId string, retry int, tx *bun.Tx) (err error) {
	_, err = d.GetRowsAffected(d.GetUpdate(&Subtask{}, tx).Where("subtask_id = ?", subtaskId).Set("retry = ?", retry).Exec(context.TODO()))
	return
}
