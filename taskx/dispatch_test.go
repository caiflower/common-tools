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
	"encoding/json"
	"errors"
	"io"
	"os"
	"testing"
	"time"

	"github.com/caiflower/common-tools/cluster"
	dbv1 "github.com/caiflower/common-tools/db/v1"
	"github.com/caiflower/common-tools/pkg/inflight"
	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/caiflower/common-tools/taskx/dao"
	"github.com/caiflower/common-tools/taskx/dao/model"
	"github.com/stretchr/testify/assert"
)

const (
	taskDemoName           = "taskDemo"
	taskRollbackName       = "taskRollbackDemo"
	taskNameOfNonRetryable = "NonRetryable"

	stepOne   = "stepOne"
	stepTwo   = "stepTwo"
	stepThree = "stepThree"
	stepFour  = "stepFour"
	stepFive  = "stepFive"
)

func commonCluster() (cluster1, cluster2, cluster3 *cluster.Cluster) {
	c1 := cluster.Config{}
	c2 := cluster.Config{}
	c3 := cluster.Config{}

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

	return cluster1, cluster2, cluster3
}

func commonTaskx(cluster1, cluster2, cluster3 cluster.ICluster) (dispatcher1, dispatcher2, dispatcher3 *taskDispatcher, receiver1, receiver2, receiver3 *taskReceiver, err error) {
	//config := dbv1.Config{
	//	Url:      "mysql-primary.app.svc.cluster.local:3306",
	//	User:     "test-user",
	//	Password: "test-user",
	//	DbName:   "task_test",
	//	Debug:    true,
	//}

	config := dbv1.Config{
		Dialect: "sqlite",
		Url:     "file:./app.db?cache=shared&_fk=1&mode=rwc&journal_mode=WAL",
		Debug:   true,
	}

	l := logger.Config{
		Level: logger.DebugLevel,
	}

	logger.InitLogger(&l)

	client, err := dbv1.NewDBClient(config)
	if err != nil {
		return
	}

	if config.Dialect == "sqlite" {
		file, _ := os.Open("./dao/table-sqlite.sql")
		defer file.Close()
		sql, _ := io.ReadAll(file)
		_, err = client.DB.ExecContext(context.TODO(), string(sql))
		if err != nil {
			panic(err)
		}
	}

	taskDao := dao.NewTaskDAOWithClient(client)
	taskBakDao := dao.NewTaskBakDAOWithClient(client)
	subtaskDao := dao.NewSubtaskDAOWithClient(client)
	subtaskBakDao := dao.NewSubtaskBakDAOWithClient(client)
	cfg := &Config{
		RemoteCallTimeout: time.Second * 3,
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
		TaskBakDao:             taskBakDao,
		SubtaskDao:             subtaskDao,
		SubtaskBakDao:          subtaskBakDao,
		DBClient:               client,
		cfg:                    cfg,
		TaskReceiver:           receiver1,
		allocateWorkerInflight: inflight.NewInFlight(),
	}
	dispatcher2 = &taskDispatcher{
		Cluster:                cluster2,
		TaskDao:                taskDao,
		TaskBakDao:             taskBakDao,
		SubtaskDao:             subtaskDao,
		SubtaskBakDao:          subtaskBakDao,
		DBClient:               client,
		cfg:                    cfg,
		TaskReceiver:           receiver2,
		allocateWorkerInflight: inflight.NewInFlight(),
	}
	dispatcher3 = &taskDispatcher{
		Cluster:                cluster3,
		TaskDao:                taskDao,
		TaskBakDao:             taskBakDao,
		SubtaskDao:             subtaskDao,
		SubtaskBakDao:          subtaskBakDao,
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

func submitDemoTaskAndCheck(t *testing.T, dispatcher1 *taskDispatcher) string {
	var (
		requestId   = "traceId"
		description = "description"
	)

	task := NewTask(taskDemoName).SetRequestID(requestId).SetDescription(description).SetUrgent()
	one := NewSubtask(stepOne).SetInput(stepOne)
	two := NewSubtask(stepTwo).SetInput(stepTwo)
	three := NewSubtask(stepThree).SetInput(stepThree)
	four := NewSubtask(stepFour).SetInput(stepFour)
	five := NewSubtask(stepFive).SetInput(stepFive)

	err := task.AddSubtask(one)
	if err != nil {
		panic(err)
	}
	err = task.AddSubtask(two)
	if err != nil {
		panic(err)
	}
	err = task.AddSubtask(three)
	if err != nil {
		panic(err)
	}
	err = task.AddSubtask(four)
	if err != nil {
		panic(err)
	}
	err = task.AddSubtask(five)
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
		dbTask       *model.Task
		dbSubTasks   []model.Subtask
		dbSubTaskMap map[string]*model.Subtask
	)

	dbSubTaskMap = make(map[string]*model.Subtask)
	for {
		dbTask, err = dispatcher1.TaskDao.GetByID(context.TODO(), task.GetID())
		if err != nil {
			panic(err)
		}
		if isFinished(dbTask.State) {
			dbSubTasks, _ = dispatcher1.SubtaskDao.GetByTaskID(context.TODO(), task.GetID())
			for i, subTask := range dbSubTasks {
				dbSubTaskMap[subTask.ID] = &dbSubTasks[i]
			}
			break
		}
		time.Sleep(time.Second * 2)
	}

	assert.Equal(t, requestId, dbTask.RequestID, "check requestId failed")
	assert.Equal(t, description, dbTask.Description, "check description failed")
	for _, v := range dbSubTasks {
		assert.Equal(t, true, isFinished(v.State), "check subtask finished failed")
		output := Output{}
		_ = json.Unmarshal([]byte(v.Output), &output)
		assert.Equal(t, v.TaskName, output.Output, "check subtask output failed")
	}

	dbOne := dbSubTaskMap[one.GetID()]
	dbTwo := dbSubTaskMap[two.GetID()]
	dbThree := dbSubTaskMap[three.GetID()]
	dbFour := dbSubTaskMap[four.GetID()]
	dbFive := dbSubTaskMap[five.GetID()]

	// check preSubtaskId
	assert.Equal(t, dbOne.PreSubtaskID, "", "check preSubtaskId failed")
	assert.Equal(t, dbTwo.PreSubtaskID, one.GetID(), "check preSubtaskId failed")
	assert.Equal(t, dbThree.PreSubtaskID, two.GetID(), "check preSubtaskId failed")
	assert.Equal(t, dbFour.PreSubtaskID, two.GetID(), "check preSubtaskId failed")
	assert.Contains(t, dbFive.PreSubtaskID, three.GetID(), "check preSubtaskId failed")
	assert.Contains(t, dbFive.PreSubtaskID, four.GetID(), "check preSubtaskId failed")

	// check finish time
	assert.Equal(t, true, dbOne.LastRunTime.Time().Sub(dbTwo.LastRunTime.Time()) <= 0, "check finishTime failed")
	assert.Equal(t, true, dbTwo.LastRunTime.Time().Sub(dbThree.LastRunTime.Time()) <= 0, "check finishTime failed")
	assert.Equal(t, true, dbTwo.LastRunTime.Time().Sub(dbFour.LastRunTime.Time()) <= 0, "check finishTime failed")
	assert.Equal(t, true, dbThree.LastRunTime.Time().Sub(dbFive.LastRunTime.Time()) <= 0, "check finishTime failed")
	assert.Equal(t, true, dbFour.LastRunTime.Time().Sub(dbFive.LastRunTime.Time()) <= 0, "check finishTime failed")

	outputs, err := dispatcher1.GetTaskOutput(task.GetID())
	assert.Nil(t, err, "check task output failed")
	assert.Equal(t, 6, len(outputs), "check task output failed")
	assert.Equal(t, stepOne, outputs[stepOne].Output, "check task output failed")
	assert.Equal(t, stepTwo, outputs[stepTwo].Output, "check task output failed")
	assert.Equal(t, stepThree, outputs[stepThree].Output, "check task output failed")
	assert.Equal(t, stepFour, outputs[stepFour].Output, "check task output failed")
	assert.Equal(t, stepFive, outputs[stepFive].Output, "check task output failed")

	return task.GetID()
}

type TaskDemo struct {
}

func (t *TaskDemo) Name() string {
	return taskDemoName
}

func (t *TaskDemo) FinishedTask(data *TaskData) (err error) {
	return nil
}
func (t *TaskDemo) FailedTask(data *TaskData) (err error) {
	return nil
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

func (t *TaskDemo) StepOne(data *TaskData) (output interface{}, err error) {
	return data.Input, err
}

func (t *TaskDemo) StepOneRollback(data *TaskData) (output interface{}, err error) {
	return data.Input, err
}

func (t *TaskDemo) StepTwo(data *TaskData) (output interface{}, err error) {
	return data.Input, nil
}

func (t *TaskDemo) StepThree(data *TaskData) (output interface{}, err error) {
	return data.Input, err
}

func (t *TaskDemo) StepFour(data *TaskData) (output interface{}, err error) {
	return data.Input, err
}

func (t *TaskDemo) StepFive(data *TaskData) (output interface{}, err error) {
	return data.Input, err
}

type TaskRollbackDemo struct {
}

func (t *TaskRollbackDemo) Name() string {
	return taskRollbackName
}

func (t *TaskRollbackDemo) FinishedTask(data *TaskData) (err error) {
	return nil
}
func (t *TaskRollbackDemo) FailedTask(data *TaskData) (err error) {
	return nil
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
			stepFive:  t.StepFourRollback,
		}
}

func (t *TaskRollbackDemo) StepOne(data *TaskData) (output interface{}, err error) {
	return data.Input, err
}

func (t *TaskRollbackDemo) StepOneRollback(data *TaskData) (output interface{}, err error) {
	return data.Input, err
}

func (t *TaskRollbackDemo) StepTwo(data *TaskData) (output interface{}, err error) {
	return "", nil
}

func (t *TaskRollbackDemo) StepTwoRollback(data *TaskData) (output interface{}, err error) {
	return data.Input + " rollback", err
}

func (t *TaskRollbackDemo) StepThree(data *TaskData) (output interface{}, err error) {
	return data.Input, err
}

func (t *TaskRollbackDemo) StepThreeRollback(data *TaskData) (output interface{}, err error) {
	time.Sleep(2 * time.Second)
	return data.Input + " rollback", err
}

func (t *TaskRollbackDemo) StepFour(data *TaskData) (output interface{}, err error) {
	return data.Input, errors.New("test rollback err")
}

func (t *TaskRollbackDemo) StepFourRollback(data *TaskData) (output interface{}, err error) {
	time.Sleep(1 * time.Second)
	return data.Input + " rollback", err
}

func (t *TaskRollbackDemo) StepFive(data *TaskData) (output interface{}, err error) {
	return data.Input, nil
}

func submitRollbackTaskAndCheck(t *testing.T, dispatcher1 *taskDispatcher) {
	task := NewTask(taskRollbackName).SetUrgent()
	one := NewSubtask(stepOne).SetInput(stepOne)
	two := NewSubtask(stepTwo).SetInput(stepTwo)
	three := NewSubtask(stepThree).SetInput(stepThree)
	four := NewSubtask(stepFour).SetInput(stepFour)
	five := NewSubtask(stepFive).SetInput(stepFive)

	_ = task.AddSubtask(one)
	_ = task.AddSubtask(two)
	_ = task.AddSubtask(three)
	_ = task.AddSubtask(four)
	_ = task.AddSubtask(five)
	_ = task.AddDirectedEdge(one, two)
	_ = task.AddDirectedEdge(two, three)
	_ = task.AddDirectedEdge(two, four)
	_ = task.AddDirectedEdge(three, five)
	_ = task.AddDirectedEdge(four, five)
	_ = dispatcher1.SubmitTask(task)

	var (
		dbSubTaskMap map[string]*model.Subtask
	)

	dbSubTaskMap = make(map[string]*model.Subtask)
	for {
		dbTask, err := dispatcher1.TaskDao.GetByID(context.TODO(), task.GetID())
		if err != nil {
			panic(err)
		}
		if isFinished(dbTask.State) {
			dbSubTasks, _ := dispatcher1.SubtaskDao.GetByTaskID(context.TODO(), task.GetID())
			for i, v := range dbSubTasks {
				dbSubTaskMap[v.ID] = &dbSubTasks[i]
			}
			break
		}
		time.Sleep(time.Second * 2)
	}

	dbOne := dbSubTaskMap[one.GetID()]
	dbTwo := dbSubTaskMap[two.GetID()]
	dbThree := dbSubTaskMap[three.GetID()]
	dbFour := dbSubTaskMap[four.GetID()]
	dbFive := dbSubTaskMap[five.GetID()]

	// check preSubtaskId
	assert.Equal(t, true, isRollbackFinished(dbTwo.Rollback), "check rollback finished failed")
	assert.Equal(t, true, isRollbackFinished(dbThree.Rollback), "check rollback finished failed")
	assert.Equal(t, true, isRollbackFinished(dbFour.Rollback), "check rollback finished failed")
	assert.Equal(t, true, TaskRollbackState(dbOne.Rollback) == NoneRollback, "check noneRollback rollback failed")
	assert.Equal(t, true, TaskRollbackState(dbFive.Rollback) == RollbackPending, "check rollbackPending rollback failed")

	assert.Equal(t, TaskFailed, dbFour.State, "check subtask state failed")
	assert.Equal(t, int8(0), dbFour.Retry, "check subtask retryCount failed")
	assert.Equal(t, false, isFinished(dbFive.State), "check subtask finish state failed")

	// check finish time
	assert.Equal(t, true, dbTwo.LastRunTime.Time().Sub(dbThree.LastRunTime.Time()) > 0, "check finishTime failed")
	assert.Equal(t, true, dbTwo.LastRunTime.Time().Sub(dbFour.LastRunTime.Time()) > 0, "check finishTime failed")

	return
}

type TaskNonRetryable struct {
}

func (t *TaskNonRetryable) Name() string {
	return taskNameOfNonRetryable
}

func (t *TaskNonRetryable) FinishedTask(data *TaskData) (err error) {
	return nil
}
func (t *TaskNonRetryable) FailedTask(data *TaskData) (err error) {
	return errors.New("FailedTask")
}

func (t *TaskNonRetryable) StepOne(data *TaskData) (output interface{}, err error) {
	return nil, ErrNonRetryable
}

func (t *TaskNonRetryable) GetExecutor() (TaskExecutor, map[string]SubTaskExecutor) {
	return t, map[string]SubTaskExecutor{
		stepOne: t.StepOne,
	}
}

func submitNonRetryTaskAndCheck(t *testing.T, dispatcher1 *taskDispatcher) {
	task := NewTask(taskNameOfNonRetryable).SetUrgent()
	one := NewSubtask(stepOne).SetInput(stepOne)
	_ = task.AddSubtask(one)

	_ = dispatcher1.SubmitTask(task)

	var dbOne model.Subtask
	for {
		dbTask, err := dispatcher1.TaskDao.GetByID(context.TODO(), task.GetID())
		if err != nil {
			panic(err)
		}
		if isFinished(dbTask.State) {
			subTasks, _ := dispatcher1.SubtaskDao.GetByTaskID(context.TODO(), task.GetID())
			dbOne = subTasks[0]
			break
		}
		time.Sleep(time.Second * 2)
	}

	assert.Equal(t, TaskFailed, dbOne.State, "check task state failed")
	assert.Equal(t, int8(DefaultRetryCount), dbOne.Retry, "check task retryCount failed")
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

	_ = cluster1.AddJobTracker(tracker1)
	_ = cluster2.AddJobTracker(tracker2)
	_ = cluster3.AddJobTracker(tracker3)
	_ = cluster1.Start()
	_ = cluster2.Start()
	_ = cluster3.Start()
	defer cluster1.Close()
	defer cluster2.Close()
	defer cluster3.Close()
	for {
		if cluster1.IsReady() && cluster2.IsReady() && cluster3.IsReady() {
			break
		}
	}

	// 提交一个任务
	_ = submitDemoTaskAndCheck(t, dispatcher1)

	//提交一个回滚任务
	submitRollbackTaskAndCheck(t, dispatcher1)

	// test NonRetryable Task
	submitNonRetryTaskAndCheck(t, dispatcher1)

	os.Remove("./app.db")
}
