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
	"math/rand"
	"time"

	"github.com/caiflower/common-tools/pkg/inflight"

	"github.com/caiflower/common-tools/cluster"
	dbv1 "github.com/caiflower/common-tools/db/v1"
	"github.com/caiflower/common-tools/pkg/bean"
	golocalv1 "github.com/caiflower/common-tools/pkg/golocal/v1"
	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/caiflower/common-tools/pkg/tools"
	"github.com/caiflower/common-tools/taskx/dao"
	"github.com/caiflower/common-tools/taskx/dao/model"
	"github.com/uptrace/bun"
)

var SingletonTaskDispatcher = &taskDispatcher{
	allocateWorkerInflight: inflight.NewInFlight(),
}

type taskDispatcher struct {
	cluster.DefaultCaller
	Cluster                cluster.ICluster  `autowired:""`
	TaskDao                dao.TaskDAO       `autowired:""`
	TaskBakDao             dao.TaskBakDAO    `autowired:""`
	SubtaskDao             dao.SubtaskDAO    `autowired:""`
	SubtaskBakDao          dao.SubtaskBakDAO `autowired:""`
	DBClient               dbv1.DB           `autowired:""`
	TaskReceiver           *taskReceiver     `autowired:""`
	cfg                    *Config
	running                bool
	allocateWorkerInflight *inflight.InFlight
}

type Config struct {
	TaskWorker               int           `yaml:"taskWorker" default:"20"`
	TaskQueueSize            int           `yaml:"taskQueueSize" default:"100"`
	SubtaskWorker            int           `yaml:"subtaskWorker" default:"100"`
	SubtaskQueueSize         int           `yaml:"subtaskQueueSize" default:"200"`
	SubtaskRollbackWorker    int           `yaml:"subtaskRollbackWorker" default:"50"`
	SubtaskRollbackQueueSize int           `yaml:"subtaskRollbackQueueSize" default:"100"`
	RemoteCallTimeout        time.Duration `yaml:"remoteCallTimeout" default:"3s"`
	BackupTaskAge            time.Duration `yaml:"backupTaskAge" default:"168h"`
}

func InitTaskDispatcher(cfg *Config) {
	_ = tools.DoTagFunc(&cfg, []tools.FnObj{{Fn: tools.SetDefaultValueIfNil}})
	_tr.subtaskWorker = cfg.SubtaskWorker
	_tr.taskWorker = cfg.TaskWorker
	_tr.subtaskRollbackWorker = cfg.SubtaskRollbackWorker
	_tr.subtaskQueueSize = cfg.SubtaskQueueSize
	_tr.taskQueueSize = cfg.TaskQueueSize
	_tr.subtaskRollbackQueueSize = cfg.SubtaskRollbackQueueSize
	SingletonTaskDispatcher.cfg = cfg
	_tr.cfg = cfg
	bean.AddBean(dao.NewTaskDAO())
	bean.AddBean(dao.NewSubtaskBakDAO())
	bean.AddBean(SingletonTaskDispatcher)
	bean.AddBean(_tr)
}

func (t *taskDispatcher) MasterCall() {
	if t.running || t.Cluster == nil {
		return
	}
	t.running = true

	golocalv1.PutTraceID(tools.UUID())
	defer func() {
		t.running = false
		golocalv1.Clean()
	}()

	// handle task
	t.handleTask(context.TODO())
	// back task
	//t.backupTask()
}

func SubmitTask(task *Task) error {
	return SingletonTaskDispatcher.SubmitTask(task)
}

func (t *taskDispatcher) SubmitTask(task *Task) error {
	tx := dbv1.NewBatchTx(t.TaskDao.GetClient().GetDB())
	taskBean, subtaskBeans := task.convert2Bean()
	ctx := golocalv1.GetContext()

	tx.Add(func(tx *bun.Tx) error {
		_, err := t.TaskDao.Insert(ctx, taskBean, tx)
		return err
	})

	// if not rollback executor, set rollback to NoneRollback
	for i, subtask := range subtaskBeans {
		if getRollbackTaskExecutor(taskBean.TaskName, subtask.TaskName) == nil {
			subtaskBeans[i].Rollback = string(NoneRollback)
		}
	}

	tx.Add(func(tx *bun.Tx) error {
		_, err := t.SubtaskDao.BatchInsert(ctx, subtaskBeans, tx)
		return err
	})

	if err := tx.Submit(); err != nil {
		return err
	}
	if task.task.Urgent {
		taskID := taskBean.ID
		funcSpec := cluster.NewAsyncFuncSpec(t.Cluster.GetLeaderName(), handleTaskImmediately, taskID, t.cfg.RemoteCallTimeout).SetTraceId(golocalv1.GetTraceID())
		_, err := t.Cluster.CallFunc(funcSpec)
		if err != nil {
			logger.Warn("task %v remote call 'handleTaskImmediately' failed. Error: %v", taskID, err)
		}
	}

	return nil
}

