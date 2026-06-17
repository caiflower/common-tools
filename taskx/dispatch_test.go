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
	"fmt"
	"io"
	"math/rand"
	"net"
	"os"
	"testing"
	"time"

	"github.com/caiflower/common-tools/cluster"
	dbv1 "github.com/caiflower/common-tools/db/v1"
	"github.com/caiflower/common-tools/pkg/basic"
	"github.com/caiflower/common-tools/pkg/inflight"
	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/caiflower/common-tools/taskx/dao"
	"github.com/caiflower/common-tools/taskx/dao/model"
	"github.com/caiflower/common-tools/taskx/executor"
	gocache "github.com/patrickmn/go-cache"
	"github.com/stretchr/testify/assert"
)

const (
	taskDemoName           = "taskDemo_flow"
	taskRollbackName       = "taskRollback_flow"
	taskRollbackFailedName = "taskRollbackFailed_flow"
	taskRollbackCustomName = "taskRollbackCustom_flow"
	taskNameOfNonRetryable = "taskNonRetryable_flow"
	taskBranchName         = "taskBranch_flow"
	taskNestedBranchName   = "taskNestedBranch_flow"

	stepOne   = "stepOne"
	stepTwo   = "stepTwo"
	stepThree = "stepThree"
	stepFour  = "stepFour"
	stepFive  = "stepFive"
)

// ===== 公共基础设施 =====

