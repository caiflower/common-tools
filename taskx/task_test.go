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
	taskBean, subtaskBeans, _ := myTask.convert2Bean()
	restoredTask := &Task{em: &executorManager{}}
	_, err = restoredTask.initByBean(taskBean, subtaskBeans, nil)
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
	_, err = restoredTask2.initByBean(taskBean, updatedBeans, nil)
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
	_, err = restoredTask3.initByBean(taskBean, updatedBeans2, nil)
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
	_, err = restoredTask4.initByBean(taskBean, updatedBeans3, nil)
	assert.NoError(t, err)

	next = restoredTask4.NextSubTasks()
	assert.Equal(t, 0, len(next))
	assert.True(t, restoredTask4.IsFinished())
}

// TestTask_ProviderExecution 测试 Task + ExecutorProvider 的完整执行流程

func TestTask_EdgeTypeInputPrecompute(t *testing.T) {
	// 测试三种边类型在 convert2Bean → initByBean → 模拟 computeInput 全链路中的行为
	task := NewTask("edge-input-test")

	a := NewSubtask("a", noopExec).SetInput(`{"from":"a"}`)
	b := NewSubtask("b", noopExec).SetInput(`{"from":"b"}`)
	c := NewSubtask("c", noopExec).SetInput(`{"from":"c"}`)
	d := NewSubtask("d", noopExec).SetInput(`{"from":"d"}`)

	_ = task.AddSubtask(a)
	_ = task.AddSubtask(b)
	_ = task.AddSubtask(c)
	_ = task.AddSubtask(d)
	_ = task.AddControlEdge(a, b) // control-only
	_ = task.AddDataEdge(a, c)    // data-only
	_ = task.AddEdge(a, d)        // control+data

	_, err := task.Compile()
	assert.NoError(t, err)

	// 模拟 A 执行完成，输出经过 Output 结构序列化
	aOutput := map[string]any{"result": "from-a"}
	aOutputJSON := tools.ToJson(&Output{Output: tools.ToJson(aOutput)})
	task.subtaskMap[a.GetID()].subtask.Output = aOutputJSON
	task.subtaskMap[a.GetID()].subtask.State = string(TaskSucceeded)

	// convert2Bean → initByBean 模拟 DB 重建
	taskBean, subtaskBeans, edgeBeans := task.convert2Bean()

	// 验证 edgeBeans 中的边类型
	edgeTypeMap := make(map[string]string)
	for _, e := range edgeBeans {
		key := e.FromSubtaskID + "->" + e.ToSubtaskID
		edgeTypeMap[key] = e.EdgeType
	}

	// 找到从 a 出发的边（通过 subtaskName 查找 ID）
	var aID, bID, cID, dID string
	for _, s := range subtaskBeans {
		switch s.TaskName {
		case "a":
			aID = s.ID
		case "b":
			bID = s.ID
		case "c":
			cID = s.ID
		case "d":
			dID = s.ID
		}
	}

	assert.Equal(t, "control", edgeTypeMap[aID+"->"+bID], "a->b should be control")
	assert.Equal(t, "data", edgeTypeMap[aID+"->"+cID], "a->c should be data")
	assert.Equal(t, "control+data", edgeTypeMap[aID+"->"+dID], "a->d should be control+data")

	// initByBean 重建 Task（传 edges 走精确恢复路径）
	restoredTask := &Task{em: &executorManager{}}
	restoredTask, err = restoredTask.initByBean(taskBean, subtaskBeans, edgeBeans)
	assert.NoError(t, err)

	// 验证重建后的邻接表
	assert.Len(t, restoredTask.dag.controlPred[bID], 1, "B should have 1 control pred")
	assert.Len(t, restoredTask.dag.dataPred[bID], 0, "B should have 0 data pred")
	assert.Len(t, restoredTask.dag.controlPred[cID], 0, "C should have 0 control pred")
	assert.Len(t, restoredTask.dag.dataPred[cID], 1, "C should have 1 data pred")
	assert.Len(t, restoredTask.dag.dataPred[dID], 1, "D should have 1 data pred")

	// 模拟 dispatcher 的 computeInput 逻辑
	runnings := []*model.Subtask{
		&subtaskBeans[0], // 用任意 bean 占位，实际用 restoredTask.subtaskMap
	}
	_ = runnings

	// 用 restoredTask 的 subtaskMap 模拟 computeInput
	for name, id := range map[string]string{"b": bID, "c": cID, "d": dID} {
		dataPreds := restoredTask.dag.dataPred[id]
		subtask := restoredTask.subtaskMap[id]
		if len(dataPreds) > 0 {
			var computedInput string
			preOutputs := make(map[string]string)
			for _, predID := range dataPreds {
				if s, ok := restoredTask.subtaskMap[predID]; ok {
					var output Output
					if err := tools.Unmarshal([]byte(s.subtask.Output), &output); err == nil {
						preOutputs[s.GetName()] = output.Output
					}
				}
			}
			if len(preOutputs) == 1 {
				for _, v := range preOutputs {
					computedInput = v
					break
				}
			}
			if computedInput != "" {
				subtask.subtask.Input = computedInput
			}
		}
		switch name {
		case "b":
			// B 只有 ControlEdge → Input 保持原值
			assert.Equal(t, `{"from":"b"}`, subtask.subtask.Input, "B should keep original input")
		case "c":
			// C 有 DataEdge → Input 应为 A 的输出（经 Output 结构解包后）
			assert.Equal(t, `{"result":"from-a"}`, subtask.subtask.Input, "C should receive A's output")
		case "d":
			// D 有 ControlAndDataEdge → Input 应为 A 的输出
			assert.Equal(t, `{"result":"from-a"}`, subtask.subtask.Input, "D should receive A's output")
		}
	}
}

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

