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
	"fmt"
	"testing"

	"github.com/caiflower/common-tools/pkg/tools"
	"github.com/caiflower/common-tools/taskx/dao/model"
	"github.com/caiflower/common-tools/taskx/executor"
	"github.com/stretchr/testify/assert"
)

func TestTask(t *testing.T) {
	myTask := NewTask("testTask").SetInput("test")
	stp1 := NewSubtask("stp1", noopExec).SetInput("stp1")
	stp2 := NewSubtask("stp2", noopExec).SetInput("stp2")
	stp3 := NewSubtask("stp3", noopExec).SetInput("stp3")
	stp4 := NewSubtask("stp4", noopExec).SetInput("stp4")
	stp5 := NewSubtask("stp5", noopExec).SetInput("stp5")
	_ = myTask.AddSubtask(stp1)
	_ = myTask.AddSubtask(stp2)
	_ = myTask.AddSubtask(stp3)
	_ = myTask.AddSubtask(stp4)
	_ = myTask.AddSubtask(stp5)

	if err := myTask.AddEdge(stp1, stp2); err != nil {
		panic(err)
	}
	if err := myTask.AddEdge(stp1, stp3); err != nil {
		panic(err)
	}
	if err := myTask.AddEdge(stp3, stp4); err != nil {
		panic(err)
	}
	if err := myTask.AddEdge(stp5, stp4); err != nil {
		panic(err)
	}
	if err := myTask.AddEdge(stp2, stp5); err != nil {
		panic(err)
	}

	_, err := myTask.Compile()
	if err != nil {
		panic(err)
	}

	assert.Equal(t, 5, myTask.Size())
	fmt.Println(myTask.Graph())

	// 逐步执行
	next := myTask.NextSubTasks()
	assert.Equal(t, 1, len(next))
	assert.Equal(t, "stp1", next[0].GetName())
	_ = myTask.UpdateSubtaskState(stp1.GetID(), NodeSucceeded)

	next = myTask.NextSubTasks()
	assert.Equal(t, 2, len(next))
	for _, s := range next {
		_ = myTask.UpdateSubtaskState(s.GetID(), NodeSucceeded)
	}

	next = myTask.NextSubTasks()
	for _, s := range next {
		_ = myTask.UpdateSubtaskState(s.GetID(), NodeSucceeded)
	}

	next = myTask.NextSubTasks()
	for _, s := range next {
		_ = myTask.UpdateSubtaskState(s.GetID(), NodeSucceeded)
	}

	next = myTask.NextSubTasks()
	assert.Equal(t, 0, len(next))
}

// simpleTaskExecutor 是一个简单的 TaskExecutor 实现（用于测试）
type simpleTaskExecutor struct {
	name string
}

func (s *simpleTaskExecutor) Name() string                      { return s.name }
func (s *simpleTaskExecutor) FinishedTask(data *TaskData) error { return nil }
func (s *simpleTaskExecutor) FailedTask(data *TaskData) error   { return nil }

