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
	"sync"
	"sync/atomic"

	"github.com/caiflower/common-tools/cluster"
	golocalv1 "github.com/caiflower/common-tools/pkg/golocal/v1"
	"github.com/caiflower/common-tools/pkg/inflight"
	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/caiflower/common-tools/pkg/tools"
	"github.com/caiflower/common-tools/taskx/dao"
	"github.com/caiflower/common-tools/taskx/dao/model"
	"github.com/caiflower/common-tools/taskx/proto"
)

const (
	deliverTask            = "github.caiflower.common.taskx.deliverTask"
	deliverSubtask         = "github.caiflower.common.taskx.deliverSubtask"
	deliverSubtaskRollback = "github.caiflower.common.taskx.deliverSubtaskRollback"
)

var _tr = &taskReceiver{}

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
	if t.isRunning() {
		logger.Warn("[taskReceiver] already running, skip start")
		return nil
	}

	logger.Info("[taskReceiver] starting...")

	if err := t.RegisterGRPCService(); err != nil {
		return err
	}

	// stopChan must be created before the workers are started so that
	// Close() can interrupt their inner select{} even if Start fails later.
	t.stopChan = make(chan struct{})

	t.subtaskQueue = make(chan *SubtaskBag, t.subtaskQueueSize)
	t.taskQueue = make(chan *model.Task, t.taskQueueSize)
	t.subtaskRollbackQueue = make(chan *SubtaskBag, t.subtaskRollbackQueueSize)

	t.startSubtaskWorkers()
	t.startRollbackWorkers()
	t.running.Store(true)

	logger.Info("[taskReceiver] started successfully (subtaskWorkers=%d rollbackWorkers=%d)",
		t.subtaskWorker, t.subtaskRollbackWorker)
	return nil
}

func (t *taskReceiver) Close() {
	if !t.isRunning() {
		logger.Warn("[taskReceiver] not running, skip close")
		return
	}

	t.running.Store(false)
	close(t.stopChan)

	logger.Info("[taskReceiver] waiting for workers to finish...")
	t.wg.Wait()
	logger.Info("[taskReceiver] closed")
}

// deliverSubtask is the public entry point for handing a batch of subtask
// IDs to the local worker. The `rollback=false` variant pushes work into
// the main subtask queue.
func (t *taskReceiver) deliverSubtask(ctx context.Context, subtaskIDs []string) error {
	if len(subtaskIDs) == 0 {
		return nil
	}
	return t.handleSubtask(ctx, subtaskIDs, false)
}

// deliverSubtaskRollback is the public entry point for handing a batch of
// subtask IDs to the rollback worker.
func (t *taskReceiver) deliverSubtaskRollback(ctx context.Context, subtaskIDs []string) error {
	if len(subtaskIDs) == 0 {
		return nil
	}
	return t.handleSubtask(ctx, subtaskIDs, true)
}

// handleSubtask loads the requested subtasks + their parent tasks and
// pushes every eligible subtask onto either the main or the rollback
// worker queue. Returns the first non-recoverable error encountered.
func (t *taskReceiver) handleSubtask(ctx context.Context, subtaskIDs []string, rollback bool) error {
	subtasks, taskByID, err := t.loadSubtasksAndTasks(ctx, subtaskIDs)
	if err != nil {
		logger.Error("[handleSubtask] %v", err)
		return err
	}
	if len(subtasks) == 0 {
		return nil
	}

	for i := range subtasks {
		subtask := &subtasks[i]
		task := taskByID[subtask.TaskID]
		if task == nil {
			logger.Warn("[handleSubtask] parent task %s for subtask %s not found, skip",
				subtask.TaskID, subtask.ID)
			continue
		}

		if skip, reason := t.shouldSkipSubtask(subtask, rollback); skip {
			logger.Trace("[handleSubtask] subtask %s skipped: %s", subtask.ID, reason)
			continue
		}

		if err := t.enqueueSubtask(&SubtaskBag{subtask: subtask, task: task}, rollback); err != nil {
			return err
		}
	}
	return nil
}

