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

const (
	taskIdKey = "common-tools/taskx/taskId"
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
	TaskQueueSize            int           `yaml:"taskQueueSize" default:"1000"`
	SubtaskWorker            int           `yaml:"subtaskWorker" default:"100"`
	SubtaskQueueSize         int           `yaml:"subtaskQueueSize" default:"2000"`
	SubtaskRollbackWorker    int           `yaml:"subtaskRollbackWorker" default:"50"`
	SubtaskRollbackQueueSize int           `yaml:"subtaskRollbackQueueSize" default:"500"`
	RemoteCallTimeout        time.Duration `yaml:"remoteCallTimeout" default:"3s"`
	BackupTaskAgeSeconds     int           `yaml:"backupTaskAgeSeconds" default:"7200"`
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
	t.handleTask()
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
		go t.handleTaskImmediately(task.task.ID)
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

func (t *taskDispatcher) handleTask() {
	ctx := golocalv1.GetContext()

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
	for i, v := range tasks {
		taskID := v.ID
		subtasks, err := t.SubtaskDao.GetByTaskID(ctx, taskID)
		if err != nil {
			logger.Error("get task %v subtasks failed. err: %s", taskID, err.Error())
			continue
		}

		task := &Task{}
		task, err = task.initByBean(&v, subtasks)
		if err != nil {
			logger.Error("task %v init by bean failed. err: %s", taskID, err.Error())
			continue
		}

		subtaskMap := make(map[string]*model.Subtask)
		for j, vv := range subtasks {
			subtaskMap[vv.ID] = &subtasks[j]
		}

		finished, retry, running, rollback := t.analysisTask(task, &v, subtaskMap)
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

	t.allocateWorker(runningTasks, runningSubtasks, rollbackSubtasks, t.Cluster.GetAliveNodeNames(), t.Cluster.GetLostNodeNames())
}

func (t *taskDispatcher) analysisTask(task *Task, taskFromDB *model.Task, subtaskMap map[string]*model.Subtask) (finished, retry bool, runningSubtasks []*model.Subtask, rollbackSubtasks []*model.Subtask) {
	taskState := task.GetTaskState()
	state := task.task.State

	if state == TaskPending || state == TaskRunning || state == TaskSubtaskRunning {
		nextPendingSubTasks, rollback := task.NextSubTasks()
		if len(nextPendingSubTasks) > 0 {
			if rollback {
				for _, subtask := range nextPendingSubTasks {
					subtaskFromDB := subtaskMap[subtask.GetID()]
					if subtaskFromDB.Rollback == string(RollbackPending) ||
						time.Now().Add(time.Duration(subtaskFromDB.RetryInterval)*time.Second).After(subtaskFromDB.UpdateTime.Time()) {
						rollbackSubtasks = append(rollbackSubtasks, subtaskFromDB)
					}
				}
			} else {
				if task.GetTaskState() == TaskPending {
					taskState = TaskSubtaskRunning
				}

				for _, subtask := range nextPendingSubTasks {
					subtaskFromDB := subtaskMap[subtask.GetID()]
					if subtaskFromDB.State == TaskPending ||
						time.Now().Add(time.Duration(subtaskFromDB.RetryInterval)*time.Second).After(subtaskFromDB.UpdateTime.Time()) {
						runningSubtasks = append(runningSubtasks, subtaskFromDB)
					}
				}
			}
		} else {
			if time.Now().Add(time.Duration(taskFromDB.RetryInterval) * time.Second).After(taskFromDB.UpdateTime.Time()) {
				finished = true
			}

			if !finished {
				// if task has not retry，task updateTime must before all subtasks updateTime
				var subtaskLastUpdateTime time.Time
				for _, subtask := range subtaskMap {
					if subtask.UpdateTime.Time().After(subtaskLastUpdateTime) {
						subtaskLastUpdateTime = subtask.UpdateTime.Time()
					}
				}
				if subtaskLastUpdateTime.After(taskFromDB.UpdateTime.Time()) {
					finished = true
				}
			}
		}
	}

	if task.GetTaskState() != taskState {
		err := t.TaskDao.SetWorkerAndTaskState(golocalv1.GetContext(), task.GetID(), "", taskState)
		if err != nil {
			retry = true
			return
		}
	}

	return
}

func (t *taskDispatcher) allocateWorker(_runningTasks []*model.Task, _runningSubtasks, _runningSubtaskRollbacks []*model.Subtask, aliveNodes, lostNodes []string) {
	if len(_runningTasks) == 0 && len(_runningSubtasks) == 0 && len(_runningSubtaskRollbacks) == 0 {
		return
	}

	if !t.Cluster.IsReady() {
		logger.Warn("deliver tasks failed, cluster not ready")
		return
	}

	runningTaskIds := make([]string, 0, len(_runningTasks))
	runningSubtaskIds := make([]string, 0, len(_runningSubtasks))
	runningRollBackSubtaskIds := make([]string, 0, len(_runningSubtaskRollbacks))
	for _, runningTask := range _runningTasks {
		if !t.allocateWorkerInflight.InsertString(runningTask.ID) {
			continue
		}
		runningTaskIds = append(runningTaskIds, runningTask.ID)
	}
	for _, runningSubtask := range _runningSubtasks {
		if !t.allocateWorkerInflight.InsertString(runningSubtask.ID) {
			continue
		}
		runningSubtaskIds = append(runningSubtaskIds, runningSubtask.ID)
	}
	for _, runningSubtaskRollback := range _runningSubtaskRollbacks {
		if !t.allocateWorkerInflight.InsertString(runningSubtaskRollback.ID) {
			continue
		}
		runningRollBackSubtaskIds = append(runningRollBackSubtaskIds, runningSubtaskRollback.ID)
	}

	if len(runningTaskIds) == 0 && len(runningSubtaskIds) == 0 && len(runningRollBackSubtaskIds) == 0 {
		return
	}

	var (
		runningTasks            []model.Task
		runningSubtasks         []model.Subtask
		runningSubtaskRollbacks []model.Subtask
		ctx                     = golocalv1.GetContext()
	)

	if len(runningTaskIds) > 0 {
		runningTasks, _ = t.TaskDao.GetByIDs(ctx, runningTaskIds)
	}
	if len(runningSubtaskIds) > 0 {
		runningSubtasks, _ = t.SubtaskDao.GetSubtasksByIDs(ctx, runningSubtaskIds)
	}
	if len(runningRollBackSubtaskIds) > 0 {
		runningSubtaskRollbacks, _ = t.SubtaskDao.GetSubtasksByIDs(ctx, runningRollBackSubtaskIds)
	}

	defer func() {
		for _, runningTask := range runningTasks {
			t.allocateWorkerInflight.DeleteString(runningTask.ID)
		}
		for _, runningSubtask := range runningSubtasks {
			t.allocateWorkerInflight.DeleteString(runningSubtask.ID)
		}
		for _, runningSubtaskRollback := range runningSubtaskRollbacks {
			t.allocateWorkerInflight.DeleteString(runningSubtaskRollback.ID)
		}
	}()

	subtaskWorkerMap := make(map[string][]string)
	subtaskRollbackWorkerMap := make(map[string][]string)
	taskWorkerMap := make(map[string][]string)
	tx := dbv1.NewBatchTx(t.TaskDao.GetClient().GetDB())

	for _, runningSubtask := range runningSubtasks {
		var nodeName string
		if runningSubtask.State == TaskRunning && !tools.StringSliceContains(lostNodes, runningSubtask.Worker) {
			nodeName = runningSubtask.Worker
		} else {
			nodeName = aliveNodes[rand.Intn(len(aliveNodes))]
		}

		if nodeName != runningSubtask.Worker {
			subtaskId := runningSubtask.ID
			tx.Add(func(tx *bun.Tx) error {
				return t.SubtaskDao.SetWorkerAndState(ctx, subtaskId, nodeName, TaskRunning, tx)
			})
		}

		subtaskWorkerMap[nodeName] = append(subtaskWorkerMap[nodeName], runningSubtask.ID)
	}

	for _, runningTask := range runningTasks {
		var nodeName string
		if runningTask.State == TaskRunning && runningTask.Worker != "" && !tools.StringSliceContains(lostNodes, runningTask.Worker) {
			nodeName = runningTask.Worker
		} else {
			nodeName = aliveNodes[rand.Intn(len(aliveNodes))]
		}

		if nodeName != runningTask.Worker {
			taskId := runningTask.ID
			tx.Add(func(tx *bun.Tx) error {
				return t.TaskDao.SetWorkerAndTaskState(ctx, taskId, nodeName, TaskRunning, tx)
			})
		}

		taskWorkerMap[nodeName] = append(taskWorkerMap[nodeName], runningTask.ID)
	}

	for _, runningSubtaskRollback := range runningSubtaskRollbacks {
		var nodeName string
		if runningSubtaskRollback.Worker != "" && !tools.StringSliceContains(lostNodes, runningSubtaskRollback.Worker) {
			nodeName = runningSubtaskRollback.Worker
		} else {
			nodeName = aliveNodes[rand.Intn(len(aliveNodes))]
		}

		if nodeName != runningSubtaskRollback.Worker {
			subtaskId := runningSubtaskRollback.ID
			tx.Add(func(tx *bun.Tx) error {
				return t.SubtaskDao.SetWorkerAndRollback(ctx, subtaskId, nodeName, string(RollingBack), tx)
			})
		}

		subtaskRollbackWorkerMap[nodeName] = append(subtaskRollbackWorkerMap[nodeName], runningSubtaskRollback.ID)
	}

	if err := tx.Submit(); err != nil {
		logger.Error("allocateWorker for task failed. err: %s", err.Error())
		return
	}

	for nodeName, taskIds := range subtaskWorkerMap {
		_, err := t.Cluster.CallFunc(cluster.NewAsyncFuncSpec(nodeName, deliverSubtask, taskIds, t.cfg.RemoteCallTimeout).SetTraceId(golocalv1.GetTraceID()))
		if err != nil {
			logger.Error("deliver subtasks failed. err: %s", err.Error())
		}
	}

	for nodeName, taskIds := range taskWorkerMap {
		_, err := t.Cluster.CallFunc(cluster.NewAsyncFuncSpec(nodeName, deliverTask, taskIds, t.cfg.RemoteCallTimeout).SetTraceId(golocalv1.GetTraceID()))
		if err != nil {
			logger.Error("deliver tasks failed. err: %s", err.Error())
		}
	}

	for nodeName, taskIds := range subtaskRollbackWorkerMap {
		_, err := t.Cluster.CallFunc(cluster.NewAsyncFuncSpec(nodeName, deliverSubtaskRollback, taskIds, t.cfg.RemoteCallTimeout).SetTraceId(golocalv1.GetTraceID()))
		if err != nil {
			logger.Error("deliver subtaskRollbacks failed. err: %s", err.Error())
		}
	}
}

func HandleTaskImmediately(taskId string) {
	SingletonTaskDispatcher.HandleTaskImmediately(taskId)
}

func (t *taskDispatcher) HandleTaskImmediately(taskId string) {
	go t.handleTaskImmediately(taskId)
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

	subtaskMap := make(map[string]*model.Subtask)
	for i, vv := range subtasks {
		subtaskMap[vv.ID] = &subtasks[i]
	}

	finished, retry, runningSubtasks, rollbackSubtasks := t.analysisTask(task, dbTask, subtaskMap)
	if retry {
		return
	} else if finished {
		t.allocateWorker([]*model.Task{dbTask}, nil, nil, t.Cluster.GetAliveNodeNames(), t.Cluster.GetLostNodeNames())
	} else if len(runningSubtasks) > 0 || len(rollbackSubtasks) > 0 {
		t.allocateWorker(nil, runningSubtasks, rollbackSubtasks, t.Cluster.GetAliveNodeNames(), t.Cluster.GetLostNodeNames())
	}
}
