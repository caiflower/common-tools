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
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/caiflower/common-tools/cluster"
	dbv1 "github.com/caiflower/common-tools/db/v1"
	"github.com/caiflower/common-tools/pkg/inflight"
	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/caiflower/common-tools/taskx/dao"
	"github.com/stretchr/testify/assert"
)

func commonCluster() (cluster1, cluster2, cluster3 *cluster.Cluster) {
	c1 := cluster.Config{Enable: "true"}
	c2 := cluster.Config{Enable: "true"}
	c3 := cluster.Config{Enable: "true"}

	c1.Nodes = append(c1.Nodes,
		&struct {
			Name  string
			Ip    string
			Port  int
			Local bool
		}{
			Ip:   "127.0.0.1",
			Name: "localhost1",
			Port: 8080,
		},
		&struct {
			Name  string
			Ip    string
			Port  int
			Local bool
		}{
			Ip:   "127.0.0.1",
			Name: "localhost2",
			Port: 8081,
		}, &struct {
			Name  string
			Ip    string
			Port  int
			Local bool
		}{
			Ip:   "127.0.0.1",
			Name: "localhost3",
			Port: 8082,
		})

	c2.Nodes = append(c2.Nodes,
		&struct {
			Name  string
			Ip    string
			Port  int
			Local bool
		}{
			Ip:   "127.0.0.1",
			Name: "localhost1",
			Port: 8080,
		},
		&struct {
			Name  string
			Ip    string
			Port  int
			Local bool
		}{
			Ip:   "127.0.0.1",
			Name: "localhost2",
			Port: 8081,
		}, &struct {
			Name  string
			Ip    string
			Port  int
			Local bool
		}{
			Ip:   "127.0.0.1",
			Name: "localhost3",
			Port: 8082,
		})

	c3.Nodes = append(c3.Nodes,
		&struct {
			Name  string
			Ip    string
			Port  int
			Local bool
		}{
			Ip:   "127.0.0.1",
			Name: "localhost1",
			Port: 8080,
		},
		&struct {
			Name  string
			Ip    string
			Port  int
			Local bool
		}{
			Ip:   "127.0.0.1",
			Name: "localhost2",
			Port: 8081,
		}, &struct {
			Name  string
			Ip    string
			Port  int
			Local bool
		}{
			Ip:   "127.0.0.1",
			Name: "localhost3",
			Port: 8082,
		})

	c1.Nodes[0].Local = true
	cluster1, err := cluster.NewClusterWithArgs(c1, logger.NewLogger(&logger.Config{}))
	if err != nil {
		panic(err)
	}

	c2.Nodes[1].Local = true
	cluster2, err = cluster.NewClusterWithArgs(c2, logger.NewLogger(&logger.Config{}))
	if err != nil {
		panic(err)
	}

	c3.Nodes[2].Local = true
	cluster3, err = cluster.NewClusterWithArgs(c3, logger.NewLogger(&logger.Config{}))
	if err != nil {
		panic(err)
	}

	fmt.Printf("clusterName: %s term:%d leader: %s isready: %v\n", cluster1.GetMyName(), cluster1.GetMyTerm(), cluster1.GetLeaderName(), cluster1.IsReady())
	fmt.Printf("clusterName: %s term:%d leader: %s isready: %v\n", cluster2.GetMyName(), cluster1.GetMyTerm(), cluster2.GetLeaderName(), cluster2.IsReady())
	fmt.Printf("clusterName: %s term:%d leader: %s isready: %v\n", cluster3.GetMyName(), cluster1.GetMyTerm(), cluster3.GetLeaderName(), cluster3.IsReady())
	return cluster1, cluster2, cluster3
}

const (
	taskName               = "taskDemo"
	taskRollbackName       = "taskRollbackDemo"
	taskNameOfNonRetryable = "NonRetryable"

	stepOne   = "stepOne"
	stepTwo   = "stepTwo"
	stepThree = "stepThree"
	stepFour  = "stepFour"
	stepFive  = "stepFive"
)

type TaskDemo struct {
}

func (t *TaskDemo) Name() string {
	return taskName
}

func (t *TaskDemo) FinishedTask(data *TaskData) (retry bool, err error) {
	return false, nil
}
func (t *TaskDemo) FailedTask(data *TaskData) (retry bool, err error) {
	return false, nil
}

func (t *TaskDemo) GetExecutor() (TaskExecutor, map[string]SubTaskExecutor) {
	return t, map[string]SubTaskExecutor{
		stepOne:   t.StepOne,
		stepTwo:   t.StepTwo,
		stepThree: t.StepThree,
		stepFour:  t.StepFour,
		stepFive:  t.StepFive,
	}
}