// deliverTask is the public entry point for handing a batch of task IDs
// to the local task worker. The corresponding execTask path is currently
// disabled (see commented startTaskThreads above); this function is kept
// for the gRPC service contract.
func (t *taskReceiver) deliverTask(ctx context.Context, taskIDs []string) error {
	if len(taskIDs) == 0 {
		return nil
	}
	tasks, err := t.TaskDao.GetByIDs(ctx, taskIDs)
	if err != nil {
		logger.Error("[deliverTask] get tasks by IDs failed. err=%v", err)
		return err
	}
	for i := range tasks {
		task := &tasks[i]

		if task.Worker != t.Cluster.GetMyName() {
			logger.Trace("[deliverTask] task %s owned by %s, skip", task.ID, task.Worker)
			continue
		}
		if isFinished(task.State) {
			logger.Trace("[deliverTask] task %s is finished, skip", task.ID)
			continue
		}
		// enqueueTask owns the running-check, inflight tracking and the
		// queue-full backpressure. The earlier worker/state filters only
		// avoid a SQL insert when we already know the slot would be wasted.
		if err := t.enqueueTask(task); err != nil {
			return err
		}
	}
	return nil
}

// startWorkerPool launches n goroutines that drain queue until stopChan is
// closed. handle is invoked once per dequeued item; it is expected to be
// safe to call concurrently.
func (t *taskReceiver) startWorkerPool(name string, n int, queue <-chan *SubtaskBag, handle func(*SubtaskBag)) {
	runOne := func(i int) {
		defer t.wg.Done()
		logger.Trace("[%s] worker %d started", name, i)
		for {
			select {
			case <-t.stopChan:
				logger.Trace("[%s] worker %d exited (stop signal)", name, i)
				return
			case v := <-queue:
				if v == nil {
					logger.Trace("[%s] worker %d exited (queue closed)", name, i)
					return
				}
				handle(v)
			}
		}
	}
	t.wg.Add(n)
	for i := 1; i <= n; i++ {
		go runOne(i)
	}
}

func (t *taskReceiver) startSubtaskWorkers() {
	t.startWorkerPool("subtaskWorker", t.subtaskWorker, t.subtaskQueue, t.execSubtask)
}

func (t *taskReceiver) startRollbackWorkers() {
	t.startWorkerPool("subtaskRollbackWorker", t.subtaskRollbackWorker, t.subtaskRollbackQueue, t.execSubtaskRollback)
}

// execSubtask runs the user-supplied executor for a single subtask. The
// function is responsible for the full lifecycle: re-validating ownership
// in the DB, persisting the result, and re-notifying the leader so that
// downstream subtasks can be scheduled.
func (t *taskReceiver) execSubtask(bag *SubtaskBag) {
	defer t.subtaskInflight.DeleteString(bag.subtask.ID)

	golocalv1.PutTraceID(bag.task.RequestID)
	defer golocalv1.Clean()
	ctx := golocalv1.GetContext()

	subtaskID := bag.subtask.ID
	taskID := bag.task.ID

	if !t.prepareSubtaskRun(ctx, bag, false, subtaskID, taskID) {
		return
	}

	provider := getProvider(bag.task.TaskName, bag.subtask.TaskName)
	if provider == nil {
		t.handleExecutorMissing(ctx, bag.task, bag.subtask)
		return
	}

	execCtx, cancel := withSubtaskTimeout(ctx, bag.subtask)
	output, err := t.runExecutor(execCtx, provider,
		bag.task.TaskName, bag.subtask.TaskName,
		taskID, subtaskID, bag.task.RequestID, bag.subtask.Input)
	cancel()

	t.persistSubtaskOutcome(ctx, bag, output, err)
	t.notifyLeader(taskID)
}

