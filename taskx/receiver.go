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
	"github.com/caiflower/common-tools/taskx/executor"
	"github.com/caiflower/common-tools/taskx/proto"
)

const (
	deliverTask            = "github.caiflower.common.taskx.deliverTask"
	deliverSubtask         = "github.caiflower.common.taskx.deliverSubtask"
	deliverSubtaskRollback = "github.caiflower.common.taskx.deliverSubtaskRollback"
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
	grpcRegisterOnce         sync.Once
	grpcRegisterErr          error
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

func (t *taskReceiver) RegisterGRPCService() error {
	t.grpcRegisterOnce.Do(func() {
		t.grpcRegisterErr = t.Cluster.RegisterGRPCService(&proto.TaskXService_ServiceDesc, newTaskXServiceServer(t))
	})
	return t.grpcRegisterErr
}

func (t *taskReceiver) Start() error {
	if t.running.Load() != nil && t.running.Load().(bool) {
		logger.Warn("[taskReceiver] already running, skip start")
		return nil
	}

	logger.Info("[taskReceiver] starting...")

	if err := t.RegisterGRPCService(); err != nil {
		return err
	}
	// Initialize stopChan before starting workers
	t.stopChan = make(chan struct{})

	t.subtaskQueue = make(chan *SubtaskBag, t.subtaskQueueSize)
	t.taskQueue = make(chan *model.Task, t.taskQueueSize)
	t.subtaskRollbackQueue = make(chan *SubtaskBag, t.subtaskRollbackQueueSize)

	//t.startTaskThreads()
	t.startSubtaskThreads()
	t.startRollbackTaskThreads()
	t.running.Store(true)

	logger.Info("[taskReceiver] started successfully")
	return nil
}

func (t *taskReceiver) Close() {
	if t.running.Load() == nil || !t.running.Load().(bool) {
		logger.Warn("[taskReceiver] not running, skip close")
		return
	}

	t.running.Store(false)
	close(t.stopChan)

	// Wait for all workers to finish
	logger.Info("[taskReceiver] waiting for workers to finish...")
	t.wg.Wait()

	logger.Info("[taskReceiver] closed")
}

func (t *taskReceiver) deliverSubtask(ctx context.Context, subtaskIds []string) error {
	if len(subtaskIds) == 0 {
		return nil
	}

	return t.handleSubtask(ctx, subtaskIds, false)
}

func (t *taskReceiver) handleSubtask(ctx context.Context, subtaskIds []string, rollback bool) error {

	subtasks, err := t.SubtaskDao.GetByIDs(ctx, subtaskIds)

	if err != nil {
		logger.Error("[handleSubtask] get subtasks by IDs failed. err: %v", err)
		return err
	}
	if len(subtasks) == 0 {
		return nil
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
		logger.Error("[handleSubtask] get tasks by IDs failed. err: %v", err)
		return err
	}
	for i, v := range tasks {
		taskIdMap[v.ID] = &tasks[i]
	}

	for i := range subtasks {
		subtask := subtasks[i]
		subtaskID := subtask.ID

		if subtask.Worker != t.Cluster.GetMyName() {
			logger.Trace("[handleSubtask] subtask %s is not my job, assigned to %s", subtaskID, subtask.Worker)
			continue
		}

		if rollback && isRollbackFinished(subtask.Rollback) {
			logger.Trace("[handleSubtask] subtask %s already rollback", subtaskID)
			continue
		}
		if !rollback && isFinished(subtask.State) {
			logger.Trace("[handleSubtask] subtask %s is finished", subtaskID)
			continue
		}

		if t.running.Load() == nil || !t.running.Load().(bool) {
			logger.Warn("[handleSubtask] receiver is closed")
			return errors.New("task receiver is closed")
		}

		if !t.subtaskInflight.InsertString(subtaskID) {
			logger.Info("[handleSubtask] subtask %s is inflight, rollback=%v", subtaskID, rollback)
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
				logger.Warn("[handleSubtask] subtask rollback queue is full, subtask %s dropped", subtaskID)
				return errors.New("subtask queue is full")
			}
		} else {
			select {
			case t.subtaskQueue <- &SubtaskBag{
				subtask: &subtasks[i],
				task:    taskIdMap[subtask.TaskID],
			}:
			default:
				t.subtaskInflight.DeleteString(subtaskID)
				logger.Warn("[handleSubtask] subtask queue is full, subtask %s dropped", subtaskID)
				return errors.New("subtask queue is full")
			}
		}
	}
	return nil
}

