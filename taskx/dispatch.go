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
	"errors"
	"math/rand"
	"sync"
	"time"

	"sync/atomic"

	"github.com/caiflower/common-tools/pkg/basic"
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

var initOnce sync.Once

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
	running                atomic.Value
	runningL               atomic.Value
	allocateWorkerInflight *inflight.InFlight
	inQueueTasks           sync.Map
	delayQueue             *basic.DelayQueue
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

type affinity struct {
	Type   TaskAffinityType
	Worker string
}

func InitTaskDispatcher(cfg *Config) {
	initOnce.Do(func() {
		_ = tools.DoTagFunc(&cfg, []tools.FnObj{{Fn: tools.SetDefaultValueIfNil}})
		_tr.subtaskWorker = cfg.SubtaskWorker
		_tr.taskWorker = cfg.TaskWorker
		_tr.subtaskRollbackWorker = cfg.SubtaskRollbackWorker
		_tr.subtaskQueueSize = cfg.SubtaskQueueSize
		_tr.taskQueueSize = cfg.TaskQueueSize
		_tr.subtaskRollbackQueueSize = cfg.SubtaskRollbackQueueSize
		SingletonTaskDispatcher.cfg = cfg
		SingletonTaskDispatcher.delayQueue = basic.NewDelayQueue()
		_tr.cfg = cfg
		bean.AddBean(dao.NewTaskDAO())
		bean.AddBean(dao.NewSubtaskBakDAO())
		bean.AddBean(SingletonTaskDispatcher)
		bean.AddBean(_tr)
	})
}

func (t *taskDispatcher) MasterCall() {
	if t.Cluster == nil {
		return
	}
	if t.runningL.Load() != nil && t.runningL.Load().(bool) {
		return
	}
	t.runningL.Store(true)

	golocalv1.PutTraceID(tools.UUID())
	defer func() {
		golocalv1.Clean()
		t.runningL.Store(false)
	}()

	// handle task
	t.handleTask(context.TODO())
	// back task
	//t.backupTask()
}

// OnStartedLeading handles task distribution when becoming leader
func (t *taskDispatcher) OnStartedLeading() {
	logger.Info("[taskDispatcher] %s begin to dispatcher task", t.Cluster.GetMyName())
	t.running.Store(true)

	// Start delay queue processor
	for t.running.Load().(bool) {
		// Take task from delay queue
		item := t.delayQueue.Take()

		// Handle batch task IDs
		var taskIDs = item.([]string)

		// Batch handle tasks
		if len(taskIDs) > 0 {

			for _, v := range taskIDs {
				t.inQueueTasks.Delete(v)
			}

			t.handleTaskImmediately(context.TODO(), taskIDs)
		}
	}
}

func (t *taskDispatcher) OnStoppedLeading() {
	logger.Info("[taskDispatcher] %s stop to dispatcher task", t.Cluster.GetMyName())
	t.running.Store(false)
}

func SubmitTask(task *Task) error {
	return SingletonTaskDispatcher.SubmitTask(task)
}

