package taskx

import taskxdao "github.com/caiflower/common-tools/taskx/dao"

type TaskData struct {
	RequestId string
	TaskId    string
	SubTaskId string
	Input     string
	Subtasks  map[string]taskxdao.Subtask
}

type TaskExecutor interface {
	Name() string
	FinishedTask(data *TaskData) (retry bool, err error)
	FailedTask(data *TaskData) (retry bool, err error)
}

type SubTaskExecutor func(data *TaskData) (retry bool, output interface{}, err error)

var _em = executorManager{taskExecutor: make(map[string]TaskExecutor), subTaskExecutor: make(map[string]map[string]SubTaskExecutor)}

type executorManager struct {
	taskExecutor    map[string]TaskExecutor
	subTaskExecutor map[string]map[string]SubTaskExecutor
}

// RegisterTaskExecutor registerTaskExecutor in cluster
func RegisterTaskExecutor(taskExecutor TaskExecutor, subTaskExecutor map[string]SubTaskExecutor) {
	_em.taskExecutor[taskExecutor.Name()] = taskExecutor
	_em.subTaskExecutor[taskExecutor.Name()] = subTaskExecutor
}

func getTaskExecutor(taskName string) TaskExecutor {
	return _em.taskExecutor[taskName]
}

func getSubTaskExecutor(taskName, subTaskName string) SubTaskExecutor {
	return _em.subTaskExecutor[taskName][subTaskName]
}
