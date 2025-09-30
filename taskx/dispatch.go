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
	"reflect"
	"time"

	"github.com/caiflower/common-tools/pkg/inflight"

	"github.com/caiflower/common-tools/cluster"
	dbv1 "github.com/caiflower/common-tools/db/v1"
	"github.com/caiflower/common-tools/pkg/bean"
	"github.com/caiflower/common-tools/pkg/cache"
	golocalv1 "github.com/caiflower/common-tools/pkg/golocal/v1"
	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/caiflower/common-tools/pkg/tools"
	"github.com/caiflower/common-tools/taskx/dao"
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
	Cluster                cluster.ICluster     `autowired:""`
	TaskDao                *taskxdao.TaskDao    `autowired:""`
	SubtaskDao             *taskxdao.SubtaskDao `autowired:""`
	DBClient               dbv1.IDB             `autowired:""`
	TaskReceiver           *taskReceiver        `autowired:""`
	cfg                    *Config
	running                bool
	allocateWorkerInflight *inflight.InFlight
}

type Config struct {
	TaskWorker               int           `yaml:"taskWorker" default:"200"`
	TaskQueueSize            int           `yaml:"taskQueueSize" default:"1000"`
	SubtaskWorker            int           `yaml:"subtaskWorker" default:"400"`
	SubtaskQueueSize         int           `yaml:"subtaskQueueSize" default:"2000"`
	SubtaskRollbackWorker    int           `yaml:"subtaskRollbackWorker" default:"50"`
	SubtaskRollbackQueueSize int           `yaml:"subtaskRollbackQueueSize" default:"500"`
	RemoteCallTimout         time.Duration `yaml:"remoteCallTimout" default:"3s"`
	BackupTaskAgeSeconds     int           `yaml:"backupTaskAgeSeconds" default:"7200"`
}

func InitTaskDispatcher(cfg *Config) {
	_ = tools.DoTagFunc(&cfg, nil, []func(reflect.StructField, reflect.Value, interface{}) error{tools.SetDefaultValueIfNil})
	_tr.subtaskWorker = cfg.SubtaskWorker
	_tr.taskWorker = cfg.TaskWorker
	_tr.subtaskRollbackWorker = cfg.SubtaskRollbackWorker
	_tr.subtaskQueueSize = cfg.SubtaskQueueSize
	_tr.taskQueueSize = cfg.TaskQueueSize
	_tr.subtaskRollbackQueueSize = cfg.SubtaskRollbackQueueSize
	SingletonTaskDispatcher.cfg = cfg
	_tr.cfg = cfg
	bean.AddBean(&taskxdao.Task{})
	bean.AddBean(&taskxdao.Subtask{})
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
	t.backupTask()
}

func SubmitTask(task *Task) error {
	return SingletonTaskDispatcher.SubmitTask(task)
}

func (t *taskDispatcher) SubmitTask(task *Task) error {
	tx := dbv1.NewBatchTx(t.TaskDao.GetDB())
	taskBean, subtaskBeans := task.convert2Bean()
	tx.Add(func(tx *bun.Tx) error {
		_, err := t.TaskDao.Insert(taskBean, tx)
		return err
	})

	// if not rollback executor, set rollback to NoneRollback
	for _, subtask := range subtaskBeans {
		if getRollbackTaskExecutor(taskBean.TaskName, subtask.TaskName) == nil {
			subtask.Rollback = string(NoneRollback)
		}
	}

	tx.Add(func(tx *bun.Tx) error {
		_, err := t.SubtaskDao.Insert(&subtaskBeans, tx)
		return err
	})

	if err := tx.Submit(); err != nil {
		return err
	}
	if task.urgent {
		go t.handleTaskImmediately(task.taskId)
	}

	return nil
}

func SubmitTaskWithTx(task *Task, tx *bun.Tx) error {
	return SingletonTaskDispatcher.SubmitTaskWithTx(task, tx)
}