// execSubtaskRollback mirrors execSubtask for the rollback path. The
// pre-checks differ (Rollback state machine, not State) and the failure
// path writes RollbackPending instead of SetRetry.
func (t *taskReceiver) execSubtaskRollback(bag *SubtaskBag) {
	defer t.subtaskInflight.DeleteString(bag.subtask.ID)

	golocalv1.PutTraceID(bag.task.RequestID)
	defer golocalv1.Clean()
	ctx := golocalv1.GetContext()

	subtaskID := bag.subtask.ID
	taskID := bag.task.ID

	if !t.prepareSubtaskRun(ctx, bag, true, subtaskID, taskID) {
		return
	}

	provider := getRollbackProvider(bag.task.TaskName, bag.subtask.TaskName)
	if provider == nil {
		t.handleRollbackExecutorMissing(ctx, bag.task, bag.subtask)
		return
	}

	execCtx, cancel := withSubtaskTimeout(ctx, bag.subtask)
	output, err := t.runExecutor(execCtx, provider,
		bag.task.TaskName, bag.subtask.TaskName,
		taskID, subtaskID, bag.task.RequestID, bag.subtask.Input)
	cancel()

	t.persistRollbackOutcome(ctx, bag, output, err)
	t.notifyLeader(taskID)
}

// prepareSubtaskRun performs the second-line checks (fresh state from the
// DB, ownership, retry interval) and short-circuits with a log line if
// the subtask is no longer eligible to run. Returns true if execution can
// proceed.
func (t *taskReceiver) prepareSubtaskRun(ctx context.Context, bag *SubtaskBag, rollback bool, subtaskID, taskID string) bool {
	fresh := t.loadFreshSubtask(ctx, subtaskID)
	if fresh == nil {
		return false
	}
	if skip, reason := t.shouldSkipSubtask(fresh, rollback); skip {
		logger.Info("[execSubtask] subtask %s skipped: %s", subtaskID, reason)
		return false
	}
	logger.Trace("[execSubtask] start subtask=%s task=%s urgent=%v worker=%s",
		subtaskID, taskID, bag.task.Urgent, fresh.Worker)
	return true
}

// handleExecutorMissing records an executor-not-found error against the
// subtask. Used by execSubtask only.
func (t *taskReceiver) handleExecutorMissing(ctx context.Context, task *model.Task, subtask *model.Subtask) {
	subtaskID := subtask.ID
	logger.Error("[execSubtask] subtask %s executor not found", subtaskID)
	output := &Output{Err: fmt.Sprintf("executor for task %s/%s not found", task.TaskName, subtask.TaskName)}
	if err := t.SubtaskDao.SetOutputAndState(ctx, subtaskID, tools.ToJson(output), string(TaskFailed)); err != nil {
		logger.Error("[execSubtask] subtask %s SetOutputAndState failed. err=%v", subtaskID, err)
	}
}

// handleRollbackExecutorMissing records an executor-not-found error
// against the subtask's rollback state. Used by execSubtaskRollback only.
func (t *taskReceiver) handleRollbackExecutorMissing(ctx context.Context, task *model.Task, subtask *model.Subtask) {
	subtaskID := subtask.ID
	logger.Error("[execSubtaskRollback] subtask %s rollback executor not found", subtaskID)
	output := &Output{RollbackErr: fmt.Sprintf("rollback executor for task %s/%s not found", task.TaskName, subtask.TaskName)}
	if err := t.SubtaskDao.SetRollbackAndState(ctx, subtaskID, string(RollbackFailed), tools.ToJson(output)); err != nil {
		logger.Error("[execSubtaskRollback] subtask %s SetRollbackAndState failed. err=%v", subtaskID, err)
	}
}