func (t *TaskDemo) StepOne(data *TaskData) (retry bool, output interface{}, err error) {
	time.Sleep(time.Second)
	return false, data.Input, err
}

func (t *TaskDemo) StepOneRollback(data *TaskData) (retry bool, output interface{}, err error) {
	time.Sleep(time.Second)
	return false, data.Input, err
}

func (t *TaskDemo) StepTwo(data *TaskData) (retry bool, output interface{}, err error) {
	time.Sleep(time.Second)
	return false, data.Input, nil
}

func (t *TaskDemo) StepThree(data *TaskData) (retry bool, output interface{}, err error) {
	time.Sleep(time.Second)
	return false, data.Input, err
}

func (t *TaskDemo) StepFour(data *TaskData) (retry bool, output interface{}, err error) {
	time.Sleep(time.Second * 2)
	return false, data.Input, err
}

func (t *TaskDemo) StepFive(data *TaskData) (retry bool, output interface{}, err error) {
	time.Sleep(time.Second)
	return false, data.Input, err
}

func commonTaskx(cluster1, cluster2, cluster3 cluster.ICluster) (dispatcher1, dispatcher2, dispatcher3 *taskDispatcher, receiver1, receiver2, receiver3 *taskReceiver, err error) {
	config := dbv1.Config{
		Url:      "mysql-primary.app.svc.cluster.local:3306",
		User:     "test-user",
		Password: "test-user",
		DbName:   "task_test",
		Debug:    true,
	}

	l := logger.Config{
		Level: logger.DebugLevel,
	}

	logger.InitLogger(&l)

	client, err := dbv1.NewDBClient(config)
	if err != nil {
		return
	}

	taskDao := &taskxdao.TaskDao{
		IDB: client,
	}
	subtaskDao := &taskxdao.SubtaskDao{
		IDB: client,
	}
	cfg := &Config{
		RemoteCallTimout:     time.Second * 3,
		BackupTaskAgeSeconds: 120,
	}

	receiver1 = &taskReceiver{
		Cluster:                  cluster1,
		TaskDao:                  taskDao,
		SubtaskDao:               subtaskDao,
		subtaskInflight:          inflight.NewInFlight(),
		taskInflight:             inflight.NewInFlight(),
		subtaskWorker:            50,
		taskWorker:               5,
		subtaskRollbackWorker:    10,
		taskQueueSize:            1000,
		subtaskQueueSize:         1000,
		subtaskRollbackQueueSize: 200,
		cfg:                      cfg,
	}
	receiver2 = &taskReceiver{
		Cluster:                  cluster2,
		TaskDao:                  taskDao,
		SubtaskDao:               subtaskDao,
		subtaskInflight:          inflight.NewInFlight(),
		taskInflight:             inflight.NewInFlight(),
		subtaskWorker:            50,
		taskWorker:               5,
		subtaskRollbackWorker:    10,
		taskQueueSize:            1000,
		subtaskQueueSize:         1000,
		subtaskRollbackQueueSize: 200,
		cfg:                      cfg,
	}
	receiver3 = &taskReceiver{
		Cluster:                  cluster3,
		TaskDao:                  taskDao,
		SubtaskDao:               subtaskDao,
		subtaskInflight:          inflight.NewInFlight(),
		taskInflight:             inflight.NewInFlight(),
		subtaskWorker:            50,
		taskWorker:               5,
		subtaskRollbackWorker:    10,
		taskQueueSize:            1000,
		subtaskQueueSize:         1000,
		subtaskRollbackQueueSize: 200,
		cfg:                      cfg,
	}
	dispatcher1 = &taskDispatcher{
		Cluster:                cluster1,
		TaskDao:                taskDao,
		SubtaskDao:             subtaskDao,
		DBClient:               client,
		cfg:                    cfg,
		TaskReceiver:           receiver1,
		allocateWorkerInflight: inflight.NewInFlight(),
	}
	dispatcher2 = &taskDispatcher{
		Cluster:                cluster2,
		TaskDao:                taskDao,
		SubtaskDao:             subtaskDao,
		DBClient:               client,
		cfg:                    cfg,
		TaskReceiver:           receiver2,
		allocateWorkerInflight: inflight.NewInFlight(),
	}
	dispatcher3 = &taskDispatcher{
		Cluster:                cluster3,
		TaskDao:                taskDao,
		SubtaskDao:             subtaskDao,
		DBClient:               client,
		cfg:                    cfg,
		TaskReceiver:           receiver3,
		allocateWorkerInflight: inflight.NewInFlight(),
	}
	receiver1.TaskDispatcher = dispatcher1
	receiver2.TaskDispatcher = dispatcher2
	receiver3.TaskDispatcher = dispatcher3

	return
}

