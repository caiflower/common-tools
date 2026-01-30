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
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/caiflower/common-tools/cluster"
	golocalv1 "github.com/caiflower/common-tools/pkg/golocal/v1"
	"github.com/caiflower/common-tools/pkg/inflight"
	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/caiflower/common-tools/pkg/tools"
	"github.com/caiflower/common-tools/taskx/dao"
	"github.com/caiflower/common-tools/taskx/dao/model"
)

const (
	deliverTask            = "github.caiflower.common.taskx.deliverTask"
	deliverSubtask         = "github.caiflower.common.taskx.deliverSubtask"
	deliverSubtaskRollback = "github.caiflower.common.taskx.deliverSubtaskRollback"
	handleTaskImmediately  = "github.caiflower.common.taskx.handleTaskImmediately"
)

var _tr = &taskReceiver{
	subtaskInflight: inflight.NewInFlight(),
	taskInflight:    inflight.NewInFlight(),
}

type SubtaskBag struct {
	subtask *model.Subtask
	task    *model.Task
}

type Output struct {
	Output         string `json:"output,omitempty"`
	Err            string `json:"err,omitempty"`
	RollbackErr    string `json:"rollbackErr,omitempty"`
	RollbackOutput string `json:"rollbackOutput,omitempty"`
}

func (o Output) String() string {
	return tools.ToJson(o)
}

type taskReceiver struct {
	Cluster        cluster.ICluster `autowired:""`
	TaskDao        dao.TaskDAO      `autowired:""`
	SubtaskDao     dao.SubtaskDAO   `autowired:""`
	TaskDispatcher *taskDispatcher  `autowired:""`
	cfg            *Config

	running                  atomic.Value
	subtaskWorker            int
	subtaskQueueSize         int
	subtaskInflight          *inflight.InFlight
	subtaskQueue             chan *SubtaskBag
	taskWorker               int
	taskQueueSize            int
	taskInflight             *inflight.InFlight
	taskQueue                chan *model.Task
	subtaskRollbackWorker    int
	subtaskRollbackQueueSize int
	subtaskRollbackQueue     chan *SubtaskBag
	stopChan                 chan struct{}
	wg                       sync.WaitGroup
}

func (t *taskReceiver) Start() error {
	if t.running.Load() != nil && t.running.Load().(bool) {
		logger.Warn("taskReceiver already running, skip start")
		return nil
	}

	logger.Info("taskReceiver start.")

	// Initialize stopChan before starting workers
	t.stopChan = make(chan struct{})

	t.subtaskQueue = make(chan *SubtaskBag, t.subtaskQueueSize)
	t.taskQueue = make(chan *model.Task, t.taskQueueSize)
	t.subtaskRollbackQueue = make(chan *SubtaskBag, t.subtaskRollbackQueueSize)

	t.startTaskThreads()
	t.startSubtaskThreads()
	t.startRollbackTaskThreads()
	t.running.Store(true)

	// register func in cluster
	t.Cluster.RegisterFunc(deliverSubtask, t.deliverSubtask)
	t.Cluster.RegisterFunc(deliverTask, t.deliverTask)
	t.Cluster.RegisterFunc(handleTaskImmediately, t.handleTaskImmediately)
	t.Cluster.RegisterFunc(deliverSubtaskRollback, t.deliverSubtaskRollback)
	logger.Info("taskReceiver started successfully")
	return nil
}

func (t *taskReceiver) Close() {
	if t.running.Load() == nil || !t.running.Load().(bool) {
		logger.Warn("TaskReceiver not running, skip close")
		return
	}

	t.running.Store(false)
	close(t.stopChan)

	// Wait for all workers to finish
	logger.Info("TaskReceiver waiting for workers to finish...")
	t.wg.Wait()

	logger.Info("TaskReceiver close finish.")
}

func (t *taskReceiver) deliverSubtask(data interface{}) (interface{}, error) {
	var subtaskIds []string
	if _subtaskIds, ok := data.([]string); !ok {
		err := tools.Unmarshal([]byte(tools.ToJson(data)), &subtaskIds)
		if err != nil {
			return nil, err
		}
	} else {
		subtaskIds = _subtaskIds
	}

	if len(subtaskIds) == 0 {
		return nil, nil
	}

	return t.handleSubtask(subtaskIds, false)
}