func SubmitTaskWithTx(task *Task, tx *bun.Tx) error {
	return SingletonTaskDispatcher.SubmitTaskWithTx(task, tx)
}

func (t *taskDispatcher) SubmitTaskWithTx(task *Task, tx *bun.Tx) error {
	taskBean, subtaskBeans := task.convert2Bean()
	ctx := golocalv1.GetContext()
	_, err := t.TaskDao.Insert(ctx, taskBean, tx)
	if err != nil {
		return err
	}
	_, err = t.SubtaskDao.BatchInsert(ctx, subtaskBeans, tx)
	if err != nil {
		return err
	}
	return err
}

func (t *taskDispatcher) GetTaskOutput(taskID string) (outputs map[string]Output, err error) {
	outputs = make(map[string]Output)

	var (
		taskBak     *model.TaskBak
		subtaskBaks []model.SubtaskBak
		task        *model.Task
		subtasks    []model.Subtask
		ctx         = golocalv1.GetContext()
	)

	taskBak, err = t.TaskBakDao.GetByID(ctx, taskID)
	if err != nil {
		return
	}

	if taskBak != nil {
		output := Output{}
		_ = tools.Unmarshal([]byte(taskBak.Output), &output)
		outputs[taskBak.TaskName] = output
	} else {
		task, err = t.TaskDao.GetByID(ctx, taskID)
		if err != nil {
			return
		}
		output := Output{}
		_ = tools.Unmarshal([]byte(task.Output), &output)
		outputs[task.TaskName] = output
	}

	subtaskBaks, err = t.SubtaskBakDao.GetByTaskID(ctx, taskID)
	if err != nil {
		return
	}

	if len(subtaskBaks) > 0 {
		for _, subtask := range subtaskBaks {
			output := Output{}
			_ = tools.Unmarshal([]byte(subtask.Output), &output)
			outputs[subtask.TaskName] = output
		}
	} else {
		subtasks, err = t.SubtaskDao.GetByTaskID(ctx, taskID)
		if err != nil {
			return
		}
		for _, subtask := range subtasks {
			output := Output{}
			_ = tools.Unmarshal([]byte(subtask.Output), &output)
			outputs[subtask.TaskName] = output
		}
	}

	return
}

func (t *taskDispatcher) handleTask(ctx context.Context) {
	tasks, err := t.TaskDao.GetByTaskState(ctx, []string{TaskPending, TaskRunning, TaskSubtaskRunning})
	if err != nil {
		logger.Error("get tasks failed. err: %s", err.Error())
		return
	}
	if len(tasks) == 0 {
		return
	}

	var (
		runningTasks     []*model.Task
		runningSubtasks  []*model.Subtask
		rollbackSubtasks []*model.Subtask
	)

	for i, _ := range tasks {
		taskID := tasks[i].ID
		subtasks, err := t.SubtaskDao.GetByTaskID(ctx, taskID)
		if err != nil {
			logger.Error("get task %v subtasks failed. err: %s", taskID, err.Error())
			continue
		}

		task := &Task{}
		task, err = task.initByBean(&tasks[i], subtasks)
		if err != nil {
			logger.Error("task %v init by bean failed. err: %s", taskID, err.Error())
			continue
		}

		finished, retry, running, rollback := t.analysisTask(task, task.subtaskMap)
		if retry {
			continue
		} else if len(running) > 0 {
			runningSubtasks = append(runningSubtasks, running...)
		} else if finished {
			runningTasks = append(runningTasks, &tasks[i])
		} else if len(rollback) > 0 {
			rollbackSubtasks = append(rollbackSubtasks, rollback...)
		}
	}

	t.allocateWorker(runningTasks, runningSubtasks, rollbackSubtasks)
}

