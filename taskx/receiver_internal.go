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
	"time"

	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/caiflower/common-tools/taskx/dao/model"
	"github.com/caiflower/common-tools/taskx/executor"
)

// Sentinel errors returned by the receiver helpers. These are stable and
// safe to match with errors.Is.
var (
	// errReceiverClosed is returned when the receiver is not (or no longer)
	// running. Callers should treat it as a transient condition and stop
	// dispatching new work to this node.
	errReceiverClosed = errors.New("taskx: receiver is closed")

	// errQueueFull is returned when the worker queue cannot accept the
	// item without blocking. The inflight token is released before this
	// error is returned, so the caller can safely re-enqueue later.
	errQueueFull = errors.New("taskx: worker queue is full")
)

// isRunning reports whether the receiver is accepting new work.
func (t *taskReceiver) isRunning() bool {
	v := t.running.Load()
	return v != nil && v.(bool)
}

// loadSubtasksAndTasks fetches the requested subtasks and the related
// parent tasks in two queries. Tasks that are not present in the DB are
// silently dropped from the result map (caller must guard against nil).
//
// The returned map is keyed by task ID and stores pointers into the
// underlying tasks slice; the slice itself is not retained.
func (t *taskReceiver) loadSubtasksAndTasks(ctx context.Context, subtaskIDs []string) ([]model.Subtask, map[string]*model.Task, error) {
	subtasks, err := t.SubtaskDao.GetByIDs(ctx, subtaskIDs)
	if err != nil {
		return nil, nil, fmt.Errorf("get subtasks by ids: %w", err)
	}
	if len(subtasks) == 0 {
		return nil, nil, nil
	}

	// Collect unique task IDs in insertion order to keep DB query stable.
	seen := make(map[string]struct{}, len(subtasks))
	taskIDs := make([]string, 0, len(subtasks))
	for _, s := range subtasks {
		if _, ok := seen[s.TaskID]; ok {
			continue
		}
		seen[s.TaskID] = struct{}{}
		taskIDs = append(taskIDs, s.TaskID)
	}

	tasks, err := t.TaskDao.GetByIDs(ctx, taskIDs)
	if err != nil {
		return nil, nil, fmt.Errorf("get tasks by ids: %w", err)
	}
	taskByID := make(map[string]*model.Task, len(tasks))
	for i := range tasks {
		taskByID[tasks[i].ID] = &tasks[i]
	}
	return subtasks, taskByID, nil
}

// enqueueSubtask claims an inflight slot and pushes the bag onto the
// appropriate worker queue. On a full queue it releases the inflight slot
// before returning errQueueFull. On a closed receiver it returns
// errReceiverClosed without claiming a slot.
func (t *taskReceiver) enqueueSubtask(bag *SubtaskBag, rollback bool) error {
	if !t.isRunning() {
		return errReceiverClosed
	}
	if bag == nil || bag.subtask == nil {
		return errors.New("taskx: nil subtask bag")
	}
	if !t.subtaskInflight.InsertString(bag.subtask.ID) {
		// Already in flight on this node; skip silently. The leader will
		// retry the dispatch after the inflight record is released.
		return nil
	}
	queue := t.subtaskQueue
	qname := "subtask"
	if rollback {
		queue = t.subtaskRollbackQueue
		qname = "subtaskRollback"
	}
	select {
	case queue <- bag:
		return nil
	default:
		t.subtaskInflight.DeleteString(bag.subtask.ID)
		logger.Warn("[enqueueSubtask] %s queue is full, subtask %s dropped", qname, bag.subtask.ID)
		return errQueueFull
	}
}

// enqueueTask claims an inflight slot and pushes the task onto the worker
// queue. Behaviour mirrors enqueueSubtask.
func (t *taskReceiver) enqueueTask(task *model.Task) error {
	if !t.isRunning() {
		return errReceiverClosed
	}
	if task == nil {
		return errors.New("taskx: nil task")
	}
	if !t.taskInflight.InsertString(task.ID) {
		return nil
	}
	select {
	case t.taskQueue <- task:
		return nil
	default:
		t.taskInflight.DeleteString(task.ID)
		logger.Warn("[enqueueTask] task queue is full, task %s dropped", task.ID)
		return errQueueFull
	}
}

// shouldSkipSubtask returns true when the subtask is not eligible to run
// on this node right now. The reason is returned for structured logging.
func (t *taskReceiver) shouldSkipSubtask(s *model.Subtask, rollback bool) (skip bool, reason string) {
	if s == nil {
		return true, "not_found"
	}
	if s.Worker != t.Cluster.GetMyName() {
		return true, fmt.Sprintf("owned by %s", s.Worker)
	}
	if rollback {
		if isRollbackFinished(s.Rollback) {
			return true, "rollback_finished"
		}
	} else if isFinished(s.State) {
		return true, "finished"
	}
	// Retry interval gate uses the dispatcher's recorded lastRunTime so a
	// task that just ran on this node cannot be picked up again until the
	// interval has elapsed.
	if s.RetryInterval > 0 && !s.LastRunTime.Time().IsZero() &&
		time.Now().Before(s.LastRunTime.Time().Add(time.Duration(s.RetryInterval)*time.Second)) {
		return true, "retry_interval_active"
	}
	return false, ""
}

// loadFreshSubtask re-fetches a subtask by ID. It logs and returns nil on
// any error so the caller can simply treat nil as "skip this subtask".
func (t *taskReceiver) loadFreshSubtask(ctx context.Context, subtaskID string) *model.Subtask {
	fresh, err := t.SubtaskDao.GetByID(ctx, subtaskID)
	if err != nil {
		logger.Error("[loadFreshSubtask] get subtask %s failed. err=%v", subtaskID, err)
		return nil
	}
	return fresh
}

// withSubtaskTimeout returns a derived context that is cancelled after
// subtask.Timeout seconds. When Timeout is 0 the original context is
// returned unchanged.
func withSubtaskTimeout(ctx context.Context, subtask *model.Subtask) (context.Context, context.CancelFunc) {
	if subtask == nil || subtask.Timeout <= 0 {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, time.Duration(subtask.Timeout)*time.Second)
}

// runExecutor runs the user-supplied ExecutorProvider under panic
// recovery and pre/post-processor chain. The traceID/subtaskID context is
// expected to be already set on ctx by the caller.
func (t *taskReceiver) runExecutor(
	ctx context.Context,
	provider executor.ExecutorProvider,
	taskName, subTaskName, taskID, subtaskID, requestID, input string,
) (result any, err error) {
	defer func() {
		if r := recover(); r != nil {
			logger.Error("[runExecutor] panic recovered in subtask %s/%s: %v", taskID, subtaskID, r)
			err = fmt.Errorf("panic occurred during execution: %v", r)
		}
	}()

	taskData := &executor.TaskData{
		RequestId: requestID,
		TaskId:    taskID,
		SubTaskId: subtaskID,
		Input:     input,
	}
	// Pre/post processors are looked up on every call. The registry is
	// already RLock'd inside getPreProcessor/getPostProcessor, so there
	// is no point in caching the lookup result.
	return executeWithProcessors(ctx, provider,
		getPreProcessor(taskName, subTaskName),
		getPostProcessor(taskName, subTaskName),
		taskData,
	)
}

func (t *taskReceiver) notifyLeader(taskID string) {
	if t.TaskDispatcher == nil {
		return
	}
	t.TaskDispatcher.notifyLeaderHandleTaskImmediately(context.Background(), taskID)
}
