package taskx

import (
	"math/rand"
	"time"

	"github.com/caiflower/common-tools/cluster"
	dbv1 "github.com/caiflower/common-tools/db/v1"
	"github.com/caiflower/common-tools/pkg/bean"
	golocalv1 "github.com/caiflower/common-tools/pkg/golocal/v1"
	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/caiflower/common-tools/pkg/tools"
	"github.com/caiflower/common-tools/taskx/dao"
	"github.com/uptrace/bun"
)

const (
	defaultTimeout = time.Second * 3
)

var SingletonTaskDispatcher = &taskDispatcher{}

type taskDispatcher struct {
	cluster.DefaultCaller
	Cluster    cluster.ICluster `autowired:""`
	TaskDao    *dao.TaskDao     `autowired:""`
	SubTaskDao *dao.SubTaskDao  `autowired:""`
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
	bean.AddBean(&dao.Task{})
	bean.AddBean(&dao.Subtask{})
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

	return nil
}

func (t *taskDispatcher) handleTask() {
	tasks, err := t.TaskDao.GetByTaskState([]string{string(TaskPending), string(TaskRunning)})
	if err != nil {
		logger.Error("get tasks failed. err: %s", err.Error())
		return
	}
	if len(tasks) == 0 {
		return
	}

	var (
		runningTasks    []*dao.Task
		runningSubTasks []*dao.Subtask
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

		finished, retry, running := t.analysisTask(task, subtaskMap)
		if retry {
			continue
		}

		if len(running) > 0 {
			runningSubTasks = append(runningSubTasks, running...)
		} else if finished {
			runningTasks = append(runningTasks, v)
		}
	}

	t.allocateWorker(runningTasks, runningSubTasks, t.Cluster.GetAliveNodeNames(), t.Cluster.GetLostNodeNames())
}

func (t *taskDispatcher) analysisTask(task *Task, subtaskMap map[string]*dao.Subtask) (finished, retry bool, runningSubtasks []*dao.Subtask) {
	taskState := task.GetTaskState()

	if task.taskState == TaskPending || task.taskState == TaskRunning {
		nextPendingSubTasks := task.NextSubTasks()
		if len(nextPendingSubTasks) > 0 {
			if task.taskState == TaskPending {
				taskState = TaskRunning
			}

			for _, subtask := range nextPendingSubTasks {
				runningSubtasks = append(runningSubtasks, subtaskMap[subtask.GetTaskId()])
			}
		} else {
			finished = true
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

func (t *taskDispatcher) backupTask() {

}

func (t *taskDispatcher) allocateWorker(runningTasks []*dao.Task, runningSubTasks []*dao.Subtask, aliveNodes, lostNodes []string) {
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
				tx.Add(func(tx *bun.Tx) error {
					subtaskId := runningSubTask.SubtaskId
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
				tx.Add(func(tx *bun.Tx) error {
					taskId := runningTask.TaskId
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