func (t *taskDispatcher) analysisTask(task *Task, subtaskMap map[string]*Subtask) (finished, retry bool, runningSubtasks []*model.Subtask, rollbackSubtasks []*model.Subtask) {
	if task.IsFinished() {
		return
	}

	nextPendingSubTasks, rollback := task.NextSubTasks()
	if len(nextPendingSubTasks) > 0 {
		if rollback {
			for _, subtask := range nextPendingSubTasks {
				subtaskFromDB := subtaskMap[subtask.GetID()]
				if t.canExecuteSubtask(subtaskFromDB, true) {
					rollbackSubtasks = append(rollbackSubtasks, subtaskFromDB.getModel())
				}
			}
			return
		}

		for _, subtask := range nextPendingSubTasks {
			subtaskFromDB := subtaskMap[subtask.GetID()]
			if t.canExecuteSubtask(subtaskFromDB, false) {
				runningSubtasks = append(runningSubtasks, subtaskFromDB.getModel())
			}
		}

		if task.GetState() == TaskPending {
			_, err := t.TaskDao.SetState(golocalv1.GetContext(), task.GetID(), TaskSubtaskRunning)
			if err != nil {
				retry = true
				return
			}
		}

		return
	}

	if time.Now().After(task.task.LastRunTime.Time().Add(time.Duration(task.task.RetryInterval) * time.Second)) {
		finished = true
	} else {
		retry = true
	}

	return
}

func (t *taskDispatcher) canExecuteSubtask(subtask *Subtask, isRollback bool) bool {
	now := time.Now()
	retryTime := subtask.GetLastRunTime().Time().Add(time.Duration(subtask.GetRetryInterval()) * time.Second)
	if !now.After(retryTime) {
		return false
	}

	if isRollback {
		return !subtask.IsRollbackFinished()
	}
	return !subtask.IsFinished()
}

func (t *taskDispatcher) allocateWorker(_runningTasks []*model.Task, _runningSubtasks, _runningSubtaskRollbacks []*model.Subtask) {
	if len(_runningTasks) == 0 && len(_runningSubtasks) == 0 && len(_runningSubtaskRollbacks) == 0 {
		return
	}

	if !t.Cluster.IsReady() {
		logger.Warn("deliver tasks failed, cluster not ready")
		return
	}

	aliveNodes, lostNodes := t.Cluster.GetAliveNodeNames(), t.Cluster.GetLostNodeNames()
	runningTasks := t.filterInflightTasks(_runningTasks)
	runningSubtasks := t.filterInflightSubtasks(_runningSubtasks)
	runningSubtaskRollbacks := t.filterInflightSubtasks(_runningSubtaskRollbacks)

	if len(runningTasks) == 0 && len(runningSubtasks) == 0 && len(runningSubtaskRollbacks) == 0 {
		return
	}

	defer t.cleanupInflight(runningTasks, runningSubtasks, runningSubtaskRollbacks)

	ctx := golocalv1.GetContext()
	subtaskWorkerMap := make(map[string][]string)
	subtaskRollbackWorkerMap := make(map[string][]string)
	taskWorkerMap := make(map[string][]string)

	for i := range runningSubtasks {
		runningSubtask := runningSubtasks[i]
		nodeName := t.selectNode(runningSubtask.State == TaskRunning, runningSubtask.Worker, lostNodes, aliveNodes)
		if nodeName != runningSubtask.Worker {
			cnt, err := t.SubtaskDao.SetWorkerAndStateWithOldWorker(ctx, runningSubtask.ID, nodeName, TaskRunning, runningSubtask.Worker)
			if err != nil {
				logger.Error("allocate a worker failed. subtaskID: %s, err: %s", runningSubtask.ID, err.Error())
				continue
			}
			if cnt == 0 {
				logger.Warn("allocate a worker failed, worker may be changed. subtaskID: %s", runningSubtask.ID)
				continue
			}
		}
		subtaskWorkerMap[nodeName] = append(subtaskWorkerMap[nodeName], runningSubtask.ID)
	}

	for i, _ := range runningTasks {
		runningTask := runningTasks[i]
		nodeName := t.selectNode(runningTask.State == TaskRunning && runningTask.Worker != "", runningTask.Worker, lostNodes, aliveNodes)
		if nodeName != runningTask.Worker {
			cnt, err := t.TaskDao.SetWorkerAndTaskStateWithOldWorker(ctx, runningTask.ID, nodeName, TaskRunning, runningTask.Worker)
			if err != nil {
				logger.Error("allocate a worker failed. taskID: %s, err: %s", runningTask.ID, err.Error())
				continue
			}
			if cnt == 0 {
				logger.Warn("allocate a worker failed, worker may be changed. taskID: %s", runningTask.ID)
				continue
			}
		}
		taskWorkerMap[nodeName] = append(taskWorkerMap[nodeName], runningTask.ID)
	}

	for i := range runningSubtaskRollbacks {
		runningSubtaskRollback := runningSubtaskRollbacks[i]
		nodeName := t.selectNode(runningSubtaskRollback.Worker != "", runningSubtaskRollback.Worker, lostNodes, aliveNodes)
		if nodeName != runningSubtaskRollback.Worker {
			cnt, err := t.SubtaskDao.SetWorkerAndRollbackWithOldWorker(ctx, runningSubtaskRollback.ID, nodeName, string(RollingBack), runningSubtaskRollback.Worker)
			if err != nil {
				logger.Error("allocate a worker failed. subtaskID: %s, err: %s", runningSubtaskRollback.ID, err.Error())
				continue
			}
			if cnt == 0 {
				logger.Warn("allocate a worker failed, worker may be changed. subtaskID: %s", runningSubtaskRollback.ID)
				continue
			}
		}
		subtaskRollbackWorkerMap[nodeName] = append(subtaskRollbackWorkerMap[nodeName], runningSubtaskRollback.ID)
	}

	t.deliverToCluster(subtaskWorkerMap, deliverSubtask)
	t.deliverToCluster(taskWorkerMap, deliverTask)
	t.deliverToCluster(subtaskRollbackWorkerMap, deliverSubtaskRollback)
}