func submitTaskAndCheck(t *testing.T, dispatcher1 *taskDispatcher) string {
	var (
		requestId   = "traceId"
		description = "description"
	)

	task := NewTask(taskName).SetRequestId(requestId).SetDescription(description).SetUrgent()
	one := NewSubtask(stepOne).SetInput(stepOne)
	two := NewSubtask(stepTwo).SetInput(stepTwo)
	three := NewSubtask(stepThree).SetInput(stepThree)
	four := NewSubtask(stepFour).SetInput(stepFour)
	five := NewSubtask(stepFive).SetInput(stepFive)

	err := task.AddSubTask(one)
	if err != nil {
		panic(err)
	}
	err = task.AddSubTask(two)
	if err != nil {
		panic(err)
	}
	err = task.AddSubTask(three)
	if err != nil {
		panic(err)
	}
	err = task.AddSubTask(four)
	if err != nil {
		panic(err)
	}
	err = task.AddSubTask(five)
	if err != nil {
		panic(err)
	}

	err = task.AddDirectedEdge(one, two)
	if err != nil {
		panic(err)
	}
	err = task.AddDirectedEdge(two, three)
	if err != nil {
		panic(err)
	}
	err = task.AddDirectedEdge(two, four)
	if err != nil {
		panic(err)
	}
	err = task.AddDirectedEdge(three, five)
	if err != nil {
		panic(err)
	}
	err = task.AddDirectedEdge(four, five)
	if err != nil {
		panic(err)
	}

	err = dispatcher1.SubmitTask(task)
	if err != nil {
		panic(err)
	}

	var (
		dbTask       *taskxdao.Task
		dbSubTasks   []*taskxdao.Subtask
		dbSubTaskMap map[string]*taskxdao.Subtask
	)
	for {
		dbTasks, _ := dispatcher1.TaskDao.GetTasksByTaskIds([]string{task.GetTaskId()})
		if dbTasks[0].IsFinished() {
			dbTask = dbTasks[0]
			dbSubTasks, dbSubTaskMap, _ = dispatcher1.SubtaskDao.GetSubtasksByTaskId(task.GetTaskId())
			break
		}
		time.Sleep(time.Second * 2)
	}

	assert.Equal(t, requestId, dbTask.RequestId, "check requestId failed")
	assert.Equal(t, description, dbTask.Description, "check description failed")
	for _, v := range dbSubTasks {
		assert.Equal(t, true, v.IsFinished(), "check subtask finished failed")
		output := Output{}
		_ = json.Unmarshal([]byte(v.Output), &output)
		assert.Equal(t, v.TaskName, output.Output, "check subtask output failed")
	}

	dbOne := dbSubTaskMap[one.GetTaskId()]
	dbTwo := dbSubTaskMap[two.GetTaskId()]
	dbThree := dbSubTaskMap[three.GetTaskId()]
	dbFour := dbSubTaskMap[four.GetTaskId()]
	dbFive := dbSubTaskMap[five.GetTaskId()]

	// check preSubtaskId
	assert.Equal(t, dbOne.PreSubtaskId, "", "check preSubtaskId failed")
	assert.Equal(t, dbTwo.PreSubtaskId, one.GetTaskId(), "check preSubtaskId failed")
	assert.Equal(t, dbThree.PreSubtaskId, two.GetTaskId(), "check preSubtaskId failed")
	assert.Equal(t, dbFour.PreSubtaskId, two.GetTaskId(), "check preSubtaskId failed")
	assert.Contains(t, dbFive.PreSubtaskId, three.GetTaskId(), "check preSubtaskId failed")
	assert.Contains(t, dbFive.PreSubtaskId, four.GetTaskId(), "check preSubtaskId failed")

	// check finish time
	assert.Equal(t, true, dbOne.UpdateTime.Time().Sub(dbTwo.UpdateTime.Time()) <= 0, "check finishTime failed")
	assert.Equal(t, true, dbTwo.UpdateTime.Time().Sub(dbThree.UpdateTime.Time()) <= 0, "check finishTime failed")
	assert.Equal(t, true, dbTwo.UpdateTime.Time().Sub(dbFour.UpdateTime.Time()) <= 0, "check finishTime failed")
	assert.Equal(t, true, dbThree.UpdateTime.Time().Sub(dbFive.UpdateTime.Time()) <= 0, "check finishTime failed")
	assert.Equal(t, true, dbFour.UpdateTime.Time().Sub(dbFive.UpdateTime.Time()) <= 0, "check finishTime failed")

	return task.GetTaskId()
}