// buildRollbackTask creates a compiled Task with subtask beans ready for rollback testing.
// It goes through the convert2Bean → initByBean cycle so PreSubtaskID is populated.
// Returns the restored task and a name→ID mapping.
func buildRollbackTask(taskName string, subtasks []*Subtask, edges [][2]*Subtask) (task *Task, nameToID map[string]string) {
	t := NewTask(taskName)
	for _, s := range subtasks {
		_ = t.AddSubtask(s)
	}
	for _, e := range edges {
		_ = t.AddControlEdge(e[0], e[1])
	}
	_, _ = t.Compile()

	taskBean, subtaskBeans, _ := t.convert2Bean()

	// Set all subtasks to succeeded state with rollback_pending (rollbackable)
	for i := range subtaskBeans {
		subtaskBeans[i].State = string(TaskSucceeded)
		subtaskBeans[i].Rollback = string(RollbackPending)
	}

	restored := &Task{em: &executorManager{}}
	restored, _ = restored.initByBean(taskBean, subtaskBeans, nil)

	nameToID = make(map[string]string)
	for id, s := range restored.subtaskMap {
		nameToID[s.GetName()] = id
	}
	return restored, nameToID
}

// TestTask_LeafRollbackSubtasks_LinearChain tests A→B→C: only C (the leaf) should be returned
func TestTask_LeafRollbackSubtasks_LinearChain(t *testing.T) {
	a := NewSubtask("a", noopExec)
	b := NewSubtask("b", noopExec)
	c := NewSubtask("c", noopExec)

	task, ids := buildRollbackTask("linear-rb", []*Subtask{a, b, c}, [][2]*Subtask{{a, b}, {b, c}})

	leaves := task.LeafRollbackSubtasks()
	assert.Equal(t, 1, len(leaves))
	assert.Equal(t, ids["c"], leaves[0].ID, "C should be the only leaf")
}

// TestTask_LeafRollbackSubtasks_Diamond tests A→C, B→C, C→D: D is the only leaf
func TestTask_LeafRollbackSubtasks_Diamond(t *testing.T) {
	a := NewSubtask("a", noopExec)
	b := NewSubtask("b", noopExec)
	c := NewSubtask("c", noopExec)
	d := NewSubtask("d", noopExec)

	task, ids := buildRollbackTask("diamond-rb",
		[]*Subtask{a, b, c, d},
		[][2]*Subtask{{a, c}, {b, c}, {c, d}},
	)

	leaves := task.LeafRollbackSubtasks()
	assert.Equal(t, 1, len(leaves))
	assert.Equal(t, ids["d"], leaves[0].ID, "D should be the only leaf")
}

// TestTask_LeafRollbackSubtasks_PartialRollback tests A→B→C where C already finished rollback.
// B should become a leaf alongside C.
func TestTask_LeafRollbackSubtasks_PartialRollback(t *testing.T) {
	a := NewSubtask("a", noopExec)
	b := NewSubtask("b", noopExec)
	c := NewSubtask("c", noopExec)

	task, ids := buildRollbackTask("partial-rb", []*Subtask{a, b, c}, [][2]*Subtask{{a, b}, {b, c}})

	// Simulate C already completed rollback
	task.subtaskMap[ids["c"]].subtask.Rollback = string(RollbackSucceeded)

	leaves := task.LeafRollbackSubtasks()
	leafNames := make(map[string]bool)
	for _, l := range leaves {
		leafNames[task.subtaskMap[l.ID].GetName()] = true
	}
	assert.True(t, leafNames["b"], "B should be a leaf (dependent C finished rollback)")
	assert.True(t, leafNames["c"], "C should be a leaf (no forward dependents)")
}