func (t *taskDispatcher) filterInflightTasks(tasks []*model.Task) []model.Task {
	filtered := make([]model.Task, 0, len(tasks))
	for _, task := range tasks {
		if t.allocateWorkerInflight.InsertString(task.ID) {
			filtered = append(filtered, *task)
		}
	}
	return filtered
}

func (t *taskDispatcher) filterInflightSubtasks(subtasks []*model.Subtask) []model.Subtask {
	filtered := make([]model.Subtask, 0, len(subtasks))
	for _, subtask := range subtasks {
		if t.allocateWorkerInflight.InsertString(subtask.ID) {
			filtered = append(filtered, *subtask)
		}
	}
	return filtered
}

func (t *taskDispatcher) cleanupInflight(tasks []model.Task, subtasks, rollbackSubtasks []model.Subtask) {
	for _, task := range tasks {
		t.allocateWorkerInflight.DeleteString(task.ID)
	}
	for _, subtask := range subtasks {
		t.allocateWorkerInflight.DeleteString(subtask.ID)
	}
	for _, subtask := range rollbackSubtasks {
		t.allocateWorkerInflight.DeleteString(subtask.ID)
	}
}

func (t *taskDispatcher) selectNode(keepCurrentNode bool, currentNode string, lostNodes, aliveNodes []string) string {
	if keepCurrentNode && currentNode != "" && !tools.StringSliceContains(lostNodes, currentNode) {
		return currentNode
	}
	return aliveNodes[rand.Intn(len(aliveNodes))]
}

func (t *taskDispatcher) deliverToCluster(workerMap map[string][]string, funcName string) {
	for nodeName, taskIds := range workerMap {
		_, err := t.Cluster.CallFunc(cluster.NewAsyncFuncSpec(nodeName, funcName, taskIds, t.cfg.RemoteCallTimeout).SetTraceId(golocalv1.GetTraceID()))
		if err != nil {
			logger.Error("deliver tasks failed. Error: %s", err.Error())
		}
	}
}

func (t *taskDispatcher) handleTaskImmediately(taskId string) {
	ctx := golocalv1.GetContext()

	dbTask, err := t.TaskDao.GetByID(ctx, taskId)
	if err != nil {
		logger.Error("task %v getTasksByTaskIds failed. err: %v", taskId, err)
		return
	}
	if dbTask == nil {
		return
	}
	subtasks, err := t.SubtaskDao.GetByTaskID(ctx, taskId)
	if err != nil {
		return
	}

	task := &Task{}
	task, err = task.initByBean(dbTask, subtasks)
	if err != nil {
		logger.Error("task %v initByBean failed. err: %v", taskId, err)
		return
	}

	finished, retry, runningSubtasks, rollbackSubtasks := t.analysisTask(task, task.subtaskMap)
	if retry {
		return
	} else if finished {
		t.allocateWorker([]*model.Task{dbTask}, nil, nil)
	} else if len(runningSubtasks) > 0 || len(rollbackSubtasks) > 0 {
		t.allocateWorker(nil, runningSubtasks, rollbackSubtasks)
	}
}