// freePort asks the kernel for a free TCP port and immediately releases it.
// The tiny race window between release and bind is acceptable for unit tests.
func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("freePort: listen failed: %v", err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func commonCluster(t *testing.T) (cluster1, cluster2, cluster3 *cluster.Cluster) {
	port1, port2, port3 := freePort(t), freePort(t), freePort(t)

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
			Port: port1,
		},
		&struct {
			Name  string
			Ip    string
			Port  int
			Local bool
		}{
			Ip:   "127.0.0.1",
			Name: "localhost2",
			Port: port2,
		}, &struct {
			Name  string
			Ip    string
			Port  int
			Local bool
		}{
			Ip:   "127.0.0.1",
			Name: "localhost3",
			Port: port3,
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
			Port: port1,
		},
		&struct {
			Name  string
			Ip    string
			Port  int
			Local bool
		}{
			Ip:   "127.0.0.1",
			Name: "localhost2",
			Port: port2,
		}, &struct {
			Name  string
			Ip    string
			Port  int
			Local bool
		}{
			Ip:   "127.0.0.1",
			Name: "localhost3",
			Port: port3,
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
			Port: port1,
		},
		&struct {
			Name  string
			Ip    string
			Port  int
			Local bool
		}{
			Ip:   "127.0.0.1",
			Name: "localhost2",
			Port: port2,
		}, &struct {
			Name  string
			Ip    string
			Port  int
			Local bool
		}{
			Ip:   "127.0.0.1",
			Name: "localhost3",
			Port: port3,
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
	config := dbv1.Config{
		Dialect: "sqlite",
		Url:     "file:./app.db?cache=shared&_fk=1&mode=rwc&journal_mode=WAL",
		//Debug:   true,
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
		file, _ := os.Open("./dao/sql/table-sqlite.sql")
		sql, _ := io.ReadAll(file)
		_, err = client.DB.ExecContext(context.TODO(), string(sql))
		_ = file.Close()
		if err != nil {
			panic(err)
		}
	}

	taskDao := dao.NewTaskDAOWithClient(client)
	taskBakDao := dao.NewTaskBakDAOWithClient(client)
	subtaskDao := dao.NewSubtaskDAOWithClient(client)
	subtaskBakDao := dao.NewSubtaskBakDAOWithClient(client)
	taskEdgeDao := dao.NewTaskEdgeDAOWithClient(client)

	cfg := &Config{
		RemoteCallTimeout: time.Second * 3,
	}

	newReceiver := func(c cluster.ICluster) *taskReceiver {
		return &taskReceiver{
			Cluster:                  c,
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
	}
	newDispatcher := func(c cluster.ICluster, r *taskReceiver) *taskDispatcher {
		return &taskDispatcher{
			Cluster:                c,
			TaskDao:                taskDao,
			TaskBakDao:             taskBakDao,
			SubtaskDao:             subtaskDao,
			SubtaskBakDao:          subtaskBakDao,
			TaskEdgeDao:            taskEdgeDao,
			DBClient:               client,
			cfg:                    cfg,
			TaskReceiver:           r,
			allocateWorkerInflight: inflight.NewInFlight(),
			delayQueue:             basic.NewDelayQueue(),
			randSource:             rand.New(rand.NewSource(time.Now().UnixNano())),
			taskCache:              gocache.New(30*time.Second, 60*time.Second),
		}
	}

	receiver1 = newReceiver(cluster1)
	receiver2 = newReceiver(cluster2)
	receiver3 = newReceiver(cluster3)
	dispatcher1 = newDispatcher(cluster1, receiver1)
	dispatcher2 = newDispatcher(cluster2, receiver2)
	dispatcher3 = newDispatcher(cluster3, receiver3)
	receiver1.TaskDispatcher = dispatcher1
	receiver2.TaskDispatcher = dispatcher2
	receiver3.TaskDispatcher = dispatcher3

	return
}

// ===== 公共测试辅助方法 =====

// submitAndWait 提交任务并等待完成，返回 DB 中的 Task、子任务列表和按 TaskName 索引的子任务 Map
func submitAndWait(t *testing.T, dispatcher *taskDispatcher, task *Task) (*model.Task, []model.Subtask, map[string]*model.Subtask) {
	waitForTask(task, dispatcher)

	for {
		dbTask, err := dispatcher.TaskDao.GetByID(context.TODO(), task.GetID())
		if err != nil || dbTask == nil {
			time.Sleep(time.Second * 2)
			continue
		}
		if isFinished(dbTask.State) {
			subtasks := getSubTask(task.GetID(), dispatcher)
			subtaskMap := make(map[string]*model.Subtask)
			for i, v := range subtasks {
				subtaskMap[v.TaskName] = &subtasks[i]
			}
			return dbTask, subtasks, subtaskMap
		}
		time.Sleep(time.Second * 2)
	}
}

// waitForTask 提交任务到 dispatcher，重试直到成功
func waitForTask(task *Task, dispatcher *taskDispatcher) {
	for {
		err := dispatcher.SubmitTask(context.TODO(), task)
		if err != nil {
			time.Sleep(time.Second * 2)
			continue
		}
		break
	}
}

// getSubTask 从 DB 获取子任务列表
func getSubTask(taskID string, dispatcher *taskDispatcher) []model.Subtask {
	for {
		dbSubTasks, err := dispatcher.SubtaskDao.GetByTaskID(context.TODO(), taskID)
		if err != nil {
			time.Sleep(time.Second * 2)
			continue
		}
		return dbSubTasks
	}
}

// assertSubtaskState 断言子任务状态
func assertSubtaskState(t *testing.T, subtaskMap map[string]*model.Subtask, name, expectedState string) {
	s, ok := subtaskMap[name]
	if !ok {
		assert.Failf(t, "subtask not found", "subtask %s not found in map", name)
		return
	}
	assert.Equal(t, expectedState, s.State, fmt.Sprintf("subtask %s state check failed", name))
}

// ===== 步骤函数（泛型 LocalExecutor） =====

// echoInput 回显输入（支持单前驱 map 和多前驱嵌套 map）
func echoInput(ctx context.Context, input map[string]any) (map[string]any, error) {
	return input, nil
}

// failStep 总是返回错误
func failStep(ctx context.Context, input map[string]any) (map[string]any, error) {
	return nil, errors.New("test rollback err")
}

// rollbackStep 回滚步骤
func rollbackStep(ctx context.Context, input map[string]any) (map[string]any, error) {
	result := make(map[string]any)
	for k, v := range input {
		if s, ok := v.(string); ok {
			result[k] = s + " rollback"
		} else {
			result[k] = v
		}
	}
	return result, nil
}

// rollbackStepSlow 慢回滚步骤
func rollbackStepSlow(delay time.Duration) func(ctx context.Context, input map[string]any) (map[string]any, error) {
	return func(ctx context.Context, input map[string]any) (map[string]any, error) {
		time.Sleep(delay)
		result := make(map[string]any)
		for k, v := range input {
			if s, ok := v.(string); ok {
				result[k] = s + " rollback"
			} else {
				result[k] = v
			}
		}
		return result, nil
	}
}

// nonRetryableStep 返回 ErrNonRetryable
func nonRetryableStep(ctx context.Context, input map[string]any) (map[string]any, error) {
	return nil, ErrNonRetryable
}

// panicStep 触发 panic
func panicStep(ctx context.Context, input map[string]any) (map[string]any, error) {
	panic("this is a test panic in PanicStep")
}

// emptyStep 返回空结果
func emptyStep(ctx context.Context, input map[string]any) (map[string]any, error) {
	return map[string]any{}, nil
}

// ===== 测试用例 =====

func submitDemoTaskAndCheck(t *testing.T, dispatcher *taskDispatcher, done chan<- struct{}) {
	var (
		requestId   = "traceId"
		description = "description"
	)

	task := NewTask(taskDemoName).SetRequestID(requestId).SetDescription(description).SetUrgent()

	one := NewSubtask(stepOne, executor.NewLocalExecutor(echoInput)).SetInput(map[string]any{"name": stepOne})
	two := NewSubtask(stepTwo, executor.NewLocalExecutor(echoInput)).SetInput(map[string]any{"name": stepTwo})
	three := NewSubtask(stepThree, executor.NewLocalExecutor(echoInput)).SetInput(map[string]any{"name": stepThree})
	four := NewSubtask(stepFour, executor.NewLocalExecutor(echoInput)).SetInput(map[string]any{"name": stepFour})
	five := NewSubtask(stepFive, executor.NewLocalExecutor(echoInput)).SetInput(map[string]any{"name": stepFive})

	_ = task.AddSubtask(one)
	_ = task.AddSubtask(two)
	_ = task.AddSubtask(three)
	_ = task.AddSubtask(four)
	_ = task.AddSubtask(five)
	_ = task.AddEdge(one, two)
	_ = task.AddEdge(two, three)
	_ = task.AddEdge(two, four)
	_ = task.AddEdge(three, five)
	_ = task.AddEdge(four, five)

	_, err := task.Compile()
	assert.NoError(t, err)
	dbTask, dbSubTasks, _ := submitAndWait(t, dispatcher, task)

	assert.Equal(t, requestId, dbTask.RequestID, "check requestId failed")
	assert.Equal(t, description, dbTask.Description, "check description failed")
	for _, v := range dbSubTasks {
		assert.Equal(t, true, isFinished(v.State), "check subtask finished failed")
	}

	outputs, err := dispatcher.GetTaskOutput(context.TODO(), task.GetID())
	assert.Nil(t, err, "check task output failed")
	assert.Equal(t, 6, len(outputs), "check task output failed")

	done <- struct{}{}
}

func submitRollbackTaskAndCheck(t *testing.T, dispatcher *taskDispatcher, done chan<- struct{}) {
	task := NewTask(taskRollbackName).SetUrgent()

	one := NewSubtask(stepOne, executor.NewLocalExecutor(echoInput)).SetInput(map[string]any{"name": stepOne})
	two := NewSubtask(stepTwo, executor.NewLocalExecutor(emptyStep)).SetInput(map[string]any{"name": stepTwo}).SetRollbackExecutor(executor.NewLocalExecutor(rollbackStep))
	three := NewSubtask(stepThree, executor.NewLocalExecutor(echoInput)).SetInput(map[string]any{"name": stepThree}).SetRollbackExecutor(executor.NewLocalExecutor(rollbackStepSlow(2 * time.Second)))
	four := NewSubtask(stepFour, executor.NewLocalExecutor(failStep)).SetInput(map[string]any{"name": stepFour}).SetRollbackExecutor(executor.NewLocalExecutor(rollbackStepSlow(1 * time.Second)))
	five := NewSubtask(stepFive, executor.NewLocalExecutor(echoInput)).SetInput(map[string]any{"name": stepFive})

	_ = task.AddSubtask(one)
	_ = task.AddSubtask(two)
	_ = task.AddSubtask(three)
	_ = task.AddSubtask(four)
	_ = task.AddSubtask(five)
	_ = task.AddEdge(one, two)
	_ = task.AddEdge(two, three)
	_ = task.AddEdge(two, four)
	_ = task.AddEdge(three, five)
	_ = task.AddEdge(four, five)

	_, _, subtaskMap := submitAndWait(t, dispatcher, task)
	_, err := task.Compile()
	assert.NoError(t, err)

	dbTwo := subtaskMap[stepTwo]
	dbThree := subtaskMap[stepThree]
	dbFour := subtaskMap[stepFour]
	dbOne := subtaskMap[stepOne]
	dbFive := subtaskMap[stepFive]

	// check rollback state
	assert.Equal(t, true, isRollbackFinished(dbTwo.Rollback), "check rollback finished failed")
	assert.Equal(t, true, isRollbackFinished(dbThree.Rollback), "check rollback finished failed")
	assert.Equal(t, true, isRollbackFinished(dbFour.Rollback), "check rollback finished failed")
	assert.Equal(t, true, TaskRollbackState(dbOne.Rollback) == NoneRollback, "check noneRollback rollback failed")
	assert.Equal(t, true, TaskRollbackState(dbFive.Rollback) == NoneRollback, "check noneRollback rollback failed")

	assert.Equal(t, string(TaskFailed), dbFour.State, "check subtask state failed")
	assert.Equal(t, int8(0), dbFour.Retry, "check subtask retryCount failed")
	assert.Equal(t, false, isFinished(dbFive.State), "check subtask finish state failed")

	// check finish time
	t.Logf("two.LastRunTime=%v three.LastRunTime=%v four.LastRunTime=%v", dbTwo.LastRunTime.Time(), dbThree.LastRunTime.Time(), dbFour.LastRunTime.Time())
	assert.Equal(t, true, dbTwo.LastRunTime.Time().Sub(dbThree.LastRunTime.Time()) >= 0, "check finishTime failed")
	assert.Equal(t, true, dbTwo.LastRunTime.Time().Sub(dbFour.LastRunTime.Time()) >= 0, "check finishTime failed")

	done <- struct{}{}
}

func submitNonRetryTaskAndCheck(t *testing.T, dispatcher *taskDispatcher, done chan<- struct{}) {
	task := NewTask(taskNameOfNonRetryable).SetUrgent()
	one := NewSubtask(stepOne, executor.NewLocalExecutor(nonRetryableStep)).SetInput(map[string]any{"name": stepOne})
	_ = task.AddSubtask(one)

	_, dbSubTasks, _ := submitAndWait(t, dispatcher, task)

	dbOne := dbSubTasks[0]
	assert.Equal(t, string(TaskFailed), dbOne.State, "check task state failed")
	assert.Equal(t, int8(DefaultRetryCount), dbOne.Retry, "check task retryCount failed")

	done <- struct{}{}
}

func submitAffinityTaskAndCheck(t *testing.T, dispatcher *taskDispatcher, done chan struct{}) {
	task := NewTask(taskDemoName).
		SetInput("affinity test input").
		SetAffinityType(AffinityForceSameNode)

	subtask := NewSubtask(stepOne, executor.NewLocalExecutor(echoInput)).SetInput(map[string]any{"name": stepOne})
	subtask2 := NewSubtask(stepTwo, executor.NewLocalExecutor(echoInput)).SetInput(map[string]any{"name": stepTwo})
	subtask3 := NewSubtask(stepThree, executor.NewLocalExecutor(echoInput)).SetInput(map[string]any{"name": stepThree})
	subtask4 := NewSubtask(stepFour, executor.NewLocalExecutor(echoInput)).SetInput(map[string]any{"name": stepFour})
	subtask5 := NewSubtask(stepFive, executor.NewLocalExecutor(echoInput)).SetInput(map[string]any{"name": stepFive})

	_ = task.AddSubtask(subtask)
	_ = task.AddSubtask(subtask2)
	_ = task.AddSubtask(subtask3)
	_ = task.AddSubtask(subtask4)
	_ = task.AddSubtask(subtask5)
	_ = task.AddEdge(subtask, subtask2)
	_ = task.AddEdge(subtask, subtask3)
	_ = task.AddEdge(subtask2, subtask4)
	_ = task.AddEdge(subtask3, subtask4)
	_ = task.AddEdge(subtask4, subtask5)

	_, err := task.Compile()
	assert.NoError(t, err)

	dbTask, dbSubTasks, _ := submitAndWait(t, dispatcher, task)

	logger.Trace("[affinityTest] dbTask.Worker=%s", dbTask.Worker)
	for _, v := range dbSubTasks {
		logger.Trace("[affinityTest] subtask=%s worker=%s", v.TaskName, v.Worker)
	}

	for _, v := range dbSubTasks {
		assert.Equal(t, dbTask.Worker, v.Worker, "must same node")
	}

	done <- struct{}{}
}

func submitScheduleTask(t *testing.T, dispatcher *taskDispatcher, done chan<- struct{}) {
	task := NewTask(taskDemoName)
	executeTime := time.Now().Add(10 * time.Second)
	task.SetExecuteTime(executeTime)

	subtask := NewSubtask(stepOne, executor.NewLocalExecutor(echoInput)).SetInput(map[string]any{"name": "scheduled"})
	_ = task.AddSubtask(subtask)

	dbTask, dbSubTasks, _ := submitAndWait(t, dispatcher, task)

	assert.Equal(t, string(TaskSucceeded), dbTask.State, "check task state failed")
	for _, subTask := range dbSubTasks {
		assert.Equal(t, true, isFinished(subTask.State), "check subtask finished failed")
		assert.Equal(t, true, subTask.LastRunTime.Time().After(executeTime),
			fmt.Sprintf("must after ExecuteTime, executeTime = %v, lastTime = %v", executeTime.Format("2006-01-02 15:04:05"), subTask.LastRunTime.Time().Format("2006-01-02 15:04:05")))
	}

	done <- struct{}{}
}

func submitPanicTaskAndCheck(t *testing.T, dispatcher *taskDispatcher, done chan struct{}) {
	task := NewTask("taskPanic_flow").SetInput("panic test input")
	subtask := NewSubtask("panicStep", executor.NewLocalExecutor(panicStep)).SetInput(map[string]any{"name": "panic"})
	_ = task.AddSubtask(subtask)

	_, dbSubTasks, _ := submitAndWait(t, dispatcher, task)

	for _, subtask := range dbSubTasks {
		if subtask.State == string(TaskFailed) {
			var output Output
			_ = json.Unmarshal([]byte(subtask.Output), &output)
			assert.Contains(t, output.Err, "panic occurred during execution")
			break
		}
	}

	done <- struct{}{}
}

// submitBranchTaskAndCheck 条件分支：start -> pathA/pathB（选 pathA）-> end
func submitBranchTaskAndCheck(t *testing.T, dispatcher *taskDispatcher, done chan struct{}) {
	task := NewTask(taskBranchName).SetUrgent()

	start := NewSubtask("start", executor.NewLocalExecutor(echoInput)).SetInput(map[string]any{"name": "start"})
	pathA := NewSubtask("pathA", executor.NewLocalExecutor(echoInput)).SetInput(map[string]any{"name": "pathA"})
	pathB := NewSubtask("pathB", executor.NewLocalExecutor(echoInput)).SetInput(map[string]any{"name": "pathB"})
	end := NewSubtask("end", executor.NewLocalExecutor(echoInput)).SetInput(map[string]any{"name": "end"})

	_ = task.AddSubtask(start)
	_ = task.AddSubtask(pathA)
	_ = task.AddSubtask(pathB)
	_ = task.AddSubtask(end)
	_ = task.AddEdge(start, pathA)
	_ = task.AddEdge(start, pathB)
	_ = task.AddEdge(pathA, end)
	_ = task.AddEdge(pathB, end)
	_ = task.AddBranch(start, &Branch{
		Condition: func(ctx interface{}, input any) (string, error) {
			return pathA.GetID(), nil
		},
		EndNodes: map[string]bool{pathA.GetID(): true, pathB.GetID(): true},
	})

	_, err := task.Compile()
	assert.NoError(t, err)

	dbTask, _, subtaskMap := submitAndWait(t, dispatcher, task)

	assert.Equal(t, string(TaskSucceeded), dbTask.State, "branch task should succeed")
	assertSubtaskState(t, subtaskMap, "start", string(TaskSucceeded))
	assertSubtaskState(t, subtaskMap, "pathA", string(TaskSucceeded))
	assertSubtaskState(t, subtaskMap, "pathB", string(TaskSkipped))
	assertSubtaskState(t, subtaskMap, "end", string(TaskSucceeded))

	done <- struct{}{}
}

// submitNestedBranchTaskAndCheck 嵌套分支：start -> outerA/outerB（选 outerA）-> innerA1/innerA2（选 innerA1）-> end
func submitNestedBranchTaskAndCheck(t *testing.T, dispatcher *taskDispatcher, done chan struct{}) {
	task := NewTask(taskNestedBranchName).SetUrgent()

	start := NewSubtask("start", executor.NewLocalExecutor(echoInput)).SetInput(map[string]any{"name": "start"})
	outerA := NewSubtask("outerA", executor.NewLocalExecutor(echoInput)).SetInput(map[string]any{"name": "outerA"})
	outerB := NewSubtask("outerB", executor.NewLocalExecutor(echoInput)).SetInput(map[string]any{"name": "outerB"})
	innerA1 := NewSubtask("innerA1", executor.NewLocalExecutor(echoInput)).SetInput(map[string]any{"name": "innerA1"})
	innerA2 := NewSubtask("innerA2", executor.NewLocalExecutor(echoInput)).SetInput(map[string]any{"name": "innerA2"})
	end := NewSubtask("end", executor.NewLocalExecutor(echoInput)).SetInput(map[string]any{"name": "end"})

	_ = task.AddSubtask(start)
	_ = task.AddSubtask(outerA)
	_ = task.AddSubtask(outerB)
	_ = task.AddSubtask(innerA1)
	_ = task.AddSubtask(innerA2)
	_ = task.AddSubtask(end)
	_ = task.AddEdge(start, outerA)
	_ = task.AddEdge(start, outerB)
	_ = task.AddEdge(outerA, innerA1)
	_ = task.AddEdge(outerA, innerA2)
	_ = task.AddEdge(innerA1, end)
	_ = task.AddEdge(innerA2, end)
	_ = task.AddEdge(outerB, end)
	_ = task.AddBranch(start, &Branch{
		Condition: func(ctx interface{}, input any) (string, error) {
			return outerA.GetID(), nil
		},
		EndNodes: map[string]bool{outerA.GetID(): true, outerB.GetID(): true},
	})
	_ = task.AddBranch(outerA, &Branch{
		Condition: func(ctx interface{}, input any) (string, error) {
			return innerA1.GetID(), nil
		},
		EndNodes: map[string]bool{innerA1.GetID(): true, innerA2.GetID(): true},
	})

	_, err := task.Compile()
	assert.NoError(t, err)

	dbTask, _, subtaskMap := submitAndWait(t, dispatcher, task)

	assert.Equal(t, string(TaskSucceeded), dbTask.State, "nested branch task should succeed")
	assertSubtaskState(t, subtaskMap, "start", string(TaskSucceeded))
	assertSubtaskState(t, subtaskMap, "outerA", string(TaskSucceeded))
	assertSubtaskState(t, subtaskMap, "outerB", string(TaskSkipped))
	assertSubtaskState(t, subtaskMap, "innerA1", string(TaskSucceeded))
	assertSubtaskState(t, subtaskMap, "innerA2", string(TaskSkipped))
	assertSubtaskState(t, subtaskMap, "end", string(TaskSucceeded))

	done <- struct{}{}
}

// submitRollbackFailedTaskAndCheck tests StrategyRollbackFailed: only the failed subtask is rolled back.
// DAG: one → two → three(fail). With StrategyRollbackFailed, only three should be rolled back.
func submitRollbackFailedTaskAndCheck(t *testing.T, dispatcher *taskDispatcher, done chan<- struct{}) {
	task := NewTask(taskRollbackFailedName).SetUrgent().SetRollbackStrategy(StrategyRollbackFailed)

	one := NewSubtask(stepOne, executor.NewLocalExecutor(echoInput)).SetInput(map[string]any{"name": stepOne}).
		SetRollbackExecutor(executor.NewLocalExecutor(rollbackStep))
	two := NewSubtask(stepTwo, executor.NewLocalExecutor(echoInput)).SetInput(map[string]any{"name": stepTwo}).
		SetRollbackExecutor(executor.NewLocalExecutor(rollbackStep))
	three := NewSubtask(stepThree, executor.NewLocalExecutor(failStep)).SetInput(map[string]any{"name": stepThree}).
		SetRollbackExecutor(executor.NewLocalExecutor(rollbackStep))

	_ = task.AddSubtask(one)
	_ = task.AddSubtask(two)
	_ = task.AddSubtask(three)
	_ = task.AddEdge(one, two)
	_ = task.AddEdge(two, three)

	_, err := task.Compile()
	assert.NoError(t, err)

	_, _, subtaskMap := submitAndWait(t, dispatcher, task)

	dbOne := subtaskMap[stepOne]
	dbTwo := subtaskMap[stepTwo]
	dbThree := subtaskMap[stepThree]

	// three failed and has rollback executor → should be rolled back
	assert.Equal(t, string(TaskFailed), dbThree.State, "three should be failed")
	assert.Equal(t, true, isRollbackFinished(dbThree.Rollback), "three rollback should be finished")

	// one and two succeeded → with StrategyRollbackFailed, they should NOT be rolled back
	assert.Equal(t, string(TaskSucceeded), dbOne.State, "one should be succeeded")
	assert.Equal(t, string(TaskSucceeded), dbTwo.State, "two should be succeeded")
	assert.Equal(t, string(RollbackPending), dbOne.Rollback, "one should still be rollback_pending (not rolled back)")
	assert.Equal(t, string(RollbackPending), dbTwo.Rollback, "two should still be rollback_pending (not rolled back)")

	done <- struct{}{}
}

// submitRollbackCustomTaskAndCheck tests StrategyRollbackCustom: custom function selects which subtasks to roll back.
// DAG: one → two → three → four(fail). Custom func returns [one, three], skipping two.
func submitRollbackCustomTaskAndCheck(t *testing.T, dispatcher *taskDispatcher, done chan<- struct{}) {
	task := NewTask(taskRollbackCustomName).SetUrgent()

	one := NewSubtask(stepOne, executor.NewLocalExecutor(echoInput)).SetInput(map[string]any{"name": stepOne}).
		SetRollbackExecutor(executor.NewLocalExecutor(rollbackStep))
	two := NewSubtask(stepTwo, executor.NewLocalExecutor(echoInput)).SetInput(map[string]any{"name": stepTwo}).
		SetRollbackExecutor(executor.NewLocalExecutor(rollbackStep))
	three := NewSubtask(stepThree, executor.NewLocalExecutor(echoInput)).SetInput(map[string]any{"name": stepThree}).
		SetRollbackExecutor(executor.NewLocalExecutor(rollbackStep))
	four := NewSubtask(stepFour, executor.NewLocalExecutor(failStep)).SetInput(map[string]any{"name": stepFour}).
		SetRollbackExecutor(executor.NewLocalExecutor(rollbackStep))

	_ = task.AddSubtask(one)
	_ = task.AddSubtask(two)
	_ = task.AddSubtask(three)
	_ = task.AddSubtask(four)
	_ = task.AddEdge(one, two)
	_ = task.AddEdge(two, three)
	_ = task.AddEdge(three, four)

	// Custom rollback: only roll back one and three, skip two
	task.SetCustomRollbackFunc(func(completed []string, failed string) []string {
		var result []string
		for _, id := range completed {
			s := task.subtaskMap[id]
			if s != nil && (s.GetName() == stepOne || s.GetName() == stepThree) {
				result = append(result, id)
			}
		}
		return result
	})

	_, err := task.Compile()
	assert.NoError(t, err)

	_, _, subtaskMap := submitAndWait(t, dispatcher, task)

	dbOne := subtaskMap[stepOne]
	dbTwo := subtaskMap[stepTwo]
	dbThree := subtaskMap[stepThree]
	dbFour := subtaskMap[stepFour]

	// four failed
	assert.Equal(t, string(TaskFailed), dbFour.State, "four should be failed")

	// one and three were selected by custom func → should be rolled back
	assert.Equal(t, true, isRollbackFinished(dbOne.Rollback), "one rollback should be finished (selected by custom)")
	assert.Equal(t, true, isRollbackFinished(dbThree.Rollback), "three rollback should be finished (selected by custom)")

	// two was NOT selected by custom func → should remain rollback_pending
	assert.Equal(t, string(TaskSucceeded), dbTwo.State, "two should be succeeded")
	assert.Equal(t, string(RollbackPending), dbTwo.Rollback, "two should still be rollback_pending (not selected by custom)")

	done <- struct{}{}
}

// ===== 主测试入口 =====

func submitEdgeTypeDataFlowTask(t *testing.T, dispatcher *taskDispatcher, done chan<- struct{}) {
	// 测试 Input 预计算：只有 DataEdge / ControlAndDataEdge 传数据，ControlEdge 不传
	// stepA 用唯一输入 {"from":"fromA"}，其他用 {"from":"initial"}，方便区分数据来源
	task := NewTask("taskEdgeDataFlow_flow").SetUrgent()

	stepA := NewSubtask("stepA", executor.NewLocalExecutor(echoInput)).SetInput(map[string]any{"from": "fromA"})
	stepB := NewSubtask("stepB", executor.NewLocalExecutor(echoInput)).SetInput(map[string]any{"from": "initial"})
	stepC := NewSubtask("stepC", executor.NewLocalExecutor(echoInput)).SetInput(map[string]any{"from": "initial"})
	stepD := NewSubtask("stepD", executor.NewLocalExecutor(echoInput)).SetInput(map[string]any{"from": "initial"})
	stepE := NewSubtask("stepE", executor.NewLocalExecutor(echoInput)).SetInput(map[string]any{"from": "initial"})

	_ = task.AddSubtask(stepA)
	_ = task.AddSubtask(stepB)
	_ = task.AddSubtask(stepC)
	_ = task.AddSubtask(stepD)
	_ = task.AddSubtask(stepE)

	// stepA -> stepB (control-only): B 不接收 A 的数据
	_ = task.AddControlEdge(stepA, stepB)
	// stepA -> stepC (data-only): C 接收 A 的数据
	_ = task.AddDataEdge(stepA, stepC)
	// stepA -> stepD (control+data): D 接收 A 的数据
	_ = task.AddEdge(stepA, stepD)
	// stepA -> stepE (default AddEdge = control+data): E 接收 A 的数据
	_ = task.AddEdge(stepA, stepE)

	_, err := task.Compile()
	assert.NoError(t, err)

	_, _, subtaskMap := submitAndWait(t, dispatcher, task)

	dbA := subtaskMap["stepA"]
	dbB := subtaskMap["stepB"]
	dbC := subtaskMap["stepC"]
	dbD := subtaskMap["stepD"]
	dbE := subtaskMap["stepE"]

	// 所有子任务应执行完成
	assert.Equal(t, true, isFinished(dbA.State), "stepA should be finished")
	assert.Equal(t, true, isFinished(dbB.State), "stepB should be finished")
	assert.Equal(t, true, isFinished(dbC.State), "stepC should be finished")
	assert.Equal(t, true, isFinished(dbD.State), "stepD should be finished")
	assert.Equal(t, true, isFinished(dbE.State), "stepE should be finished")

	// stepB 只有 ControlEdge -> Input 应保持原值（不从 stepA 传数据）
	assert.Equal(t, `{"from":"initial"}`, dbB.Input, "stepB should keep original input (not from stepA)")
	// stepC 有 DataEdge -> Input 应为 stepA 的输出
	assert.Equal(t, `{"from":"fromA"}`, dbC.Input, "stepC should receive stepA's output")
	// stepD 有 ControlAndDataEdge -> Input 应为 stepA 的输出
	assert.Equal(t, `{"from":"fromA"}`, dbD.Input, "stepD should receive stepA's output")
	// stepE 有 ControlAndDataEdge(default AddEdge) -> Input 应为 stepA 的输出
	assert.Equal(t, `{"from":"fromA"}`, dbE.Input, "stepE should receive stepA's output")

	done <- struct{}{}
}

func TestDisPatch(t *testing.T) {
	cluster1, cluster2, cluster3 := commonCluster(t)
	dispatcher1, dispatcher2, dispatcher3, receiver1, receiver2, receiver3, err := commonTaskx(cluster1, cluster2, cluster3)
	if err != nil {
		logger.Info("test TestDisPatch skip. %v", err)
		return
	}

	defer func() {
		_ = os.Remove("./app.db")
	}()

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
		time.Sleep(time.Second * 2)
		if cluster1.IsReady() && cluster2.IsReady() && cluster3.IsReady() {
			break
		}
	}

	size := 11
	done := make(chan struct{}, size)

	go submitDemoTaskAndCheck(t, dispatcher1, done)
	go submitRollbackTaskAndCheck(t, dispatcher1, done)
	go submitRollbackFailedTaskAndCheck(t, dispatcher1, done)
	go submitRollbackCustomTaskAndCheck(t, dispatcher1, done)
	go submitNonRetryTaskAndCheck(t, dispatcher1, done)
	go submitScheduleTask(t, dispatcher1, done)
	go submitPanicTaskAndCheck(t, dispatcher1, done)
	go submitAffinityTaskAndCheck(t, dispatcher1, done)
	go submitBranchTaskAndCheck(t, dispatcher1, done)
	go submitNestedBranchTaskAndCheck(t, dispatcher1, done)
	go submitEdgeTypeDataFlowTask(t, dispatcher1, done)

	for i := 0; i < size; i++ {
		<-done
	}
}