func (t *taskDispatcher) SubmitTaskWithTx(task *Task, tx *bun.Tx) error {
	taskBean, subtaskBeans := task.convert2Bean()
	_, err := t.TaskDao.Insert(taskBean, tx)
	if err != nil {
		return err
	}
	_, err = t.SubtaskDao.Insert(&subtaskBeans, tx)
	if err != nil {
		return err
	}
	return err
}

func (t *taskDispatcher) GetTaskOutput(taskID string) (outputs map[string]Output, err error) {
	outputs = make(map[string]Output)

	var (
		taskBaks    []taskxdao.TaskBak
		subtaskBaks []taskxdao.SubtaskBak
		tasks       []*taskxdao.Task
		subtasks    []*taskxdao.Subtask
	)

	err = t.TaskDao.GetSelect(&taskBaks).Where("task_id = ?", taskID).Scan(context.TODO(), &taskBaks)
	if err != nil {
		return
	}

	if len(taskBaks) > 0 {
		output := Output{}
		_ = tools.Unmarshal([]byte(taskBaks[0].Output), &output)
		outputs[taskBaks[0].TaskName] = output
	} else {
		tasks, err = t.TaskDao.GetTasksByTaskIds([]string{taskID})
		if err != nil {
			return
		}
		output := Output{}
		_ = tools.Unmarshal([]byte(tasks[0].Output), &output)
		outputs[tasks[0].TaskName] = output
	}

	err = t.TaskDao.GetSelect(&subtaskBaks).Where("task_id = ?", taskID).Scan(context.TODO(), &subtaskBaks)
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
		subtasks, _, err = t.SubtaskDao.GetSubtasksByTaskId(taskID)
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
	id := 0
	if tmp, e := cache.LocalCache.Get(taskIdKey); e {
		id = tmp.(int)
	}

	tasks, err := t.TaskDao.GetByTaskState([]string{string(TaskPending), string(TaskRunning), string(TaskSubtaskRunning)}, id)
	if err != nil {
		logger.Error("get tasks failed. err: %s", err.Error())
		return
	}
	if len(tasks) == 0 {
		return
	}
	cache.LocalCache.Set(taskIdKey, tasks[0].Id, 0)

	var (
		runningTasks     []*taskxdao.Task
		runningSubtasks  []*taskxdao.Subtask
		rollbackSubtasks []*taskxdao.Subtask
	)
	for _, v := range tasks {
		subtasks, subtaskMap, err := t.SubtaskDao.GetSubtasksByTaskId(v.TaskId)
		if err != nil {
			logger.Error("get task %v subtasks failed. err: %s", v.TaskId, err.Error())
			continue
		}

		task := &Task{}
		task, err = task.initByBean(v, subtasks)
		if err != nil {
			logger.Error("task %v init by bean failed. err: %s", v.TaskId, err.Error())
			continue
		}

		finished, retry, running, rollback := t.analysisTask(task, v, subtaskMap)
		if retry {
			continue
		} else if len(running) > 0 {
			runningSubtasks = append(runningSubtasks, running...)
		} else if finished {
			runningTasks = append(runningTasks, v)
		} else if len(rollback) > 0 {
			rollbackSubtasks = append(rollbackSubtasks, rollback...)
		}
	}

	t.allocateWorker(runningTasks, runningSubtasks, rollbackSubtasks, t.Cluster.GetAliveNodeNames(), t.Cluster.GetLostNodeNames())
}

