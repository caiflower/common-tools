package taskx

import (
	"errors"
	"strings"

	"github.com/caiflower/common-tools/cluster"
	golocalv1 "github.com/caiflower/common-tools/pkg/golocal/v1"
	"github.com/caiflower/common-tools/pkg/inflight"
	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/caiflower/common-tools/pkg/tools"
	"github.com/caiflower/common-tools/taskx/dao"
)

const (
	deliverTask            = "deliverTask"
	deliverSubtask         = "deliverSubtask"
	deliverSubtaskRollback = "deliverSubtaskRollback"
	taskDoneCallBack       = "taskDoneCallBack"
)

var _tr = &taskReceiver{
	subtaskInflight: inflight.NewInFlight(),
	taskInflight:    inflight.NewInFlight(),
}

type SubtaskBag struct {
	subtask *taskxdao.Subtask
	task    *taskxdao.Task
}

type taskReceiver struct {
	Cluster        cluster.ICluster     `autowired:""`
	TaskDao        *taskxdao.TaskDao    `autowired:""`
	SubtaskDao     *taskxdao.SubtaskDao `autowired:""`
	TaskDispatcher *taskDispatcher      `autowired:""`
	cfg            *Config

	running                  bool
	subtaskWorker            int
	subtaskQueueSize         int
	subtaskInflight          *inflight.InFlight
	subtaskQueue             chan *SubtaskBag
	taskWorker               int
	taskQueueSize            int
	taskInflight             *inflight.InFlight
	taskQueue                chan *taskxdao.Task
	subtaskRollbackWorker    int
	subtaskRollbackQueueSize int
	subtaskRollbackQueue     chan *SubtaskBag
}

func (t *taskReceiver) Start() {
	if t.running {
		return
	}

	logger.Info("taskReceiver start.")
	t.subtaskQueue = make(chan *SubtaskBag, t.subtaskQueueSize)
	t.taskQueue = make(chan *taskxdao.Task, t.taskQueueSize)
	t.subtaskRollbackQueue = make(chan *SubtaskBag, t.subtaskRollbackQueueSize)

	t.startTaskThreads()
	t.startSubTaskThreads()
	t.startRollbackTaskThreads()
	t.running = true

	// register func in cluster
	t.Cluster.RegisterFunc(deliverSubtask, t.deliverSubtask)
	t.Cluster.RegisterFunc(deliverTask, t.deliverTask)
	t.Cluster.RegisterFunc(taskDoneCallBack, t.taskDoneCallBack)
	t.Cluster.RegisterFunc(deliverSubtaskRollback, t.deliverSubtaskRollback)
}

func (t *taskReceiver) Close() {
	if !t.running {
		return
	}

	t.running = false
	close(t.taskQueue)
	close(t.subtaskQueue)
	close(t.subtaskRollbackQueue)
}

func (t *taskReceiver) deliverSubtask(data interface{}) (interface{}, error) {
	var subtaskIds []string
	err := tools.Unmarshal([]byte(tools.ToJson(data)), &subtaskIds)
	if err != nil {
		return nil, err
	}

	if len(subtaskIds) == 0 {
		return nil, nil
	}

	return t.handleSubtask(subtaskIds, false)
}

func (t *taskReceiver) handleSubtask(subtaskIds []string, rollback bool) (interface{}, error) {
	subtasks, err := t.SubtaskDao.GetSubtasksBySubtaskIds(subtaskIds)

	if err != nil {
		logger.Error("get subtasks by subtaskIds failed. err: %v", err.Error())
		return nil, err
	}
	if len(subtasks) == 0 {
		return nil, nil
	}

	var taskIds []string
	taskIdMap := make(map[string]*taskxdao.Task)
	for _, subtask := range subtasks {
		if _, ok := taskIdMap[subtask.TaskId]; !ok {
			taskIds = append(taskIds, subtask.TaskId)
		}
		taskIdMap[subtask.TaskId] = &taskxdao.Task{}
	}

	tasks, err := t.TaskDao.GetTasksByTaskIds(taskIds)
	if err != nil {
		logger.Error("get task by taskIds failed. err: %v", err.Error())
		return nil, err
	}
	for _, task := range tasks {
		taskIdMap[task.TaskId] = task
	}

	for _, subtask := range subtasks {
		if subtask.Worker != t.Cluster.GetMyName() {
			logger.Warn("subtask '%s' is not my job. worker: '%s', myName: '%s'", subtask.SubtaskId, subtask.Worker, t.Cluster.GetMyName())
			continue
		}

		if rollback {
			if subtask.RollbackFinished() {
				logger.Warn("subtask '%s' already rollback", subtask.SubtaskId)
				continue
			}
		} else {
			if subtask.IsFinished() {
				logger.Warn("subtask '%s' is finished", subtask.SubtaskId)
				continue
			}
		}

		if !t.running {
			logger.Warn("task receiver is closed")
			return nil, errors.New("task receiver is closed")
		}

		if t.subtaskInflight.InFlight(subtask) {
			logger.Info("subtask '%s' is inflight, rollback is %v", subtask.SubtaskId, rollback)
			continue
		}

		if rollback {
			select {
			case t.subtaskRollbackQueue <- &SubtaskBag{
				subtask: subtask,
				task:    taskIdMap[subtask.TaskId],
			}:
			default:
				logger.Warn("subtask queue is full")
				return nil, errors.New("subtask queue is full")
			}
		} else {
			select {
			case t.subtaskQueue <- &SubtaskBag{
				subtask: subtask,
				task:    taskIdMap[subtask.TaskId],
			}:
			default:
				logger.Warn("subtask queue is full")
				return nil, errors.New("subtask queue is full")
			}
		}
	}
	return nil, nil
}