// TestTask_ConvertAndRestore 测试 convert2Bean 和 initByBean 的完整生命周期
func TestTask_ConvertAndRestore(t *testing.T) {
	// 创建复杂 DAG: one -> four, two -> four, three -> five, four -> five
	myTask := NewTask("restoreTask").SetUrgent()
	one := NewSubtask("one", noopExec).SetInput("data-one")
	two := NewSubtask("two", noopExec).SetInput("data-two")
	three := NewSubtask("three", noopExec).SetInput("data-three")
	four := NewSubtask("four", noopExec).SetInput("data-four")
	five := NewSubtask("five", noopExec).SetInput("data-five")

	_ = myTask.AddSubtask(one)
	_ = myTask.AddSubtask(two)
	_ = myTask.AddSubtask(three)
	_ = myTask.AddSubtask(four)
	_ = myTask.AddSubtask(five)
	_ = myTask.AddEdge(one, four)
	_ = myTask.AddEdge(two, four)
	_ = myTask.AddEdge(three, five)
	_ = myTask.AddEdge(four, five)

	_, err := myTask.Compile()
	assert.NoError(t, err)

	// 记录原始 ID 以便后续验证
	oneID := one.GetID()
	twoID := two.GetID()
	threeID := three.GetID()
	fourID := four.GetID()
	fiveID := five.GetID()

	// Step 1: convert2Bean -> initByBean（第一次：所有 subtask 都是 pending）
	taskBean, subtaskBeans := myTask.convert2Bean()
	restoredTask := &Task{em: &executorManager{}}
	_, err = restoredTask.initByBean(taskBean, subtaskBeans)
	assert.NoError(t, err)
	assert.Equal(t, 5, restoredTask.Size())

	// 验证 root nodes (one, two, three) 是可执行的
	next := restoredTask.NextSubTasks()
	assert.Equal(t, 3, len(next))
	nextNames := map[string]bool{}
	for _, s := range next {
		nextNames[s.GetName()] = true
	}
	assert.True(t, nextNames["one"])
	assert.True(t, nextNames["two"])
	assert.True(t, nextNames["three"])

	// Step 2: 模拟 one, two, three 执行完成 -> 重建任务
	// 在 DB 中更新 subtask 状态
	subtaskMap := make(map[string]model.Subtask)
	for i := range subtaskBeans {
		subtaskMap[subtaskBeans[i].ID] = subtaskBeans[i]
	}
	beanOne := subtaskMap[oneID]
	beanOne.State = string(TaskSucceeded)
	beanOne.Output = tools.ToJson(map[string]string{"result": "one-done"})
	subtaskMap[oneID] = beanOne

	beanTwo := subtaskMap[twoID]
	beanTwo.State = string(TaskSucceeded)
	beanTwo.Output = tools.ToJson(map[string]string{"result": "two-done"})
	subtaskMap[twoID] = beanTwo

	beanThree := subtaskMap[threeID]
	beanThree.State = string(TaskSucceeded)
	beanThree.Output = tools.ToJson(map[string]string{"result": "three-done"})
	subtaskMap[threeID] = beanThree

	// 重建 beans 列表
	updatedBeans := make([]model.Subtask, 0, len(subtaskMap))
	for _, sb := range subtaskMap {
		updatedBeans = append(updatedBeans, sb)
	}

	restoredTask2 := &Task{em: &executorManager{}}
	_, err = restoredTask2.initByBean(taskBean, updatedBeans)
	assert.NoError(t, err)

	// 验证 four 现在是可执行的（one 和 two 都完成了）
	next = restoredTask2.NextSubTasks()
	assert.Equal(t, 1, len(next))
	assert.Equal(t, "four", next[0].GetName())

	// Step 3: 模拟 four 完成 -> five 应该可执行
	beanFour := subtaskMap[fourID]
	beanFour.State = string(TaskSucceeded)
	beanFour.Output = tools.ToJson(map[string]string{"result": "four-done"})
	subtaskMap[fourID] = beanFour

	updatedBeans2 := make([]model.Subtask, 0, len(subtaskMap))
	for _, sb := range subtaskMap {
		updatedBeans2 = append(updatedBeans2, sb)
	}

	restoredTask3 := &Task{em: &executorManager{}}
	_, err = restoredTask3.initByBean(taskBean, updatedBeans2)
	assert.NoError(t, err)

	next = restoredTask3.NextSubTasks()
	assert.Equal(t, 1, len(next))
	assert.Equal(t, "five", next[0].GetName())

	// Step 4: 模拟 five 完成 -> 任务应该结束
	beanFive := subtaskMap[fiveID]
	beanFive.State = string(TaskSucceeded)
	beanFive.Output = tools.ToJson(map[string]string{"result": "five-done"})
	subtaskMap[fiveID] = beanFive

	updatedBeans3 := make([]model.Subtask, 0, len(subtaskMap))
	for _, sb := range subtaskMap {
		updatedBeans3 = append(updatedBeans3, sb)
	}

	restoredTask4 := &Task{em: &executorManager{}}
	_, err = restoredTask4.initByBean(taskBean, updatedBeans3)
	assert.NoError(t, err)

	next = restoredTask4.NextSubTasks()
	assert.Equal(t, 0, len(next))
	assert.True(t, restoredTask4.IsFinished())
}