func (t *taskDispatcher) analysisTask(task *Task, taskFromDB *taskxdao.Task, subtaskMap map[string]*taskxdao.Subtask) (finished, retry bool, runningSubtasks []*taskxdao.Subtask, rollbackSubtasks []*taskxdao.Subtask) {
	taskState := task.GetTaskState()

	if task.taskState == TaskPending || task.taskState == TaskRunning || task.taskState == TaskSubtaskRunning {
		nextPendingSubTasks, rollback := task.NextSubTasks()
		if len(nextPendingSubTasks) > 0 {
			if rollback {
				for _, subtask := range nextPendingSubTasks {
					subtaskFromDB := subtaskMap[subtask.GetTaskId()]
					if subtaskFromDB.Rollback == string(RollbackPending) ||
						time.Now().Add(time.Duration(subtaskFromDB.RetryInterval)*time.Second).After(subtaskFromDB.UpdateTime.Time()) {
						rollbackSubtasks = append(rollbackSubtasks, subtaskFromDB)
					}
				}
			} else {
				if task.taskState == TaskPending {
					taskState = TaskSubtaskRunning
				}

				for _, subtask := range nextPendingSubTasks {
					subtaskFromDB := subtaskMap[subtask.GetTaskId()]
					if subtaskFromDB.TaskState == string(TaskPending) ||
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

	if task.taskState != taskState {
		_, err := t.TaskDao.SetTaskState(task.taskId, string(taskState), nil)
		if err != nil {
			retry = true
			return
		}
	}

	return
}

func (t *taskDispatcher) allocateWorker(_runningTasks []*taskxdao.Task, _runningSubtasks, _runningSubtaskRollbacks []*taskxdao.Subtask, aliveNodes, lostNodes []string) {
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
		if !t.allocateWorkerInflight.Insert(runningTask) {
			continue
		}
		runningTaskIds = append(runningTaskIds, runningTask.TaskId)
	}
	for _, runningSubtask := range _runningSubtasks {
		if !t.allocateWorkerInflight.Insert(runningSubtask) {
			continue
		}
		runningSubtaskIds = append(runningSubtaskIds, runningSubtask.SubtaskId)
	}
	for _, runningSubtaskRollback := range _runningSubtaskRollbacks {
		if !t.allocateWorkerInflight.Insert(runningSubtaskRollback) {
			continue
		}
		runningRollBackSubtaskIds = append(runningRollBackSubtaskIds, runningSubtaskRollback.SubtaskId)
	}

	if len(runningTaskIds) == 0 && len(runningSubtaskIds) == 0 && len(runningRollBackSubtaskIds) == 0 {
		return
	}

	var (
		runningTasks            []*taskxdao.Task
		runningSubtasks         []*taskxdao.Subtask
		runningSubtaskRollbacks []*taskxdao.Subtask
	)

	if len(runningTaskIds) > 0 {
		runningTasks, _ = t.TaskDao.GetTasksByTaskIds(runningTaskIds)
	}
	if len(runningSubtaskIds) > 0 {
		runningSubtasks, _ = t.SubtaskDao.GetSubtasksBySubtaskIds(runningSubtaskIds)
	}
	if len(runningRollBackSubtaskIds) > 0 {
		runningSubtaskRollbacks, _ = t.SubtaskDao.GetSubtasksBySubtaskIds(runningRollBackSubtaskIds)
	}

	defer func() {
		for _, runningTask := range runningTasks {
			t.allocateWorkerInflight.Delete(runningTask)
		}
		for _, runningSubtask := range runningSubtasks {
			t.allocateWorkerInflight.Delete(runningSubtask)
		}
		for _, runningSubtaskRollback := range runningSubtaskRollbacks {
			t.allocateWorkerInflight.Delete(runningSubtaskRollback)
		}
	}()

	subtaskWorkerMap := make(map[string][]string)
	subtaskRollbackWorkerMap := make(map[string][]string)
	taskWorkerMap := make(map[string][]string)
	tx := dbv1.NewBatchTx(t.TaskDao.GetDB())

	for _, runningSubtask := range runningSubtasks {
		var nodeName string
		if runningSubtask.TaskState == string(TaskRunning) && !tools.StringSliceContains(lostNodes, runningSubtask.Worker) {
			nodeName = runningSubtask.Worker
		} else {
			nodeName = aliveNodes[rand.Intn(len(aliveNodes))]
		}

		if nodeName != runningSubtask.Worker {
			subtaskId := runningSubtask.SubtaskId
			tx.Add(func(tx *bun.Tx) error {
				return t.SubtaskDao.SetWorkerAndTaskState(subtaskId, nodeName, string(TaskRunning), tx)
			})
		}

		subtaskWorkerMap[nodeName] = append(subtaskWorkerMap[nodeName], runningSubtask.SubtaskId)
	}

	for _, runningTask := range runningTasks {
		var nodeName string
		if runningTask.TaskState == string(TaskRunning) && runningTask.Worker != "" && !tools.StringSliceContains(lostNodes, runningTask.Worker) {
			nodeName = runningTask.Worker
		} else {
			nodeName = aliveNodes[rand.Intn(len(aliveNodes))]
		}

		if nodeName != runningTask.Worker {
			taskId := runningTask.TaskId
			tx.Add(func(tx *bun.Tx) error {
				return t.TaskDao.SetWorkerAndTaskState(taskId, nodeName, string(TaskRunning), tx)
			})
		}

		taskWorkerMap[nodeName] = append(taskWorkerMap[nodeName], runningTask.TaskId)
	}

	for _, runningSubtaskRollback := range runningSubtaskRollbacks {
		var nodeName string
		if runningSubtaskRollback.Worker != "" && !tools.StringSliceContains(lostNodes, runningSubtaskRollback.Worker) {
			nodeName = runningSubtaskRollback.Worker
		} else {
			nodeName = aliveNodes[rand.Intn(len(aliveNodes))]
		}

		if nodeName != runningSubtaskRollback.Worker {
			subtaskId := runningSubtaskRollback.SubtaskId
			tx.Add(func(tx *bun.Tx) error {
				return t.SubtaskDao.SetWorkerAndRollback(subtaskId, nodeName, string(RollingBack), tx)
			})
		}

		subtaskRollbackWorkerMap[nodeName] = append(subtaskRollbackWorkerMap[nodeName], runningSubtaskRollback.SubtaskId)
	}

	if err := tx.Submit(); err != nil {
		logger.Error("allocateWorker for task failed. err: %s", err.Error())
		return
	}

	for nodeName, taskIds := range subtaskWorkerMap {
		_, err := t.Cluster.CallFunc(cluster.NewAsyncFuncSpec(nodeName, deliverSubtask, taskIds, t.cfg.RemoteCallTimout).SetTraceId(golocalv1.GetTraceID()))
		if err != nil {
			logger.Error("deliver subtasks failed. err: %s", err.Error())
		}
	}

	for nodeName, taskIds := range taskWorkerMap {
		_, err := t.Cluster.CallFunc(cluster.NewAsyncFuncSpec(nodeName, deliverTask, taskIds, t.cfg.RemoteCallTimout).SetTraceId(golocalv1.GetTraceID()))
		if err != nil {
			logger.Error("deliver tasks failed. err: %s", err.Error())
		}
	}

	for nodeName, taskIds := range subtaskRollbackWorkerMap {
		_, err := t.Cluster.CallFunc(cluster.NewAsyncFuncSpec(nodeName, deliverSubtaskRollback, taskIds, t.cfg.RemoteCallTimout).SetTraceId(golocalv1.GetTraceID()))
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
	tasks, err := t.TaskDao.GetTasksByTaskIds([]string{taskId})
	if err != nil {
		logger.Error("task %v getTasksByTaskIds failed. err: %v", taskId, err)
		return
	}
	if len(tasks) == 0 {
		return
	}
	subtasks, subtaskMap, err := t.SubtaskDao.GetSubtasksByTaskId(taskId)
	if err != nil {
		return
	}

	task := &Task{}
	task, err = task.initByBean(tasks[0], subtasks)
	if err != nil {
		logger.Error("task %v initByBean failed. err: %v", taskId, err)
		return
	}

	finished, retry, runningSubtasks, rollbackSubtasks := t.analysisTask(task, tasks[0], subtaskMap)
	if retry {
		return
	} else if finished {
		t.allocateWorker(tasks, nil, nil, t.Cluster.GetAliveNodeNames(), t.Cluster.GetLostNodeNames())
	} else if len(runningSubtasks) > 0 || len(rollbackSubtasks) > 0 {
		t.allocateWorker(nil, runningSubtasks, rollbackSubtasks, t.Cluster.GetAliveNodeNames(), t.Cluster.GetLostNodeNames())
	}
}