func (t *taskReceiver) deliverTask(ctx context.Context, taskIds []string) error {
	if len(taskIds) == 0 {
		return nil
	}

	tasks, err := t.TaskDao.GetByIDs(ctx, taskIds)
	if err != nil {
		logger.Error("[deliverTask] get tasks by IDs failed. err: %v", err)
		return err
	}
	for i := range tasks {
		task := tasks[i]
		taskID := task.ID

		if task.Worker != t.Cluster.GetMyName() {
			logger.Trace("[deliverTask] task %s is not my job, assigned to %s", taskID, task.Worker)
			continue
		}
		if isFinished(task.State) {
			logger.Trace("[deliverTask] task %s is finished", taskID)
			continue
		}
		if t.running.Load() == nil || !t.running.Load().(bool) {
			logger.Trace("[deliverTask] receiver is closed")
			return errors.New("task receiver is closed")
		}
		if !t.taskInflight.InsertString(taskID) {
			logger.Trace("[deliverTask] task %s is inflight", taskID)
			continue
		}

		select {
		case t.taskQueue <- &task:
		default:
			t.taskInflight.DeleteString(taskID)
			logger.Warn("[deliverTask] task queue is full, task %s dropped", taskID)
			return errors.New("task queue is full")
		}
	}

	return nil
}

func (t *taskReceiver) deliverSubtaskRollback(ctx context.Context, subtaskIds []string) error {
	if len(subtaskIds) == 0 {
		return nil
	}

	return t.handleSubtask(ctx, subtaskIds, true)
}

//func (t *taskReceiver) startTaskThreads() {
//	runThread := func(i int) {
//		defer t.wg.Done()
//		logger.Trace("[taskWorker] %d start", i)
//		for {
//			select {
//			case <-t.stopChan:
//				logger.Trace("[taskWorker] %d exited (stop signal)", i)
//				return
//			case v := <-t.taskQueue:
//				t.execTask(v)
//			}
//		}
//	}
//
//	t.wg.Add(t.taskWorker)
//	for i := 1; i <= t.taskWorker; i++ {
//		go runThread(i)
//	}
//}