func (t *taskReceiver) handleSubtask(subtaskIds []string, rollback bool) (interface{}, error) {
	ctx := golocalv1.GetContext()

	subtasks, err := t.SubtaskDao.GetByIDs(ctx, subtaskIds)

	if err != nil {
		logger.Error("get subtasks by subtaskIds failed. err: %v", err.Error())
		return nil, err
	}
	if len(subtasks) == 0 {
		return nil, nil
	}

	var taskIds []string
	taskIdMap := make(map[string]*model.Task)
	for _, subtask := range subtasks {
		if _, ok := taskIdMap[subtask.TaskID]; !ok {
			taskIds = append(taskIds, subtask.TaskID)
		}
	}

	tasks, err := t.TaskDao.GetByIDs(ctx, taskIds)
	if err != nil {
		logger.Error("get task by taskIds failed. Error: %v", err.Error())
		return nil, err
	}
	for i, v := range tasks {
		taskIdMap[v.ID] = &tasks[i]
	}

	for i := range subtasks {
		subtask := subtasks[i]
		subtaskID := subtask.ID

		if subtask.Worker != t.Cluster.GetMyName() {
			logger.Warn("subtask '%s' is not my job. worker: '%s', myName: '%s'", subtaskID, subtask.Worker, t.Cluster.GetMyName())
			continue
		}

		if rollback && isRollbackFinished(subtask.Rollback) {
			logger.Warn("subtask '%s' already rollback", subtaskID)
			continue
		}
		if !rollback && isFinished(subtask.State) {
			logger.Warn("subtask '%s' is finished", subtaskID)
			continue
		}

		if t.running.Load() == nil || !t.running.Load().(bool) {
			logger.Warn("task receiver is closed")
			return nil, errors.New("task receiver is closed")
		}

		if !t.subtaskInflight.InsertString(subtaskID) {
			logger.Warn("subtask '%s' is inflight, rollback is %v", subtaskID, rollback)
			continue
		}

		if rollback {
			select {
			case t.subtaskRollbackQueue <- &SubtaskBag{
				subtask: &subtasks[i],
				task:    taskIdMap[subtask.TaskID],
			}:
			default:
				t.subtaskInflight.DeleteString(subtaskID)
				logger.Warn("subtask queue is full")
				return nil, errors.New("subtask queue is full")
			}
		} else {
			select {
			case t.subtaskQueue <- &SubtaskBag{
				subtask: &subtasks[i],
				task:    taskIdMap[subtask.TaskID],
			}:
			default:
				t.subtaskInflight.DeleteString(subtaskID)
				logger.Warn("subtask queue is full")
				return nil, errors.New("subtask queue is full")
			}
		}
	}
	return nil, nil
}

func (t *taskReceiver) deliverTask(data interface{}) (interface{}, error) {
	var taskIds []string

	if _taskIds, ok := data.([]string); !ok {
		err := tools.Unmarshal([]byte(tools.ToJson(data)), &taskIds)
		if err != nil {
			return nil, err
		}
	} else {
		taskIds = _taskIds
	}

	if len(taskIds) == 0 {
		return nil, nil
	}

	ctx := golocalv1.GetContext()
	tasks, err := t.TaskDao.GetByIDs(ctx, taskIds)
	if err != nil {
		logger.Error("get task by taskIds failed. err: %v", err.Error())
		return nil, err
	}
	for i := range tasks {
		task := tasks[i]
		taskID := task.ID

		if task.Worker != t.Cluster.GetMyName() {
			logger.Warn("task '%s' is not my job. worker: '%s', myName: '%s'", taskID, task.Worker, t.Cluster.GetMyName())
			continue
		}
		if isFinished(task.State) {
			logger.Warn("task '%s' is finished", taskID)
			continue
		}
		if t.running.Load() == nil || !t.running.Load().(bool) {
			logger.Warn("task receiver is closed")
			return nil, errors.New("task receiver is closed")
		}
		if !t.taskInflight.InsertString(taskID) {
			logger.Warn("task '%s' is inflight", taskID)
			continue
		}

		select {
		case t.taskQueue <- &task:
		default:
			t.taskInflight.DeleteString(taskID)
			logger.Warn("subtask queue is full")
			return nil, errors.New("task queue is full")
		}
	}

	return nil, nil
}

func (t *taskReceiver) deliverSubtaskRollback(data interface{}) (interface{}, error) {
	var subtaskIds []string
	if _subtaskIds, ok := data.([]string); !ok {
		err := tools.Unmarshal([]byte(tools.ToJson(data)), &subtaskIds)
		if err != nil {
			return nil, err
		}
	} else {
		subtaskIds = _subtaskIds
	}

	if len(subtaskIds) == 0 {
		return nil, nil
	}

	return t.handleSubtask(subtaskIds, true)
}

