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
	"fmt"

	taskxdao "github.com/caiflower/common-tools/taskx/dao"
)

type TaskData struct {
	RequestId string
	TaskId    string
	SubTaskId string
	Input     string
	Subtasks  map[string]taskxdao.Output
}

type TaskExecutor interface {
	Name() string
	FinishedTask(data *TaskData) (retry bool, err error)
	FailedTask(data *TaskData) (retry bool, err error)
}

type SubTaskExecutor func(data *TaskData) (retry bool, output interface{}, err error)

var _em = executorManager{taskExecutor: make(map[string]TaskExecutor), subtaskExecutor: make(map[string]map[string]SubTaskExecutor), rollbackTaskExecutor: make(map[string]SubTaskExecutor)}

type executorManager struct {
	taskExecutor         map[string]TaskExecutor
	subtaskExecutor      map[string]map[string]SubTaskExecutor
	rollbackTaskExecutor map[string]SubTaskExecutor
}

// RegisterTaskExecutor registerTaskExecutor in cluster
func RegisterTaskExecutor(taskExecutor TaskExecutor, subTaskExecutor map[string]SubTaskExecutor) {
	RegisterTaskExecutorWithRollback(taskExecutor, subTaskExecutor, nil)
}

func RegisterTaskExecutorWithRollback(taskExecutor TaskExecutor, subtaskExecutor map[string]SubTaskExecutor, subtaskRollbackExecutor map[string]SubTaskExecutor) {
	if _, ok := _em.taskExecutor[taskExecutor.Name()]; ok {
		panic(fmt.Sprintf("task executor '%s' exist.", taskExecutor.Name()))
	}
	_em.taskExecutor[taskExecutor.Name()] = taskExecutor
	_em.subtaskExecutor[taskExecutor.Name()] = subtaskExecutor
	for k, v := range subtaskRollbackExecutor {
		if _, ok := _em.subtaskExecutor[taskExecutor.Name()][k]; !ok {
			panic(fmt.Sprintf("subtask executor '%s' not exist.", k))
		}
		registerRollbackTaskExecutor(taskExecutor.Name(), k, v)
	}
}

// registerRollbackTaskExecutor register subtask rollbackExecutor
func registerRollbackTaskExecutor(taskName, subtaskName string, subTaskExecutor SubTaskExecutor) {
	_em.rollbackTaskExecutor[taskName+"/"+subtaskName] = subTaskExecutor
}

func getTaskExecutor(taskName string) TaskExecutor {
	return _em.taskExecutor[taskName]
}

func getSubTaskExecutor(taskName, subTaskName string) SubTaskExecutor {
	return _em.subtaskExecutor[taskName][subTaskName]
}

func getRollbackTaskExecutor(taskName, subtaskName string) SubTaskExecutor {
	return _em.rollbackTaskExecutor[taskName+"/"+subtaskName]
}