func (t *taskReceiver) startSubtaskThreads() {
	runThread := func(i int) {
		defer t.wg.Done()
		logger.Trace("[subtaskWorker] %d start", i)
		for {
			select {
			case <-t.stopChan:
				logger.Trace("[subtaskWorker] %d exited (stop signal)", i)
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
		logger.Trace("[subtaskRollbackWorker] %d start", i)
		for {
			select {
			case <-t.stopChan:
				logger.Trace("[subtaskRollbackWorker] %d exited (stop signal)", i)
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

//func (t *taskReceiver) execTask(task *model.Task) {
//	defer t.taskInflight.DeleteString(task.ID)
//
//	golocalv1.PutTraceID(task.RequestID)
//	defer golocalv1.Clean()
//	ctx := golocalv1.GetContext()
//	taskID := task.ID
//
//	executor := getTaskExecutor(task.TaskName)
//	if executor == nil {
//		logger.Error("[execTask] task %s executor not found", taskID)
//		err := t.TaskDao.SetOutputAndState(ctx, taskID, tools.ToJson(&Output{Err: fmt.Sprintf("executor for task %s not found", task.TaskName)}), string(TaskFailed))
//		if err != nil {
//			logger.Error("[execTask] task %s set output and state failed. err: %v", taskID, err)
//		}
//		return
//	}
//
//	// check task state again
//	_task, err := t.TaskDao.GetByID(ctx, taskID)
//	if err != nil {
//		logger.Error("[execTask] get task %s failed. err: %v", taskID, err)
//		return
//	}
//	if _task == nil ||
//		_task.Worker != t.Cluster.GetMyName() ||
//		isFinished(_task.State) ||
//		time.Now().Before(_task.LastRunTime.Time().Add(time.Duration(_task.RetryInterval)*time.Second)) {
//		logger.Info("[execTask] task %s not satisfy exec condition", taskID)
//		return
//	}
//
//	subtaskMap := make(map[string]Output)
//	subtasks, err := t.SubtaskDao.GetByTaskID(ctx, taskID)
//	if err != nil {
//		logger.Error("[execTask] get subtasks for task %s failed. err: %v", taskID, err)
//		return
//	}
//
//	// 检查是否所有子任务都已完成（成功、失败或跳过）
//	// 防止任务在子任务全部完成前被过早标记为终态
//	allDone := true
//	failed := false
//	for _, subtask := range subtasks {
//		if subtask.State == string(TaskFailed) {
//			failed = true
//		}
//		if subtask.State != string(TaskSucceeded) && subtask.State != string(TaskFailed) && subtask.State != string(TaskSkipped) {
//			allDone = false
//		}
//		var output Output
//		_ = tools.Unmarshal([]byte(subtask.Output), &output)
//		subtaskMap[subtask.TaskName] = output
//	}
//
//	// 如果还有未完成的子任务（pending/running），不应执行 FinishedTask/FailedTask 回调
//	// 任务终态由 dispatcher 的 analysisTask 在所有子任务完成后设置
//	if !allDone {
//		logger.Trace("[execTask] task %s has unfinished subtasks (allDone=false), skip callback", taskID)
//		return
//	}
//
//	var (
//		state  string
//		output string
//	)
//
//	if !failed {
//		taskErr := executor.FinishedTask(&TaskData{
//			RequestId: task.RequestID,
//			TaskId:    task.ID,
//			Input:     task.Input,
//			Subtasks:  subtaskMap,
//		})
//		if taskErr != nil {
//			if task.Retry > 0 && !errors.Is(taskErr, ErrNonRetryable) {
//				if dErr := t.TaskDao.SetRetry(ctx, taskID, task.Retry-1); dErr != nil {
//					logger.Error("[execTask] task %s setRetry failed. err: %v", taskID, dErr)
//				}
//				return
//			}
//
//			output = tools.ToJson(&Output{Err: taskErr.Error()})
//			state = string(TaskFailed)
//		} else {
//			state = string(TaskSucceeded)
//		}
//	} else {
//		taskErr := executor.FailedTask(&TaskData{
//			RequestId: task.RequestID,
//			TaskId:    task.ID,
//			Input:     task.Input,
//			Subtasks:  subtaskMap,
//		})
//		if taskErr != nil {
//			if task.Retry > 0 && !errors.Is(taskErr, ErrNonRetryable) {
//				if dErr := t.TaskDao.SetRetry(ctx, taskID, task.Retry-1); dErr != nil {
//					logger.Error("[execTask] task %s setRetry failed. err: %v", taskID, dErr)
//				}
//				return
//			}
//
//			output = tools.ToJson(&Output{Err: taskErr.Error()})
//		}
//		state = string(TaskFailed)
//	}
//
//	err = t.TaskDao.SetOutputAndState(ctx, taskID, output, state)
//	if err != nil {
//		logger.Error("[execTask] task %s set output and state failed. err: %v", taskID, err)
//		return
//	}
//}

func (t *taskReceiver) execSubtask(task *model.Task, subtask *model.Subtask) {
	defer t.subtaskInflight.DeleteString(subtask.ID)

	logger.Trace("[execSubtask] start subtask=%s task=%s urgent=%v worker=%s myName=%s", subtask.ID, task.ID, task.Urgent, subtask.Worker, t.Cluster.GetMyName())

	golocalv1.PutTraceID(task.RequestID)
	defer golocalv1.Clean()
	ctx := golocalv1.GetContext()
	taskID := task.ID
	subtaskID := subtask.ID

	// check subtask state again
	_subtask, err := t.SubtaskDao.GetByID(ctx, subtaskID)
	if err != nil {
		logger.Error("[execSubtask] get subtask %s failed. err: %v", subtaskID, err)
		return
	}
	if _subtask == nil ||
		_subtask.Worker != t.Cluster.GetMyName() ||
		isFinished(_subtask.State) ||
		time.Now().Before(_subtask.LastRunTime.Time().Add(time.Duration(subtask.RetryInterval)*time.Second)) {
		logger.Info("[execSubtask] subtask %s not satisfy exec condition", subtaskID)
		return
	}

	// 使用 ExecutorProvider 接口（统一本地函数、gRPC、HTTP、MCP）
	provider := getProvider(task.TaskName, subtask.TaskName)
	if provider == nil {
		logger.Error("[execSubtask] subtask %s executor not found", subtaskID)
		if err = t.SubtaskDao.SetOutputAndState(ctx, subtaskID,
			tools.ToJson(&Output{Err: fmt.Sprintf("executor for task %s/%s not found", task.TaskName, subtask.TaskName)}),
			string(TaskFailed)); err != nil {
			logger.Error("[execSubtask] subtask %s set output and state failed. err: %v", subtaskID, err)
		}
		return
	}

	var (
		_output = &Output{}
		state   string
	)
	_ = tools.Unmarshal([]byte(subtask.Output), _output)
	output, err := t.exec(ctx, provider, task.TaskName, subtask.TaskName, taskID, subtask.PreSubtaskID, subtaskID, task.RequestID, subtask.Input)

	if err != nil {
		logger.Trace("[execSubtask] subtask=%s exec failed: %v, retry=%d, isNonRetryable=%v", subtaskID, err, subtask.Retry, errors.Is(err, ErrNonRetryable))
		if subtask.Retry > 0 && !errors.Is(err, ErrNonRetryable) {
			if err = t.SubtaskDao.SetRetry(ctx, subtaskID, subtask.Retry-1); err != nil {
				logger.Error("[execSubtask] subtask %s setRetry failed. err: %v", subtaskID, err)
			}
			// 通知 leader 重新调度，加快 retry 子任务的处理速度
			t.TaskDispatcher.notifyLeaderHandleTaskImmediately(ctx, taskID)
			return
		}

		_output.Err = err.Error()
		state = string(TaskFailed)
	} else {
		bytes, _ := tools.ToByte(output)
		_output.Output = string(bytes)
		state = string(TaskSucceeded)
	}

	logger.Trace("[execSubtask] subtask=%s finished, state=%s, urgent=%v", subtaskID, state, task.Urgent)

	err = t.SubtaskDao.SetOutputAndState(ctx, subtaskID, tools.ToJson(_output), state)
	if err != nil {
		logger.Error("[execSubtask] subtask %s set output and state failed. err: %v", subtaskID, err)
		return
	}

	// 无论是否 urgent，都通知 leader 重新调度，加快非 urgent 任务的处理速度
	t.TaskDispatcher.notifyLeaderHandleTaskImmediately(ctx, taskID)
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
		logger.Error("[execSubtaskRollback] get subtask %s failed. err: %v", subtaskID, err)
		return
	}
	if _subtask == nil ||
		_subtask.Worker != t.Cluster.GetMyName() ||
		isRollbackFinished(_subtask.Rollback) ||
		time.Now().Before(_subtask.LastRunTime.Time().Add(time.Duration(subtask.RetryInterval)*time.Second)) {
		logger.Info("[execSubtaskRollback] subtask %s not satisfy rollback condition", subtaskID)
		return
	}

	// 使用 ExecutorProvider 接口（回滚）
	provider := getRollbackProvider(task.TaskName, subtask.TaskName)
	if provider == nil {
		logger.Error("[execSubtaskRollback] subtask %s rollback executor not found", subtaskID)
		if err = t.SubtaskDao.SetRollbackAndState(ctx, subtaskID, string(RollbackFailed), tools.ToJson(&Output{RollbackErr: fmt.Sprintf("rollback executor for task %s/%s not found", task.TaskName, subtask.TaskName)})); err != nil {
			logger.Error("[execSubtaskRollback] subtask %s set rollback and state failed. err: %v", subtaskID, err)
		}
		return
	}

	var (
		_output  = &Output{}
		rollback string
	)
	_ = tools.Unmarshal([]byte(subtask.Output), _output)
	output, err := t.exec(ctx, provider, task.TaskName, subtask.TaskName, taskID, subtask.PreSubtaskID, subtaskID, task.RequestID, subtask.Input)

	if err != nil {
		if subtask.Retry > 0 && !errors.Is(err, ErrNonRetryable) {
			if err = t.SubtaskDao.SetRollbackAndState(ctx, subtaskID, string(RollbackPending), tools.ToJson(_output)); err != nil {
				logger.Error("[execSubtaskRollback] subtask %s setRollbackAndState failed. err: %v", subtaskID, err)
			}
			if err = t.SubtaskDao.SetRetry(ctx, subtaskID, subtask.Retry-1); err != nil {
				logger.Error("[execSubtaskRollback] subtask %s setRetry failed. err: %v", subtaskID, err)
			}
			// 通知 leader 重新调度，加快 rollback retry 的处理速度
			t.TaskDispatcher.notifyLeaderHandleTaskImmediately(ctx, taskID)
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
		logger.Error("[execSubtaskRollback] subtask %s set rollback and state failed. err: %v", subtaskID, err)
		return
	}

	// 无论是否 urgent，都通知 leader 重新调度，加快非 urgent 任务的处理速度
	t.TaskDispatcher.notifyLeaderHandleTaskImmediately(ctx, taskID)
}

func (t *taskReceiver) exec(ctx context.Context, provider executor.ExecutorProvider, taskName, subTaskName, taskID, preSubtaskID, subtaskID, requestID, input string) (any, error) {
	preSubtasks := make(map[string]Output)
	if preSubtaskID != "" {
		preSubtaskIDs := strings.Split(preSubtaskID, ",")
		preSubtaskList, err := t.SubtaskDao.GetByIDs(ctx, preSubtaskIDs)
		if err != nil {
			logger.Error("[exec] subtask %s get preSubtasks failed. err: %v", subtaskID, err)
		} else {
			for _, v := range preSubtaskList {
				var output Output
				_ = tools.Unmarshal([]byte(v.Output), &output)
				preSubtasks[v.TaskName] = output
			}
		}
	}

	// 替换策略：如果节点有前驱，用前驱的输出替换 Input
	actualInput := input
	if len(preSubtasks) > 0 {
		if len(preSubtasks) == 1 {
			// 单前驱：直接使用前驱输出作为输入
			for _, v := range preSubtasks {
				actualInput = v.Output
				break
			}
		} else {
			// 多前驱：将所有前驱输出合并为 JSON map（key 为前驱 TaskName）
			merged := make(map[string]any, len(preSubtasks))
			for k, v := range preSubtasks {
				var parsed any
				if err := tools.Unmarshal([]byte(v.Output), &parsed); err != nil {
					merged[k] = v.Output
				} else {
					merged[k] = parsed
				}
			}
			if bytes, err := tools.ToByte(merged); err == nil {
				actualInput = string(bytes)
			}
		}
	}

	taskData := &executor.TaskData{
		RequestId: requestID,
		TaskId:    taskID,
		SubTaskId: subtaskID,
		Input:     actualInput,
		Subtasks:  convertSubtasksForExecutor(preSubtasks),
	}

	// 从全局注册表获取处理器
	preProc := getPreProcessor(taskName, subTaskName)
	postProc := getPostProcessor(taskName, subTaskName)

	// 添加panic处理机制，捕获panic并返回错误
	var result any
	var execErr error

	func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Error("[exec] panic recovered in subtask %s: %v", subtaskID, r)
				execErr = fmt.Errorf("panic occurred during execution: %v", r)
			}
		}()
		// 统一执行流程：preProcessor → provider → postProcessor
		result, execErr = executeWithProcessors(ctx, provider, preProc, postProc, taskData)
	}()

	return result, execErr
}

// convertSubtasksForExecutor 将 map[string]Output 转换为 map[string]any，供 ExecutorProvider 使用
func convertSubtasksForExecutor(preSubtasks map[string]Output) map[string]any {
	result := make(map[string]any, len(preSubtasks))
	for k, v := range preSubtasks {
		result[k] = v
	}
	return result
}