// TestTask_ProviderExecution 测试 Task + ExecutorProvider 的完整执行流程
func TestTask_ProviderExecution(t *testing.T) {
	// 定义本地执行器（使用泛型）
	echoFn := func(ctx context.Context, input map[string]string) (map[string]string, error) {
		return map[string]string{"echoed": input["value"], "from": "local"}, nil
	}
	concatFn := func(ctx context.Context, input map[string]string) (map[string]string, error) {
		return map[string]string{"result": input["a"] + "+" + input["b"]}, nil
	}

	localEcho := executor.NewLocalExecutor[map[string]string](echoFn)
	localConcat := executor.NewLocalExecutor[map[string]string](concatFn)

	// 创建 DAG: echo1, echo2 -> concat
	myTask := NewTask("providerExec")
	echo1 := NewSubtask("echo1", localEcho).SetInput(map[string]string{"value": "hello"})
	echo2 := NewSubtask("echo2", localEcho).SetInput(map[string]string{"value": "world"})
	concat := NewSubtask("concat", localConcat)

	_ = myTask.AddSubtask(echo1)
	_ = myTask.AddSubtask(echo2)
	_ = myTask.AddSubtask(concat)
	_ = myTask.AddEdge(echo1, concat)
	_ = myTask.AddEdge(echo2, concat)

	// 注册任务回调（执行器已在 AddSubtask 时自动注册）
	taskExec := &simpleTaskExecutor{name: "providerExec"}
	myTask.RegisterTaskExecutor(taskExec)

	_, err := myTask.Compile()
	assert.NoError(t, err)

	// 逐步执行并验证
	// 第一步：echo1, echo2 可执行
	next := myTask.NextSubTasks()
	assert.Equal(t, 2, len(next))

	// 执行 echo1（通过 Subtask.GetExecutor() 直接获取）
	providerEcho1 := echo1.GetExecutor()
	assert.NotNil(t, providerEcho1)
	echo1Data := &executor.TaskData{
		TaskId: "task-1",
		Input:  tools.ToJson(map[string]string{"value": "hello"}),
	}
	echo1Output, err := providerEcho1.Execute(context.Background(), echo1Data)
	assert.NoError(t, err)
	echo1Result := echo1Output.(map[string]string)
	assert.Equal(t, "hello", echo1Result["echoed"])

	// 执行 echo2
	echo2Data := &executor.TaskData{
		TaskId: "task-1",
		Input:  tools.ToJson(map[string]string{"value": "world"}),
	}
	echo2Output, err := echo2.GetExecutor().Execute(context.Background(), echo2Data)
	assert.NoError(t, err)
	echo2Result := echo2Output.(map[string]string)
	assert.Equal(t, "world", echo2Result["echoed"])

	// 更新节点状态
	_ = myTask.UpdateSubtaskState(echo1.GetID(), NodeSucceeded)
	_ = myTask.UpdateSubtaskState(echo2.GetID(), NodeSucceeded)

	// 第二步：concat 现在可执行
	next = myTask.NextSubTasks()
	assert.Equal(t, 1, len(next))
	assert.Equal(t, "concat", next[0].GetName())

	// 执行 concat
	concatData := &executor.TaskData{
		TaskId: "task-1",
		Input:  tools.ToJson(map[string]string{"a": "hello", "b": "world"}),
	}
	concatOutput, err := concat.GetExecutor().Execute(context.Background(), concatData)
	assert.NoError(t, err)
	concatResult := concatOutput.(map[string]string)
	assert.Equal(t, "hello+world", concatResult["result"])

	// 完成 concat
	_ = myTask.UpdateSubtaskState(concat.GetID(), NodeSucceeded)

	// 验证：无更多任务，且任务完成
	next = myTask.NextSubTasks()
	assert.Equal(t, 0, len(next))
	assert.True(t, myTask.IsFinished())
}