// persistSubtaskOutcome writes the result of a forward execution back to
// the DB. The retry/failure split mirrors the previous behaviour:
//
//   - retryable error with retry budget left: SetRetry, leave state alone,
//     notify leader
//   - non-retryable error or retry budget exhausted: write Output.Err and
//     state=TaskFailed
//   - success: write Output.Output and state=TaskSucceeded
func (t *taskReceiver) persistSubtaskOutcome(ctx context.Context, bag *SubtaskBag, output any, execErr error) {
	subtaskID := bag.subtask.ID
	taskID := bag.task.ID

	_output := &Output{}
	_ = tools.Unmarshal([]byte(bag.subtask.Output), _output)

	if execErr != nil {
		logger.Trace("[execSubtask] subtask=%s exec failed: %v retry=%d nonRetryable=%v",
			subtaskID, execErr, bag.subtask.Retry, errors.Is(execErr, ErrNonRetryable))

		if bag.subtask.Retry > 0 && !errors.Is(execErr, ErrNonRetryable) {
			if err := t.SubtaskDao.SetRetry(ctx, subtaskID, bag.subtask.Retry-1); err != nil {
				logger.Error("[execSubtask] subtask %s SetRetry failed. err=%v", subtaskID, err)
			}
			// 通知 leader 重新调度，加快 retry 子任务的处理速度
			t.notifyLeader(taskID)
			return
		}

		_output.Err = execErr.Error()
		if err := t.SubtaskDao.SetOutputAndState(ctx, subtaskID, tools.ToJson(_output), string(TaskFailed)); err != nil {
			logger.Error("[execSubtask] subtask %s SetOutputAndState failed. err=%v", subtaskID, err)
		}
		return
	}

	bytes, _ := tools.ToByte(output)
	_output.Output = string(bytes)
	logger.Trace("[execSubtask] subtask=%s finished, state=Succeeded urgent=%v", subtaskID, bag.task.Urgent)
	if err := t.SubtaskDao.SetOutputAndState(ctx, subtaskID, tools.ToJson(_output), string(TaskSucceeded)); err != nil {
		logger.Error("[execSubtask] subtask %s SetOutputAndState failed. err=%v", subtaskID, err)
	}
}

// persistRollbackOutcome is the rollback-path counterpart of
// persistSubtaskOutcome. The only meaningful difference is that on the
// retry path we record RollbackPending (not just "pending") so the
// dispatcher can distinguish a forward retry from a rollback retry.
func (t *taskReceiver) persistRollbackOutcome(ctx context.Context, bag *SubtaskBag, output any, execErr error) {
	subtaskID := bag.subtask.ID
	taskID := bag.task.ID

	_output := &Output{}
	_ = tools.Unmarshal([]byte(bag.subtask.Output), _output)

	if execErr != nil {
		if bag.subtask.Retry > 0 && !errors.Is(execErr, ErrNonRetryable) {
			if err := t.SubtaskDao.SetRollbackAndState(ctx, subtaskID, string(RollbackPending), tools.ToJson(_output)); err != nil {
				logger.Error("[execSubtaskRollback] subtask %s SetRollbackAndState failed. err=%v", subtaskID, err)
			}
			if err := t.SubtaskDao.SetRetry(ctx, subtaskID, bag.subtask.Retry-1); err != nil {
				logger.Error("[execSubtaskRollback] subtask %s SetRetry failed. err=%v", subtaskID, err)
			}
			t.notifyLeader(taskID)
			return
		}

		_output.RollbackErr = execErr.Error()
		if err := t.SubtaskDao.SetRollbackAndState(ctx, subtaskID, string(RollbackFailed), tools.ToJson(_output)); err != nil {
			logger.Error("[execSubtaskRollback] subtask %s SetRollbackAndState failed. err=%v", subtaskID, err)
		}
		return
	}

	bytes, _ := tools.ToByte(output)
	_output.RollbackOutput = string(bytes)
	logger.Trace("[execSubtaskRollback] subtask=%s rollback succeeded", subtaskID)
	if err := t.SubtaskDao.SetRollbackAndState(ctx, subtaskID, string(RollbackSucceeded), tools.ToJson(_output)); err != nil {
		logger.Error("[execSubtaskRollback] subtask %s SetRollbackAndState failed. err=%v", subtaskID, err)
	}
}