func (t *taskReceiver) deliverTask(data interface{}) (interface{}, error) {
	var taskIds []string
	err := tools.Unmarshal([]byte(tools.ToJson(data)), &taskIds)
	if len(taskIds) == 0 {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	tasks, err := t.TaskDao.GetTasksByTaskIds(taskIds)
	if err != nil {
		logger.Error("get task by taskIds failed. err: %v", err.Error())
		return nil, err
	}
	for _, task := range tasks {
		if task.Worker != t.Cluster.GetMyName() {
			logger.Warn("task '%s' is not my job. worker: '%s', myName: '%s'", task.TaskId, task.Worker, t.Cluster.GetMyName())
			continue
		}
		if task.IsFinished() {
			logger.Warn("task '%s' is finished", task.TaskId)
			continue
		}
		if t.taskInflight.InFlight(task) {
			logger.Warn("task '%s' is inflight", task.TaskId)
			continue
		}
		if !t.running {
			logger.Warn("task receiver is closed")
			return nil, errors.New("task receiver is closed")
		}
		select {
		case t.taskQueue <- task:
		default:

			logger.Warn("subtask queue is full")
			return nil, errors.New("task queue is full")
		}
	}

	return nil, nil
}

func (t *taskReceiver) deliverSubtaskRollback(data interface{}) (interface{}, error) {
	var subtaskIds []string
	err := tools.Unmarshal([]byte(tools.ToJson(data)), &subtaskIds)
	if err != nil {
		return nil, err
	}

	if len(subtaskIds) == 0 {
		return nil, nil
	}

	return t.handleSubtask(subtaskIds, true)
}

func (t *taskReceiver) startTaskThreads() {
	for i := 0; i < t.taskWorker; i++ {
		go func() {
			for v := range t.taskQueue {
				if !t.taskInflight.Insert(v) {
					logger.Warn("task %v already inflight", v.TaskId)
					continue
				}

				t.execTask(v)
				t.taskInflight.Delete(v)
			}
		}()
	}
}

func (t *taskReceiver) startSubTaskThreads() {
	for i := 0; i < t.subtaskWorker; i++ {
		go func() {
			for v := range t.subtaskQueue {
				if !t.subtaskInflight.Insert(v.subtask) {
					logger.Warn("subtask %v already inflight", v.subtask.SubtaskId)
					continue
				}

				t.execSubtask(v.task, v.subtask)
				t.subtaskInflight.Delete(v.subtask)
			}
		}()
	}
}

func (t *taskReceiver) startRollbackTaskThreads() {
	for i := 0; i < t.subtaskRollbackWorker; i++ {
		go func() {
			for v := range t.subtaskRollbackQueue {
				if !t.subtaskInflight.Insert(v.subtask) {
					logger.Warn("subtask rollback task %v already inflight", v.subtask.SubtaskId)
					continue
				}

				t.execSubtaskRollback(v.task, v.subtask)
				t.subtaskInflight.Delete(v.subtask)
			}
		}()
	}
}

func (t *taskReceiver) execTask(task *taskxdao.Task) {
	golocalv1.PutTraceID(task.RequestId)
	defer golocalv1.Clean()

	executor := getTaskExecutor(task.TaskName)
	if executor == nil {
		logger.Warn("task %v executor is not found", task.TaskId)
		return
	}

	// check task state again
	tasks, err := t.TaskDao.GetTasksByTaskIds([]string{task.TaskId})
	if err != nil {
		logger.Error("get task %v failed. err: %v", task.TaskId, err)
		return
	}
	if len(tasks) == 0 || tasks[0].Worker != t.Cluster.GetMyName() || tasks[0].IsFinished() {
		logger.Warn("task %v is not satisfy exec condition", tasks[0].TaskId)
		return
	}
	task = tasks[0]

	subtaskMap := make(map[string]taskxdao.Output)
	subtasks, _, err := t.SubtaskDao.GetSubtasksByTaskId(task.TaskId)
	if err != nil {
		logger.Error("get task %v subtasks failed. err: %v", task.TaskId, err)
		return
	}

	failed := false
	for _, subtask := range subtasks {
		if subtask.TaskState == string(TaskFailed) {
			failed = true
		}
		var output taskxdao.Output
		_ = tools.Unmarshal([]byte(subtask.Output), &output)
		subtaskMap[subtask.TaskName] = output
	}

	if !failed {
		retry, err := executor.FinishedTask(&TaskData{
			RequestId: task.RequestId,
			TaskId:    task.TaskId,
			Input:     task.Input,
			Subtasks:  subtaskMap,
		})
		if retry {
			// retry
			return
		} else if err != nil {
			if task.Retry > 0 {
				if err = t.TaskDao.SetRetry(task.TaskId, task.Retry-1, nil); err != nil {
					logger.Error("task %v setRetry failed. err: %v", task.TaskId, err)
				}
				return
			} else {
				task.Output = tools.ToJson(taskxdao.Output{
					Err: err.Error(),
					Msg: "retry exceed the count",
				})
				task.TaskState = string(TaskFailed)
			}
		} else {
			task.TaskState = string(TaskSucceeded)
		}
	} else {
		retry, err := executor.FailedTask(&TaskData{
			RequestId: task.RequestId,
			TaskId:    task.TaskId,
			Input:     task.Input,
			Subtasks:  subtaskMap,
		})
		if retry {
			return
		} else if err != nil {
			if task.Retry > 0 {
				if err = t.TaskDao.SetRetry(task.TaskId, task.Retry-1, nil); err != nil {
					logger.Error("task %v setRetry failed. err: %v", task.TaskId, err)
				}
				return
			} else {
				task.Output = tools.ToJson(taskxdao.Output{
					Err: err.Error(),
					Msg: "retry exceed the count",
				})
			}
		}
		task.TaskState = string(TaskFailed)
	}

	err = t.TaskDao.SetOutputAndTaskState(task.TaskId, task.Output, task.TaskState, nil)
	if err != nil {
		logger.Error("task %v setOutputAndTaskState failed. error: %s", task.TaskId, err.Error())
		return
	}

	//if task.Urgent {
	//	_, err = t.Cluster.CallFunc(cluster.NewAsyncFuncSpec(t.Cluster.GetLeaderName(), taskDoneCallBack, task.TaskId, defaultTimeout))
	//	if err != nil {
	//		logger.Warn("task %v remote call 'taskDoneCallBack' failed. err: %v", task.TaskId, err)
	//	}
	//}
}

func (t *taskReceiver) execSubtask(task *taskxdao.Task, subtask *taskxdao.Subtask) {
	golocalv1.PutTraceID(task.RequestId)
	defer golocalv1.Clean()

	// check subtask state again
	subtasks, err := t.SubtaskDao.GetSubtasksBySubtaskIds([]string{subtask.SubtaskId})
	if err != nil {
		return
	}
	if len(subtasks) == 0 || subtasks[0].Worker != t.Cluster.GetMyName() || subtasks[0].IsFinished() {
		logger.Warn("subtask '%s' is not satisfy exec condition", subtask.SubtaskId)
		return
	}
	subtask = subtasks[0]

	executor := getSubTaskExecutor(task.TaskName, subtask.TaskName)
	if executor == nil {
		logger.Warn("subtask '%s' executor is not found", subtask.SubtaskId)
		return
	}

	preSubtasks := make(map[string]taskxdao.Output)
	if subtask.PreSubtaskId != "" {
		preSubtaskIds := strings.Split(subtask.PreSubtaskId, ",")
		preSubtaskList, err := t.SubtaskDao.GetSubtasksBySubtaskIds(preSubtaskIds)
		if err != nil {
			logger.Error("subtask '%s' get preSubtasks failed. err: %v", subtask.SubtaskId, err)
			return
		}
		for _, v := range preSubtaskList {
			var output taskxdao.Output
			_ = tools.Unmarshal([]byte(subtask.Output), &output)
			preSubtasks[v.TaskName] = output
		}
	}

	retry, output, err := executor(&TaskData{RequestId: task.RequestId, TaskId: subtask.TaskId, SubTaskId: subtask.SubtaskId, Input: subtask.Input, Subtasks: preSubtasks})
	if retry {
		// retry
		return
	} else if err != nil {
		if subtask.Retry > 0 {
			if err = t.SubtaskDao.SetRetry(subtask.SubtaskId, subtask.Retry-1, nil); err != nil {
				logger.Error("subtask %v setRetry failed. err: %v", subtask.SubtaskId, err)
			}
			return
		}

		subtask.Output = tools.ToJson(taskxdao.Output{
			Output: tools.ToJson(output),
			Err:    err.Error(),
			Msg:    "retry exceed the count",
		})
		subtask.TaskState = string(TaskFailed)
	} else if output != nil {
		subtask.Output = tools.ToJson(taskxdao.Output{
			Output: tools.ToJson(output),
		})
		subtask.TaskState = string(TaskSucceeded)
	}

	_, err = t.SubtaskDao.SetOutputAndTaskState(subtask.SubtaskId, subtask.Output, subtask.TaskState, nil)
	if err != nil {
		logger.Error("subtask %v setOutputAndTaskState failed. error: %s", subtask.SubtaskId, err.Error())
		return
	}

	if task.Urgent {
		_, err = t.Cluster.CallFunc(cluster.NewAsyncFuncSpec(t.Cluster.GetLeaderName(), taskDoneCallBack, task.TaskId, t.cfg.RemoteCallTimout).SetTraceId(golocalv1.GetTraceID()))
		if err != nil {
			logger.Warn("task %v remote call 'taskDoneCallBack' failed. err: %v", task.TaskId, err)
		}
	}
}

func (t *taskReceiver) execSubtaskRollback(task *taskxdao.Task, subtask *taskxdao.Subtask) {
	golocalv1.PutTraceID(task.RequestId)
	defer golocalv1.Clean()

	// check subtask state again
	subtasks, err := t.SubtaskDao.GetSubtasksBySubtaskIds([]string{subtask.SubtaskId})
	if err != nil {
		return
	}
	if len(subtasks) == 0 || subtasks[0].Worker != t.Cluster.GetMyName() || subtasks[0].RollbackFinished() {
		logger.Warn("subtask '%s' is not satisfy rollback condition", subtask.SubtaskId)
		return
	}
	subtask = subtasks[0]

	executor := getRollbackTaskExecutor(task.TaskName, subtask.TaskName)
	if executor == nil {
		logger.Warn("subtask '%s' executor is not found", subtask.SubtaskId)
		return
	}

	preSubtasks := make(map[string]taskxdao.Output)
	if subtask.PreSubtaskId != "" {
		preSubtaskIds := strings.Split(subtask.PreSubtaskId, ",")
		preSubtaskList, err := t.SubtaskDao.GetSubtasksBySubtaskIds(preSubtaskIds)
		if err != nil {
			logger.Error("subtask '%s' get preSubtasks failed. err: %v", subtask.SubtaskId, err)
			return
		}
		for _, v := range preSubtaskList {
			var output taskxdao.Output
			_ = tools.Unmarshal([]byte(subtask.Output), &output)
			preSubtasks[v.TaskName] = output
		}
	}

	retry, output, err := executor(&TaskData{RequestId: task.RequestId, TaskId: subtask.TaskId, SubTaskId: subtask.SubtaskId, Input: subtask.Input, Subtasks: preSubtasks})
	if retry {
		// retry
		return
	} else {
		_output := &taskxdao.Output{}
		err = tools.Unmarshal([]byte(subtask.Output), _output)
		if err != nil {
			logger.Warn("unmarshal output failed. err: %v", err)
		}

		if err != nil {
			if subtask.Retry > 0 {
				if err = t.SubtaskDao.SetRetry(subtask.SubtaskId, subtask.Retry-1, nil); err != nil {
					logger.Error("subtask %v setRetry failed. err: %v", subtask.SubtaskId, err)
				}
				return
			}

			_output.RollbackMsg = "retry exceed the count"
			_output.RollbackErr = err.Error()
			subtask.Output = tools.ToJson(_output)
			subtask.Rollback = string(RollbackFailed)
		} else {
			_output.RollbackOutput = tools.ToJson(output)
			subtask.Output = tools.ToJson(_output)
			subtask.Rollback = string(RollbackSucceeded)
		}
	}

	_, err = t.SubtaskDao.SetRollbackAndTaskState(subtask.SubtaskId, subtask.Output, subtask.Rollback, nil)
	if err != nil {
		logger.Error("subtask %v setOutputAndTaskState failed. error: %s", subtask.SubtaskId, err.Error())
		return
	}

	if task.Urgent {
		_, err = t.Cluster.CallFunc(cluster.NewAsyncFuncSpec(t.Cluster.GetLeaderName(), taskDoneCallBack, task.TaskId, t.cfg.RemoteCallTimout).SetTraceId(golocalv1.GetTraceID()))
		if err != nil {
			logger.Warn("task %v remote call 'taskDoneCallBack' failed. err: %v", task.TaskId, err)
		}
	}
}

func (t *taskReceiver) taskDoneCallBack(data interface{}) (interface{}, error) {
	logger.Debug("[taskReceiver] task %v taskDoneCallBack", data)
	t.TaskDispatcher.handleTaskImmediately(data.(string))
	return nil, nil
}