func (t *taskDispatcher) SubmitTask(task *Task) error {
	tx := dbv1.NewBatchTx(t.TaskDao.GetClient().GetDB())
	taskBean, subtaskBeans := task.convert2Bean()
	ctx := golocalv1.GetContext()

	if TaskAffinityType(taskBean.AffinityType) != AffinityRandom && taskBean.PrimaryWorker == "" {
		nodeName := t.selectNodeByAffinity(AffinityRandom, "", "", t.Cluster.GetLostNodeNames(), t.Cluster.GetAliveNodeNames())
		if nodeName == "" {
			return errors.New("task node name failed")
		}
		taskBean.PrimaryWorker = nodeName
	}

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
		funcSpec := cluster.NewAsyncFuncSpec(t.Cluster.GetLeaderName(), handleTaskImmediately, []string{taskID}, t.cfg.RemoteCallTimeout).SetTraceId(golocalv1.GetTraceID())
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
	// Query tasks that need to be executed within the next 2 minutes
	now := time.Now()
	endTime := basic.NewFromTime(now.Add(2 * time.Minute))

	// Get tasks by filter
	tasks, err := t.TaskDao.GetTodoTask(ctx, []string{TaskPending, TaskRunning, TaskSubtaskRunning}, endTime)
	if err != nil {
		logger.Error("get tasks failed. err: %s", err.Error())
		return
	}
	if len(tasks) == 0 {
		return
	}

	// Add task IDs to delay queue in batches
	var immediateTasks []string
	var scheduledTasks []struct {
		taskID      string
		executeTime time.Time
	}

	for _, task := range tasks {
		if _, ok := t.inQueueTasks.Load(task.ID); ok {
			continue
		}
		t.inQueueTasks.Store(task.ID, true)
		if !task.ExecuteTime.IsZero() {
			// Scheduled task
			scheduledTasks = append(scheduledTasks, struct {
				taskID      string
				executeTime time.Time
			}{task.ID, task.ExecuteTime.Time()})
		} else {
			// Immediate task
			immediateTasks = append(immediateTasks, task.ID)
		}
	}

	// Add immediate tasks as batch
	if len(immediateTasks) > 0 {
		logger.Debug("add task %v, executeTime = %s", immediateTasks, now.Format("2006-01-02 15:04:05.000"))
		t.delayQueue.Add(immediateTasks, now)
	}

	// Add scheduled tasks
	for _, task := range scheduledTasks {
		logger.Debug("add task %v, executeTime = %s", task.taskID, task.executeTime.Format("2006-01-02 15:04:05.000"))
		t.delayQueue.Add([]string{task.taskID}, task.executeTime)
	}
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

func (t *taskDispatcher) allocateWorker(_runningTasks []*model.Task, _runningSubtasks, _runningSubtaskRollbacks []*model.Subtask, taskAffinityMap map[string]affinity) {
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

	getAffinity := func(taskID string) affinity {
		affinityConf, exists := taskAffinityMap[taskID]
		if !exists {
			affinityConf = affinity{
				Type:   AffinityRandom,
				Worker: "",
			}
		}
		return affinityConf
	}

	for i := range runningSubtasks {
		runningSubtask := runningSubtasks[i]
		affinityConf := getAffinity(runningSubtask.TaskID)

		nodeName := t.selectNodeByAffinity(affinityConf.Type, affinityConf.Worker, runningSubtask.Worker, lostNodes, aliveNodes)
		if nodeName == "" {
			logger.Warn("allocate a worker failed: no available node for subtaskID: %s", runningSubtask.ID)
			continue
		}
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

	for i := range runningTasks {
		runningTask := runningTasks[i]
		affinityConf := getAffinity(runningTask.ID)

		nodeName := t.selectNodeByAffinity(affinityConf.Type, affinityConf.Worker, runningTask.Worker, lostNodes, aliveNodes)
		if nodeName == "" {
			logger.Warn("allocate a worker failed: no available node for taskID: %s", runningTask.ID)
			continue
		}
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
		affinityConf := getAffinity(runningSubtaskRollback.TaskID)

		nodeName := t.selectNodeByAffinity(affinityConf.Type, affinityConf.Worker, runningSubtaskRollback.Worker, lostNodes, aliveNodes)
		if nodeName == "" {
			logger.Warn("allocate a worker failed: no available node for subtaskRollbackID: %s", runningSubtaskRollback.ID)
			continue
		}
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

func (t *taskDispatcher) selectNodeByAffinity(taskAffinityType TaskAffinityType, primaryWorker string, currentNode string, lostNodes, aliveNodes []string) string {
	if len(aliveNodes) == 0 {
		logger.Warn("selectNode failed: no alive nodes available")
		return ""
	}

	switch taskAffinityType {
	case AffinityForceSameNode:
		if primaryWorker != "" && tools.StringSliceContains(aliveNodes, primaryWorker) && !tools.StringSliceContains(lostNodes, primaryWorker) {
			return primaryWorker
		}
		return ""
	case AffinityPreferSameNode:
		if primaryWorker != "" && tools.StringSliceContains(aliveNodes, primaryWorker) && !tools.StringSliceContains(lostNodes, primaryWorker) {
			return primaryWorker
		}
		if currentNode != "" && !tools.StringSliceContains(lostNodes, currentNode) {
			return currentNode
		}
		return aliveNodes[rand.Intn(len(aliveNodes))]
	case AffinityRandom:
		fallthrough
	default:
		if currentNode != "" && !tools.StringSliceContains(lostNodes, currentNode) {
			return currentNode
		}
		return aliveNodes[rand.Intn(len(aliveNodes))]
	}
}

func (t *taskDispatcher) deliverToCluster(workerMap map[string][]string, funcName string) {
	for nodeName, taskIds := range workerMap {
		_, err := t.Cluster.CallFunc(cluster.NewAsyncFuncSpec(nodeName, funcName, taskIds, t.cfg.RemoteCallTimeout).SetTraceId(golocalv1.GetTraceID()))
		if err != nil {
			logger.Error("deliver tasks failed. Error: %s", err.Error())
		}
	}
}

func (t *taskDispatcher) handleTaskImmediately(ctx context.Context, taskIDs []string) {
	logger.Debug("[taskDispatcher] tasks %v handleTask immediately", taskIDs)

	if !t.Cluster.IsReady() {
		logger.Warn("handleTaskImmediately failed. cluster is not ready.")
		return
	}
	if !t.Cluster.IsLeader() {
		logger.Warn("handleTaskImmediately failed. cluster is not leader")
		return
	}

	// Get all tasks
	tasks, err := t.TaskDao.GetByIDs(ctx, taskIDs)
	if err != nil {
		logger.Error("getTasksByTaskIds failed. err: %v", err)
		return
	}

	var (
		runningTasks     []*model.Task
		runningSubtasks  []*model.Subtask
		rollbackSubtasks []*model.Subtask
	)

	taskAffinityMap := make(map[string]affinity)
	for i := range tasks {
		dbTask := &tasks[i]

		subtasks, err := t.SubtaskDao.GetByTaskID(ctx, dbTask.ID)
		if err != nil {
			continue
		}

		task := &Task{}
		task, err = task.initByBean(dbTask, subtasks)
		if err != nil {
			logger.Error("task %v initByBean failed. err: %v", dbTask.ID, err)
			continue
		}
		taskAffinityMap[task.GetID()] = affinity{
			Type:   task.GetAffinityType(),
			Worker: task.GetPrimaryWorker(),
		}

		finished, retry, running, rollback := t.analysisTask(task, task.subtaskMap)
		if retry {
			continue
		} else if finished {
			runningTasks = append(runningTasks, dbTask)
		} else if len(running) > 0 {
			runningSubtasks = append(runningSubtasks, running...)
		} else if len(rollback) > 0 {
			rollbackSubtasks = append(rollbackSubtasks, rollback...)
		}
	}

	// Batch allocate workers
	if len(runningTasks) > 0 || len(runningSubtasks) > 0 || len(rollbackSubtasks) > 0 {
		t.allocateWorker(runningTasks, runningSubtasks, rollbackSubtasks, taskAffinityMap)
	}
}
