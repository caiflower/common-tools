package taskx

import (
	"errors"

	"github.com/caiflower/common-tools/cluster"
	golocalv1 "github.com/caiflower/common-tools/pkg/golocal/v1"
	"github.com/caiflower/common-tools/pkg/inflight"
	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/caiflower/common-tools/pkg/tools"
	"github.com/caiflower/common-tools/taskx/dao"
)

const (
	deliverTask      = "deliverTask"
	deliverSubTask   = "deliverSubTask"
	taskDoneCallBack = "taskDoneCallBack"
)

var _tr = &taskReceiver{
	subtaskInflight: inflight.NewInFlight(),
	taskInflight:    inflight.NewInFlight(),
}

type SubTaskBag struct {
	subTask *dao.Subtask
	task    *dao.Task
}

type taskReceiver struct {
	Cluster        cluster.ICluster `autowired:""`
	TaskDao        *dao.TaskDao     `autowired:""`
	SubTaskDao     *dao.SubTaskDao  `autowired:""`
	TaskDispatcher *taskDispatcher  `autowired:""`

	closed           bool
	subtaskWorker    int
	subtaskQueueSize int
	subtaskInflight  *inflight.InFlight
	subTaskQueue     chan *SubTaskBag
	taskWorker       int
	taskQueueSize    int
	taskInflight     *inflight.InFlight
	taskQueue        chan *dao.Task
}

func (t *taskReceiver) Start() {
	logger.Info("taskReceiver start.")
	t.subTaskQueue = make(chan *SubTaskBag, t.subtaskQueueSize)
	t.taskQueue = make(chan *dao.Task, t.taskQueueSize)

	t.startTaskThreads()
	t.startSubTaskThreads()
	t.closed = false

	// register func in cluster
	t.Cluster.RegisterFunc(deliverSubTask, t.deliverSubTask)
	t.Cluster.RegisterFunc(deliverTask, t.deliverTask)
	t.Cluster.RegisterFunc(taskDoneCallBack, t.taskDoneCallBack)
}

func (t *taskReceiver) Close() {
	t.closed = true
	close(t.taskQueue)
	close(t.subTaskQueue)
}

func (t *taskReceiver) deliverSubTask(data interface{}) (interface{}, error) {
	var subtaskIds []string
	err := tools.Unmarshal([]byte(tools.ToJson(data)), &subtaskIds)
	if err != nil {
		return nil, err
	}

	if len(subtaskIds) == 0 {
		return nil, nil
	}
	subtasks, err := t.SubTaskDao.GetSubtasksBySubtaskIds(subtaskIds)
	if err != nil {
		logger.Error("get subtasks by subtaskIds failed. err: %v", err.Error())
		return nil, err
	}
	if len(subtasks) == 0 {
		return nil, nil
	}

	var taskIds []string
	taskIdMap := make(map[string]*dao.Task)
	for _, subtask := range subtasks {
		if _, ok := taskIdMap[subtask.TaskId]; !ok {
			taskIds = append(taskIds, subtask.TaskId)
		}
		taskIdMap[subtask.TaskId] = &dao.Task{}
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
			logger.Warn("subtask %v is not my job. worker: %v, myName: %v", subtask.SubtaskId, subtask.Worker, t.Cluster.GetMyName())
			continue
		}
		if subtask.IsFinished() {
			logger.Warn("subtask %v is finished", subtask.SubtaskId)
			continue
		}
		if t.closed {
			logger.Warn("task receiver is closed")
			return nil, errors.New("task receiver is closed")
		}
		select {
		case t.subTaskQueue <- &SubTaskBag{
			subTask: subtask,
			task:    taskIdMap[subtask.TaskId],
		}:
		default:
			logger.Warn("subtask queue is full")
			return nil, errors.New("subtask queue is full")
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
			logger.Warn("task %v is not my job. worker: %v, myName: %v", task.TaskId, task.Worker, t.Cluster.GetMyName())
			continue
		}
		if task.IsFinished() {
			logger.Warn("task %v is finished", task.TaskId)
			continue
		}
		if t.closed {
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
			for v := range t.subTaskQueue {
				if !t.subtaskInflight.Insert(v.subTask) {
					logger.Warn("subtask %v already inflight", v.subTask.SubtaskId)
					continue
				}

				t.execSubTask(v.task, v.subTask)
				t.subtaskInflight.Delete(v.subTask)
			}
		}()
	}
}