func (t *taskReceiver) startTaskThreads() {
	runThread := func(i int) {
		defer t.wg.Done()
		logger.Trace("TaskReceiver taskWorker %d start", i)
		for {
			select {
			case <-t.stopChan:
				logger.Trace("TaskReceiver taskWorker %d Exited (stop signal)", i)
				return
			case v := <-t.taskQueue:
				t.execTask(v)
			}
		}
	}

	t.wg.Add(t.taskWorker)
	for i := 1; i <= t.taskWorker; i++ {
		go runThread(i)
	}
}

func (t *taskReceiver) startSubtaskThreads() {
	runThread := func(i int) {
		defer t.wg.Done()
		logger.Trace("TaskReceiver subtaskWorker %d start", i)
		for {
			select {
			case <-t.stopChan:
				logger.Trace("TaskReceiver subtaskWorker %d Exited (stop signal)", i)
				return
			case v := <-t.subtaskQueue:

				t.execSubtask(v.task, v.subtask)
			}
		}
	}

	t.wg.Add(t.subtaskWorker)
	for i := 1; i <= t.subtaskWorker; i++ {
		go runThread(i)
	}
}

func (t *taskReceiver) startRollbackTaskThreads() {
	runThread := func(i int) {
		defer t.wg.Done()
		logger.Trace("TaskReceiver subtaskRollbackWorker %d start", i)
		for {
			select {
			case <-t.stopChan:
				logger.Trace("TaskReceiver subtaskRollbackWorker %d Exited (stop signal)", i)
				return
			case v := <-t.subtaskRollbackQueue:
				t.execSubtaskRollback(v.task, v.subtask)
			}
		}
	}

	t.wg.Add(t.subtaskRollbackWorker)
	for i := 1; i <= t.subtaskRollbackWorker; i++ {
		go runThread(i)
	}
}

func (t *taskReceiver) execTask(task *model.Task) {
	defer t.taskInflight.DeleteString(task.ID)

	golocalv1.PutTraceID(task.RequestID)
	defer golocalv1.Clean()
	ctx := context.TODO()
	taskID := task.ID

	executor := getTaskExecutor(task.TaskName)
	if executor == nil {
		logger.Error("task %v executor is not found", taskID)
		// 尝试将任务设置为失败状态
		err := t.TaskDao.SetOutputAndState(ctx, taskID, tools.ToJson(&Output{Err: fmt.Sprintf("executor for task %s not found", task.TaskName)}), TaskFailed)
		if err != nil {
			logger.Error("task %v set output and state failed. Error: %s", taskID, err.Error())
		}
		return
	}

	// check task state again
	_task, err := t.TaskDao.GetByID(ctx, taskID)
	if err != nil {
		logger.Error("get task %v failed. Error: %v", taskID, err)
		return
	}
	if _task == nil ||
		_task.Worker != t.Cluster.GetMyName() ||
		isFinished(_task.State) ||
		time.Now().Before(_task.LastRunTime.Time().Add(time.Duration(_task.RetryInterval)*time.Second)) {
		logger.Warn("task %v is not satisfy exec condition", taskID)
		return
	}

	subtaskMap := make(map[string]Output)
	subtasks, err := t.SubtaskDao.GetByTaskID(ctx, taskID)
	if err != nil {
		logger.Error("get task %v subtasks failed. Error: %v", taskID, err)
		return
	}

	failed := false
	for _, subtask := range subtasks {
		if subtask.State == TaskFailed {
			failed = true
		}
		var output Output
		_ = tools.Unmarshal([]byte(subtask.Output), &output)
		subtaskMap[subtask.TaskName] = output
	}

	var (
		state  string
		output string
	)

	if !failed {
		taskErr := executor.FinishedTask(&TaskData{
			RequestId: task.RequestID,
			TaskId:    task.ID,
			Input:     task.Input,
			Subtasks:  subtaskMap,
		})
		if taskErr != nil {
			if task.Retry > 0 && !errors.Is(taskErr, ErrNonRetryable) {
				if dErr := t.TaskDao.SetRetry(ctx, taskID, task.Retry-1); dErr != nil {
					logger.Error("task %v setRetry failed. Error: %v", taskID, dErr)
				}
				return
			}

			output = tools.ToJson(&Output{Err: taskErr.Error()})
			state = TaskFailed
		} else {
			state = TaskSucceeded
		}
	} else {
		taskErr := executor.FailedTask(&TaskData{
			RequestId: task.RequestID,
			TaskId:    task.ID,
			Input:     task.Input,
			Subtasks:  subtaskMap,
		})
		if taskErr != nil {
			if task.Retry > 0 && !errors.Is(taskErr, ErrNonRetryable) {
				if dErr := t.TaskDao.SetRetry(ctx, taskID, task.Retry-1); dErr != nil {
					logger.Error("task %v setRetry failed. Error: %v", taskID, dErr)
				}
				return
			}

			output = tools.ToJson(&Output{Err: taskErr.Error()})
		}
		state = TaskFailed
	}

	err = t.TaskDao.SetOutputAndState(ctx, taskID, output, state)
	if err != nil {
		logger.Error("task %v set output and state failed. Error: %s", taskID, err.Error())
		return
	}
}