type TaskRollbackDemo struct {
}

func (t *TaskRollbackDemo) Name() string {
	return taskRollbackName
}

func (t *TaskRollbackDemo) FinishedTask(data *TaskData) (retry bool, err error) {
	return false, nil
}
func (t *TaskRollbackDemo) FailedTask(data *TaskData) (retry bool, err error) {
	return false, errors.New("FailedTask")
}

func (t *TaskRollbackDemo) GetExecutorWithRollback() (TaskExecutor, map[string]SubTaskExecutor, map[string]SubTaskExecutor) {
	return t, map[string]SubTaskExecutor{
			stepOne:   t.StepOne,
			stepTwo:   t.StepTwo,
			stepThree: t.StepThree,
			stepFour:  t.StepFour,
			stepFive:  t.StepFive,
		}, map[string]SubTaskExecutor{
			stepTwo:   t.StepTwoRollback,
			stepThree: t.StepThreeRollback,
			stepFour:  t.StepFourRollback,
		}
}

func (t *TaskRollbackDemo) StepOne(data *TaskData) (retry bool, output interface{}, err error) {
	logger.Info("step one")
	return false, data.Input, err
}

func (t *TaskRollbackDemo) StepOneRollback(data *TaskData) (retry bool, output interface{}, err error) {
	return false, data.Input, err
}

func (t *TaskRollbackDemo) StepTwo(data *TaskData) (retry bool, output interface{}, err error) {
	return false, "", nil
}

func (t *TaskRollbackDemo) StepTwoRollback(data *TaskData) (retry bool, output interface{}, err error) {
	return false, data.Input + " rollback", err
}

func (t *TaskRollbackDemo) StepThree(data *TaskData) (retry bool, output interface{}, err error) {
	return false, data.Input, err
}

func (t *TaskRollbackDemo) StepThreeRollback(data *TaskData) (retry bool, output interface{}, err error) {
	time.Sleep(2 * time.Second)
	return false, data.Input + " rollback", err
}

func (t *TaskRollbackDemo) StepFour(data *TaskData) (retry bool, output interface{}, err error) {
	return false, data.Input, errors.New("test rollback err")
}

func (t *TaskRollbackDemo) StepFourRollback(data *TaskData) (retry bool, output interface{}, err error) {
	time.Sleep(1 * time.Second)
	return false, data.Input + " rollback", err
}

func (t *TaskRollbackDemo) StepFive(data *TaskData) (retry bool, output interface{}, err error) {
	return false, data.Input, nil
}

func submitRollbackTaskAndCheck(t *testing.T, dispatcher1 *taskDispatcher) {
	task := NewTask(taskRollbackName).SetUrgent()
	one := NewSubtask(stepOne).SetInput(stepOne)
	two := NewSubtask(stepTwo).SetInput(stepTwo)
	three := NewSubtask(stepThree).SetInput(stepThree)
	four := NewSubtask(stepFour).SetInput(stepFour)
	five := NewSubtask(stepFive).SetInput(stepFive)

	_ = task.AddSubTask(one)
	_ = task.AddSubTask(two)
	_ = task.AddSubTask(three)
	_ = task.AddSubTask(four)
	_ = task.AddSubTask(five)
	_ = task.AddDirectedEdge(one, two)
	_ = task.AddDirectedEdge(two, three)
	_ = task.AddDirectedEdge(two, four)
	_ = task.AddDirectedEdge(three, five)
	_ = task.AddDirectedEdge(four, five)
	_ = dispatcher1.SubmitTask(task)

	var (
		dbSubTaskMap map[string]*taskxdao.Subtask
	)
	for {
		dbTasks, _ := dispatcher1.TaskDao.GetTasksByTaskIds([]string{task.GetTaskId()})
		if dbTasks[0].IsFinished() {
			_, dbSubTaskMap, _ = dispatcher1.SubtaskDao.GetSubtasksByTaskId(task.GetTaskId())
			break
		}
		time.Sleep(time.Second * 2)
	}

	dbTwo := dbSubTaskMap[two.GetTaskId()]
	dbThree := dbSubTaskMap[three.GetTaskId()]
	dbFour := dbSubTaskMap[four.GetTaskId()]
	dbFive := dbSubTaskMap[five.GetTaskId()]

	// check preSubtaskId
	assert.Equal(t, true, dbTwo.RollbackFinished(), "check rollback finished failed")
	assert.Equal(t, true, dbThree.RollbackFinished(), "check rollback finished failed")
	assert.Equal(t, true, dbFour.RollbackFinished(), "check rollback finished failed")

	assert.Equal(t, string(TaskFailed), dbFour.TaskState, "check subtask state failed")
	assert.Equal(t, 0, dbFour.Retry, "check subtask retryCount failed")
	assert.Equal(t, false, dbFive.IsFinished(), "check subtask finish state failed")

	// check finish time
	assert.Equal(t, true, dbTwo.UpdateTime.Time().Sub(dbThree.UpdateTime.Time()) >= 0, "check finishTime failed")
	assert.Equal(t, true, dbTwo.UpdateTime.Time().Sub(dbFour.UpdateTime.Time()) >= 0, "check finishTime failed")

	return
}