func (t *taskReceiver) execTask(task *dao.Task) {
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

	subtasks, _, err := t.SubTaskDao.GetSubTasksByTaskId(task.TaskId)
	if err != nil {
		logger.Error("get task %v subtasks failed. err: %v", task.TaskId, err)
		return
	}

	failed := false
	for _, subtask := range subtasks {
		if subtask.TaskState == string(TaskFailed) {
			failed = true
			break
		}
	}

	if !failed {
		retry, err := executor.FinishedTask(&TaskData{
			RequestId: task.RequestId,
			TaskId:    task.TaskId,
			Input:     task.Input,
		})
		if retry {
			if task.Retry > 0 {
				if err = t.SubTaskDao.SetRetry(task.TaskId, task.Retry-1, nil); err != nil {
					logger.Error("task %v setRetry failed. err: %v", task.TaskId, err)
				}
				return
			} else {
				task.Output = "retry exceed the count"
				if err != nil {
					task.Output = ". err=" + err.Error()
				}
				task.TaskState = string(TaskFailed)
			}
		} else if err != nil {
			task.Output = err.Error()
			task.TaskState = string(TaskFailed)
		} else {
			task.TaskState = string(TaskSucceeded)
		}
	} else {
		retry, err := executor.FailedTask(&TaskData{
			RequestId: task.RequestId,
			TaskId:    task.TaskId,
			Input:     task.Input,
		})
		if retry {
			if task.Retry > 0 {
				if err = t.SubTaskDao.SetRetry(task.TaskId, task.Retry-1, nil); err != nil {
					logger.Error("task %v setRetry failed. err: %v", task.TaskId, err)
				}
				return
			} else {
				task.Output = "retry exceed the count"
				if err != nil {
					task.Output = ". err=" + err.Error()
				}
			}
		} else if err != nil {
			task.Output = err.Error()
		}
		task.TaskState = string(TaskFailed)
	}

	err = t.TaskDao.SetOutputAndTaskState(task.TaskId, task.Output, task.TaskState, nil)
	if err != nil {
		logger.Error("task %v setOutputAndTaskState failed. error: %s", task.TaskId, err.Error())
		return
	}

	if task.Urgent {
		_, err = t.Cluster.CallFunc(cluster.NewAsyncFuncSpec(t.Cluster.GetLeaderName(), taskDoneCallBack, task.TaskId, defaultTimeout))
		if err != nil {
			logger.Warn("task %v remote call 'taskDoneCallBack' failed. err: %v", task.TaskId, err)
		}
	}
}

func (t *taskReceiver) execSubTask(task *dao.Task, subtask *dao.Subtask) {
	golocalv1.PutTraceID(task.RequestId)
	defer golocalv1.Clean()

	// check subtask state again
	subtasks, err := t.SubTaskDao.GetSubtasksBySubtaskIds([]string{subtask.SubtaskId})
	if err != nil {
		return
	}
	if len(subtasks) == 0 || subtasks[0].Worker != t.Cluster.GetMyName() || subtasks[0].IsFinished() {
		logger.Warn("subtask %v is not satisfy exec condition", subtask.SubtaskId)
		return
	}
	subtask = subtasks[0]

	executor := getSubTaskExecutor(task.TaskName, subtask.TaskName)
	if executor == nil {
		logger.Warn("subtask %v executor is not found", subtask.SubtaskId)
		return
	}
	retry, output, err := executor(&TaskData{RequestId: task.RequestId, TaskId: subtask.TaskId, SubTaskId: subtask.SubtaskId, Input: subtask.Input})
	if retry {
		if subtask.Retry > 0 {
			if err = t.SubTaskDao.SetRetry(subtask.SubtaskId, subtask.Retry-1, nil); err != nil {
				logger.Error("subtask %v setRetry failed. err: %v", subtask.SubtaskId, err)
			}
			return
		} else {
			subtask.Output = "retry exceed the count"
			if output != "" {
				_tmpBytes, _ := tools.ToByte(output)
				subtask.Output += ". output=" + string(_tmpBytes)
			}
			if err != nil {
				subtask.Output = ". err=" + err.Error()
			}
			subtask.TaskState = string(TaskFailed)
		}
	} else if output != nil {
		_tmpBytes, _ := tools.ToByte(output)
		subtask.Output = string(_tmpBytes)
		subtask.TaskState = string(TaskSucceeded)
	} else if err != nil {
		subtask.Output = err.Error()
		subtask.TaskState = string(TaskFailed)
	}

	_, err = t.SubTaskDao.SetOutputAndTaskState(subtask.SubtaskId, subtask.Output, subtask.TaskState, nil)
	if err != nil {
		logger.Error("subtask %v setOutputAndTaskState failed. error: %s", subtask.SubtaskId, err.Error())
		return
	}

	if task.Urgent {
		_, err = t.Cluster.CallFunc(cluster.NewAsyncFuncSpec(t.Cluster.GetLeaderName(), taskDoneCallBack, task.TaskId, defaultTimeout))
		if err != nil {
			logger.Warn("task %v remote call 'taskDoneCallBack' failed. err: %v", task.TaskId, err)
		}
	}
}

func (t *taskReceiver) taskDoneCallBack(data interface{}) (interface{}, error) {
	logger.Debug("[taskReceiver] task %v taskDoneCallBack", data)
	taskId := data.(string)
	tasks, err := t.TaskDao.GetTasksByTaskIds([]string{taskId})
	if err != nil {
		logger.Error("task %v getTasksByTaskIds failed. err: %v", taskId, err)
		return nil, err
	}
	if len(tasks) == 0 {
		return nil, nil
	}
	subtasks, subtaskMap, err := t.SubTaskDao.GetSubTasksByTaskId(taskId)
	if err != nil {
		return nil, err
	}

	task := &Task{}
	task, err = task.initByBean(tasks[0], subtasks)
	if err != nil {
		logger.Error("task %v initByBean failed. err: %v", taskId, err)
		return nil, err
	}

	finished, retry, runningSubtasks := t.TaskDispatcher.analysisTask(task, subtaskMap)
	if retry {
		return nil, nil
	} else if finished {
		t.TaskDispatcher.allocateWorker(tasks, nil, t.Cluster.GetAliveNodeNames(), t.Cluster.GetLostNodeNames())
	} else if len(runningSubtasks) > 0 {
		t.TaskDispatcher.allocateWorker(nil, runningSubtasks, t.Cluster.GetAliveNodeNames(), t.Cluster.GetLostNodeNames())
	}

	return nil, nil
}