func (t *taskReceiver) execSubtask(task *model.Task, subtask *model.Subtask) {
	defer t.subtaskInflight.DeleteString(subtask.ID)

	golocalv1.PutTraceID(task.RequestID)
	defer golocalv1.Clean()
	ctx := golocalv1.GetContext()
	taskID := task.ID
	subtaskID := subtask.ID

	// check subtask state again
	_subtask, err := t.SubtaskDao.GetByID(ctx, subtaskID)
	if err != nil {
		logger.Error("execSubtask: failed to get subtask %s, err: %v", subtaskID, err)
		return
	}
	if _subtask == nil ||
		_subtask.Worker != t.Cluster.GetMyName() ||
		isFinished(subtask.State) ||
		time.Now().Before(_subtask.LastRunTime.Time().Add(time.Duration(subtask.RetryInterval)*time.Second)) {
		logger.Warn("subtask '%s' is not satisfy exec condition", subtaskID)
		return
	}

	executor := getSubTaskExecutor(task.TaskName, subtask.TaskName)
	if executor == nil {
		logger.Error("subtask '%s' executor is not found", subtaskID)
		// 设置任务为失败状态
		if err = t.SubtaskDao.SetOutputAndState(ctx, subtaskID, tools.ToJson(&Output{Err: fmt.Sprintf("executor for task %s/%s not found", task.TaskName, subtask.TaskName)}), TaskFailed); err != nil {
			logger.Error("subtask %v set output and state failed. Error: %s", subtaskID, err.Error())
		}
		return
	}

	var (
		_output = &Output{}
		state   = ""
	)
	_ = tools.Unmarshal([]byte(subtask.Output), _output)
	output, err := t.exec(ctx, executor, taskID, subtask.PreSubtaskID, subtaskID, task.RequestID, subtask.Input)

	if err != nil {
		if subtask.Retry > 0 && !errors.Is(err, ErrNonRetryable) {
			if err = t.SubtaskDao.SetRetry(ctx, subtaskID, subtask.Retry-1); err != nil {
				logger.Error("subtask %v setRetry failed. Error: %v", subtaskID, err)
			}
			return
		}

		_output.Err = err.Error()
		state = TaskFailed
	} else {
		bytes, _ := tools.ToByte(output)
		_output.Output = string(bytes)
		state = TaskSucceeded
	}

	err = t.SubtaskDao.SetOutputAndState(ctx, subtaskID, tools.ToJson(_output), state)
	if err != nil {
		logger.Error("subtask %v set output and state failed. Error: %s", subtaskID, err.Error())
		return
	}

	if task.Urgent {
		_, err = t.Cluster.CallFunc(cluster.NewAsyncFuncSpec(t.Cluster.GetLeaderName(), handleTaskImmediately, []string{taskID}, t.cfg.RemoteCallTimeout).SetTraceId(golocalv1.GetTraceID()))
		if err != nil {
			logger.Warn("task %v remote call 'handleTaskImmediately' failed. Error: %v", taskID, err)
		}
	}
}