type TaskNonRetryable struct {
}

func (t *TaskNonRetryable) Name() string {
	return taskNameOfNonRetryable
}

func (t *TaskNonRetryable) FinishedTask(data *TaskData) (retry bool, err error) {
	return false, nil
}
func (t *TaskNonRetryable) FailedTask(data *TaskData) (retry bool, err error) {
	return false, errors.New("FailedTask")
}

func (t *TaskNonRetryable) StepOne(data *TaskData) (retry bool, output interface{}, err error) {
	return false, nil, ErrNonRetryable
}

func (t *TaskNonRetryable) GetExecutor() (TaskExecutor, map[string]SubTaskExecutor) {
	return t, map[string]SubTaskExecutor{
		stepOne: t.StepOne,
	}
}

func submitNonRetryTaskAndCheck(t *testing.T, dispatcher1 *taskDispatcher) {
	task := NewTask(taskNameOfNonRetryable).SetUrgent()
	one := NewSubtask(stepOne).SetInput(stepOne)
	_ = task.AddSubTask(one)

	_ = dispatcher1.SubmitTask(task)

	var (
		dbSubTaskMap map[string]*taskxdao.Subtask
	)
	for {
		dbTasks, _ := dispatcher1.TaskDao.GetTasksByTaskIds([]string{task.GetTaskId()})
		if dbTasks[0].IsFinished() {
			_, dbSubTaskMap, _ = dispatcher1.SubtaskDao.GetSubtasksByTaskId(task.GetTaskId())
			break
		}
		time.Sleep(time.Second * 2)
	}

	dbOne := dbSubTaskMap[one.GetTaskId()]
	assert.Equal(t, string(TaskFailed), dbOne.TaskState, "check task state failed")
	assert.Equal(t, DefaultRetryCount, dbOne.Retry, "check task retryCount failed")
}

func TestDisPatch(t *testing.T) {
	cluster1, cluster2, cluster3 := commonCluster()
	dispatcher1, dispatcher2, dispatcher3, receiver1, receiver2, receiver3, err := commonTaskx(cluster1, cluster2, cluster3)
	if err != nil {
		logger.Info("test TestDisPatch skip. %v", err)
		return
	}

	demo := &TaskDemo{}
	rollbackDemo := TaskRollbackDemo{}
	retryable := TaskNonRetryable{}
	RegisterTaskExecutor(demo.GetExecutor())
	RegisterTaskExecutorWithRollback(rollbackDemo.GetExecutorWithRollback())
	RegisterTaskExecutor(retryable.GetExecutor())

	_ = receiver1.Start()
	_ = receiver2.Start()
	_ = receiver3.Start()
	defer receiver1.Close()
	defer receiver2.Close()
	defer receiver3.Close()

	tracker1 := cluster.NewDefaultJobTracker(5, dispatcher1)
	tracker2 := cluster.NewDefaultJobTracker(5, dispatcher2)
	tracker3 := cluster.NewDefaultJobTracker(5, dispatcher3)
	defer tracker1.Close()
	defer tracker2.Close()
	defer tracker3.Close()

	_ = cluster1.AddJobTracker(tracker1)
	_ = cluster2.AddJobTracker(tracker2)
	_ = cluster3.AddJobTracker(tracker3)
	go cluster1.Start()
	go cluster2.Start()
	go cluster3.Start()
	defer cluster1.Close()
	defer cluster2.Close()
	defer cluster3.Close()

	// 提交一个任务
	taskId := submitTaskAndCheck(t, dispatcher1)

	outputMap, err := dispatcher1.GetTaskOutput(taskId)
	if err != nil {
		return
	}

	for k, output := range outputMap {
		logger.Info("k = %s, output = %v \n", k, output)
	}

	//提交一个回滚任务
	submitRollbackTaskAndCheck(t, dispatcher1)

	// test NonRetryable Task
	submitNonRetryTaskAndCheck(t, dispatcher1)
}
