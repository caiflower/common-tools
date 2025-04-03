package taskx

import (
	"math/rand"
	"time"

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
	defaultTimeout = time.Second * 3

	taskIdKey = "common-tools/taskx/taskId"
)

var SingletonTaskDispatcher = &taskDispatcher{}

type taskDispatcher struct {
	cluster.DefaultCaller
	Cluster    cluster.ICluster     `autowired:""`
	TaskDao    *taskxdao.TaskDao    `autowired:""`
	SubTaskDao *taskxdao.SubTaskDao `autowired:""`
	DBClient   dbv1.IDB             `autowired:""`
	running    bool
}

func InitTaskDispatcher(taskWorker, subtaskWorker, taskQueueSize, subtaskQueueSize int) {
	if taskWorker <= 0 {
		taskWorker = 200
	}
	if subtaskWorker <= 0 {
		subtaskWorker = 500
	}
	if taskQueueSize <= 0 {
		taskQueueSize = 1000
	}
	if subtaskQueueSize <= 0 {
		subtaskQueueSize = 1000
	}

	_tr.subtaskWorker = subtaskWorker
	_tr.taskWorker = taskWorker
	_tr.subtaskQueueSize = subtaskQueueSize
	_tr.taskQueueSize = taskQueueSize
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

func (t *taskDispatcher) SubmitTask(task *Task) error {
	tx := dbv1.NewBatchTx(t.TaskDao.GetDB())
	taskBean, subtaskBeans := task.convert2Bean()
	tx.Add(func(tx *bun.Tx) error {
		_, err := t.TaskDao.Insert(taskBean, tx)
		return err
	})

	tx.Add(func(tx *bun.Tx) error {
		_, err := t.SubTaskDao.Insert(&subtaskBeans, tx)
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

func (t *taskDispatcher) handleTask() {
	id := 0
	if tmp, e := cache.LocalCache.Get(taskIdKey); e {
		id = tmp.(int)
	}

	tasks, err := t.TaskDao.GetByTaskState([]string{string(TaskPending), string(TaskRunning)}, id)
	if err != nil {
		logger.Error("get tasks failed. err: %s", err.Error())
		return
	}
	if len(tasks) == 0 {
		return
	}
	cache.LocalCache.Set(taskIdKey, tasks[0].Id, 0)

	var (
		runningTasks    []*taskxdao.Task
		runningSubTasks []*taskxdao.Subtask
	)
	for _, v := range tasks {
		subtasks, subtaskMap, err := t.SubTaskDao.GetSubTasksByTaskId(v.TaskId)
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

		finished, retry, running := t.analysisTask(task, v, subtaskMap)
		if retry {
			continue
		} else if len(running) > 0 {
			runningSubTasks = append(runningSubTasks, running...)
		} else if finished {
			runningTasks = append(runningTasks, v)
		}
	}

	t.allocateWorker(runningTasks, runningSubTasks, t.Cluster.GetAliveNodeNames(), t.Cluster.GetLostNodeNames())
}

func (t *taskDispatcher) analysisTask(task *Task, taskFromDB *taskxdao.Task, subtaskMap map[string]*taskxdao.Subtask) (finished, retry bool, runningSubtasks []*taskxdao.Subtask) {
	taskState := task.GetTaskState()

	if task.taskState == TaskPending || task.taskState == TaskRunning {
		nextPendingSubTasks := task.NextSubTasks()
		if len(nextPendingSubTasks) > 0 {
			if task.taskState == TaskPending {
				taskState = TaskRunning
			}

			for _, subtask := range nextPendingSubTasks {
				runningSubtask := subtaskMap[subtask.GetTaskId()]
				if runningSubtask.TaskState == string(TaskPending) ||
					time.Now().Add(time.Duration(runningSubtask.RetryInterval)*time.Second).After(runningSubtask.UpdateTime.Time()) {
					runningSubtasks = append(runningSubtasks, runningSubtask)
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

func (t *taskDispatcher) allocateWorker(runningTasks []*taskxdao.Task, runningSubTasks []*taskxdao.Subtask, aliveNodes, lostNodes []string) {
	if len(runningTasks) == 0 && len(runningSubTasks) == 0 {
		return
	}

	subTaskWorkerMap := make(map[string][]string)
	taskWorkerMap := make(map[string][]string)
	tx := dbv1.NewBatchTx(t.TaskDao.GetDB())

	if len(runningSubTasks) > 0 {
		for _, runningSubTask := range runningSubTasks {
			var nodeName string
			if runningSubTask.TaskState == string(TaskRunning) && !tools.StringSliceContains(lostNodes, runningSubTask.Worker) {
				nodeName = runningSubTask.Worker
			} else {
				nodeName = aliveNodes[rand.Intn(len(aliveNodes))]
			}

			if nodeName != runningSubTask.Worker {
				subtaskId := runningSubTask.SubtaskId
				tx.Add(func(tx *bun.Tx) error {
					return t.SubTaskDao.SetWorkerAndTaskState(subtaskId, nodeName, string(TaskRunning), tx)
				})
			}

			subTaskWorkerMap[nodeName] = append(subTaskWorkerMap[nodeName], runningSubTask.SubtaskId)
		}
	}

	if len(runningTasks) > 0 {
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
	}

	if err := tx.Submit(); err != nil {
		logger.Error("allocateWorker for task failed. err: %s", err.Error())
		return
	}

	for nodeName, taskIds := range subTaskWorkerMap {
		_, err := t.Cluster.CallFunc(cluster.NewAsyncFuncSpec(nodeName, deliverSubTask, taskIds, defaultTimeout).SetTraceId(golocalv1.GetTraceID()))
		if err != nil {
			logger.Error("deliver subTasks failed. err: %s", err.Error())
		}
	}

	for nodeName, taskIds := range taskWorkerMap {
		_, err := t.Cluster.CallFunc(cluster.NewAsyncFuncSpec(nodeName, deliverTask, taskIds, defaultTimeout).SetTraceId(golocalv1.GetTraceID()))
		if err != nil {
			logger.Error("deliver subTasks failed. err: %s", err.Error())
		}
	}
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
	subtasks, subtaskMap, err := t.SubTaskDao.GetSubTasksByTaskId(taskId)
	if err != nil {
		return
	}

	task := &Task{}
	task, err = task.initByBean(tasks[0], subtasks)
	if err != nil {
		logger.Error("task %v initByBean failed. err: %v", taskId, err)
		return
	}

	finished, retry, runningSubtasks := t.analysisTask(task, tasks[0], subtaskMap)
	if retry {
		return
	} else if finished {
		t.allocateWorker(tasks, nil, t.Cluster.GetAliveNodeNames(), t.Cluster.GetLostNodeNames())
	} else if len(runningSubtasks) > 0 {
		t.allocateWorker(nil, runningSubtasks, t.Cluster.GetAliveNodeNames(), t.Cluster.GetLostNodeNames())
	}
}