func (t *taskReceiver) execSubtaskRollback(task *model.Task, subtask *model.Subtask) {
	defer t.subtaskInflight.DeleteString(subtask.ID)

	golocalv1.PutTraceID(task.RequestID)
	defer golocalv1.Clean()

	var (
		ctx       = golocalv1.GetContext()
		subtaskID = subtask.ID
		taskID    = task.ID
	)

	// check subtask state again
	_subtask, err := t.SubtaskDao.GetByID(ctx, subtaskID)
	if err != nil {
		logger.Error("execSubtaskRollback: failed to get subtask %s, err: %v", subtaskID, err)
		return
	}
	if _subtask == nil ||
		_subtask.Worker != t.Cluster.GetMyName() ||
		isRollbackFinished(_subtask.Rollback) ||
		time.Now().Before(_subtask.LastRunTime.Time().Add(time.Duration(subtask.RetryInterval)*time.Second)) {
		logger.Warn("subtask '%s' is not satisfy rollback condition", subtaskID)
		return
	}

	executor := getRollbackTaskExecutor(task.TaskName, subtask.TaskName)
	if executor == nil {
		logger.Error("subtask '%s' rollback executor is not found", subtaskID)
		// 设置回滚状态为失败
		if err = t.SubtaskDao.SetRollbackAndState(ctx, subtaskID, string(RollbackFailed), tools.ToJson(&Output{RollbackErr: fmt.Sprintf("rollback executor for task %s/%s not found", task.TaskName, subtask.TaskName)})); err != nil {
			logger.Error("subtask %v set rollback and state failed. Error: %s", subtaskID, err.Error())
		}
		return
	}

	var (
		_output  = &Output{}
		rollback = ""
	)
	_ = tools.Unmarshal([]byte(subtask.Output), _output)
	output, err := t.exec(ctx, executor, taskID, subtask.PreSubtaskID, subtaskID, task.RequestID, subtask.Input)

	if err != nil {
		if subtask.Retry > 0 && !errors.Is(err, ErrNonRetryable) {
			if err = t.SubtaskDao.SetRetry(ctx, subtaskID, subtask.Retry-1); err != nil {
				logger.Error("subtask %v setRetry failed. err: %v", subtaskID, err)
			}
			return
		}

		_output.RollbackErr = err.Error()
		rollback = string(RollbackFailed)
	} else {
		bytes, _ := tools.ToByte(output)
		_output.RollbackOutput = string(bytes)
		rollback = string(RollbackSucceeded)
	}

	err = t.SubtaskDao.SetRollbackAndState(ctx, subtaskID, rollback, tools.ToJson(_output))
	if err != nil {
		logger.Error("subtask %v set rollback and state failed. Error: %s", subtaskID, err.Error())
		return
	}

	if task.Urgent {
		_, err = t.Cluster.CallFunc(cluster.NewAsyncFuncSpec(t.Cluster.GetLeaderName(), handleTaskImmediately, []string{taskID}, t.cfg.RemoteCallTimeout).SetTraceId(golocalv1.GetTraceID()))
		if err != nil {
			logger.Warn("task %v remote call 'handleTaskImmediately' failed. Error: %v", taskID, err)
		}
	}
}

func (t *taskReceiver) handleTaskImmediately(data interface{}) (interface{}, error) {
	logger.Debug("[remoteCall] tasks %v handleTaskImmediately", data)
	var taskIDs []string

	if _taskIds, ok := data.([]string); !ok {
		err := tools.Unmarshal([]byte(tools.ToJson(data)), &taskIDs)
		if err != nil {
			return nil, err
		}
	} else {
		taskIDs = _taskIds
	}

	t.TaskDispatcher.handleTaskImmediately(golocalv1.GetContext(), taskIDs)
	return nil, nil
}

func (t *taskReceiver) exec(ctx context.Context, executor SubTaskExecutor, taskID, preSubtaskID, subtaskID, requestID, input string) (interface{}, error) {
	preSubtasks := make(map[string]Output)
	if preSubtaskID != "" {
		preSubtaskIDs := strings.Split(preSubtaskID, ",")
		preSubtaskList, err := t.SubtaskDao.GetByIDs(ctx, preSubtaskIDs)
		if err != nil {
			logger.Error("subtask '%s' get preSubtasks failed. Error: %v", subtaskID, err)
			return "", err
		}
		for _, v := range preSubtaskList {
			var output Output
			_ = tools.Unmarshal([]byte(v.Output), &output)
			preSubtasks[v.TaskName] = output
		}
	}

	// 添加panic处理机制，捕获panic并返回错误
	var result interface{}
	var execErr error

	func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Error("panic recovered in exec for subtask '%s': %v", subtaskID, r)
				execErr = fmt.Errorf("panic occurred during execution: %v", r)
			}
		}()
		result, execErr = executor(&TaskData{RequestId: requestID, TaskId: taskID, SubTaskId: subtaskID, Input: input, Subtasks: preSubtasks})
	}()

	return result, execErr
}