// TestTask_LeafRollbackSubtasks_Empty tests no rollbackable subtasks → empty result
func TestTask_LeafRollbackSubtasks_Empty(t *testing.T) {
	a := NewSubtask("a", noopExec)
	b := NewSubtask("b", noopExec)

	task := NewTask("empty-rb")
	_ = task.AddSubtask(a)
	_ = task.AddSubtask(b)
	_ = task.AddControlEdge(a, b)
	_, _ = task.Compile()

	taskBean, subtaskBeans, _ := task.convert2Bean()
	// All subtasks pending → no rollbackable
	for i := range subtaskBeans {
		subtaskBeans[i].State = string(TaskPending)
		subtaskBeans[i].Rollback = string(NoneRollback)
	}
	restored := &Task{em: &executorManager{}}
	restored, _ = restored.initByBean(taskBean, subtaskBeans, nil)

	leaves := restored.LeafRollbackSubtasks()
	assert.Empty(t, leaves)
}

// TestTask_LeafRollbackSubtasks_MixedDependents tests A→B, A→C where B is rollbackable but C is not.
// B should be a leaf. A is NOT a leaf because B (rollbackable, not done) depends on it.
func TestTask_LeafRollbackSubtasks_MixedDependents(t *testing.T) {
	a := NewSubtask("a", noopExec)
	b := NewSubtask("b", noopExec)
	c := NewSubtask("c", noopExec)

	task := NewTask("mixed-rb")
	_ = task.AddSubtask(a)
	_ = task.AddSubtask(b)
	_ = task.AddSubtask(c)
	_ = task.AddControlEdge(a, b)
	_ = task.AddControlEdge(a, c)
	_, _ = task.Compile()

	taskBean, subtaskBeans, _ := task.convert2Bean()
	for i := range subtaskBeans {
		switch subtaskBeans[i].TaskName {
		case "a":
			subtaskBeans[i].State = string(TaskSucceeded)
			subtaskBeans[i].Rollback = string(RollbackPending)
		case "b":
			subtaskBeans[i].State = string(TaskSucceeded)
			subtaskBeans[i].Rollback = string(RollbackPending)
		case "c":
			// C is pending → not rollbackable (not succeeded/failed)
			subtaskBeans[i].State = string(TaskPending)
			subtaskBeans[i].Rollback = string(NoneRollback)
		}
	}

	restored := &Task{em: &executorManager{}}
	restored, _ = restored.initByBean(taskBean, subtaskBeans, nil)

	leaves := restored.LeafRollbackSubtasks()
	leafNames := make(map[string]bool)
	for _, l := range leaves {
		leafNames[restored.subtaskMap[l.ID].GetName()] = true
	}
	assert.True(t, leafNames["b"], "B should be a leaf (no forward dependents)")
	assert.False(t, leafNames["a"], "A should NOT be a leaf (B is rollbackable and not done)")
	assert.False(t, leafNames["c"], "C should NOT appear (not rollbackable)")
}

// TestTask_LeafRollbackSubtasks_FailedSubtask tests A→B where B failed.
// Both A (succeeded) and B (failed) are rollbackable; B is the leaf.
func TestTask_LeafRollbackSubtasks_FailedSubtask(t *testing.T) {
	a := NewSubtask("a", noopExec)
	b := NewSubtask("b", noopExec)

	task := NewTask("failed-rb")
	_ = task.AddSubtask(a)
	_ = task.AddSubtask(b)
	_ = task.AddControlEdge(a, b)
	_, _ = task.Compile()

	taskBean, subtaskBeans, _ := task.convert2Bean()
	nameToID := make(map[string]string)
	for i := range subtaskBeans {
		nameToID[subtaskBeans[i].TaskName] = subtaskBeans[i].ID
		switch subtaskBeans[i].TaskName {
		case "a":
			subtaskBeans[i].State = string(TaskSucceeded)
			subtaskBeans[i].Rollback = string(RollbackPending)
		case "b":
			subtaskBeans[i].State = string(TaskFailed)
			subtaskBeans[i].Rollback = string(RollbackPending)
		}
	}

	restored := &Task{em: &executorManager{}}
	restored, _ = restored.initByBean(taskBean, subtaskBeans, nil)

	leaves := restored.LeafRollbackSubtasks()
	assert.Equal(t, 1, len(leaves))
	assert.Equal(t, nameToID["b"], leaves[0].ID, "B (failed) should be the leaf")
}
