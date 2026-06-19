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
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/caiflower/common-tools/pkg/tools"
	"github.com/caiflower/common-tools/taskx/executor"
	"github.com/stretchr/testify/assert"
)

// noopExec 用于不需要实际执行的 DAG 结构测试
var noopExec = executor.NewLocalExecutor(func(ctx context.Context, input map[string]any) (map[string]any, error) {
	return input, nil
})

// ===== dagGraph 单元测试 =====

func TestDAGGraph_AddNode(t *testing.T) {
	g := NewDAGGraph()

	// 正常添加
	err := g.AddNode("a", AllPredecessor)
	assert.Nil(t, err)
	assert.NotNil(t, g.GetNode("a"))
	assert.Equal(t, NodePending, g.GetNode("a").state)

	// 重复添加
	err = g.AddNode("a", AllPredecessor)
	assert.NotNil(t, err)

	// 空key
	err = g.AddNode("", AllPredecessor)
	assert.NotNil(t, err)
}

func TestDAGGraph_AddEdge(t *testing.T) {
	g := NewDAGGraph()
	_ = g.AddNode("a", AllPredecessor)
	_ = g.AddNode("b", AllPredecessor)
	_ = g.AddNode("c", AllPredecessor)

	// 添加控制边
	err := g.AddEdge("a", "b", ControlEdge)
	assert.Nil(t, err)
	assert.Contains(t, g.controlAdj["a"], "b")
	assert.Contains(t, g.controlPred["b"], "a")

	// 添加数据边
	err = g.AddEdge("b", "c", DataEdge)
	assert.Nil(t, err)
	assert.Contains(t, g.dataAdj["b"], "c")
	assert.Contains(t, g.dataPred["c"], "b")

	// 添加控制+数据边
	err = g.AddEdge("a", "c", ControlAndDataEdge)
	assert.Nil(t, err)
	assert.Contains(t, g.controlAdj["a"], "c")
	assert.Contains(t, g.dataAdj["a"], "c")

	// 不存在的节点
	err = g.AddEdge("a", "d", ControlEdge)
	assert.NotNil(t, err)
}

func TestDAGGraph_TopologicalSort(t *testing.T) {
	g := NewDAGGraph()
	_ = g.AddNode("a", AllPredecessor)
	_ = g.AddNode("b", AllPredecessor)
	_ = g.AddNode("c", AllPredecessor)
	_ = g.AddEdge("a", "b", ControlEdge)
	_ = g.AddEdge("b", "c", ControlEdge)

	result, err := g.TopologicalSort()
	assert.Nil(t, err)
	assert.Equal(t, 3, len(result))
	// a 必须在 b 之前，b 必须在 c 之前
	assert.True(t, indexOf(result, "a") < indexOf(result, "b"))
	assert.True(t, indexOf(result, "b") < indexOf(result, "c"))
}

func TestDAGGraph_TopologicalSort_WithCycle(t *testing.T) {
	g := NewDAGGraph()
	_ = g.AddNode("a", AllPredecessor)
	_ = g.AddNode("b", AllPredecessor)
	_ = g.AddNode("c", AllPredecessor)
	_ = g.AddEdge("a", "b", ControlEdge)
	_ = g.AddEdge("b", "c", ControlEdge)
	_ = g.AddEdge("c", "a", ControlEdge) // 环

	_, err := g.TopologicalSort()
	assert.NotNil(t, err)
}

func TestDAGGraph_UpdateNodeState(t *testing.T) {
	g := NewDAGGraph()
	_ = g.AddNode("a", AllPredecessor)

	err := g.UpdateNodeState("a", NodeRunning)
	assert.Nil(t, err)
	assert.Equal(t, NodeRunning, g.GetNode("a").state)

	err = g.UpdateNodeState("a", NodeSucceeded)
	assert.Nil(t, err)
	assert.Equal(t, NodeSucceeded, g.GetNode("a").state)

	// 不存在的节点
	err = g.UpdateNodeState("z", NodeSucceeded)
	assert.NotNil(t, err)
}

func TestDAGGraph_GetStartEndNodes(t *testing.T) {
	g := NewDAGGraph()
	_ = g.AddNode("a", AllPredecessor)
	_ = g.AddNode("b", AllPredecessor)
	_ = g.AddNode("c", AllPredecessor)
	_ = g.AddEdge("a", "b", ControlEdge)
	_ = g.AddEdge("b", "c", ControlEdge)

	startNodes := g.GetStartNodes()
	assert.Equal(t, 1, len(startNodes))
	assert.Contains(t, startNodes, "a")

	endNodes := g.GetEndNodes()
	assert.Equal(t, 1, len(endNodes))
	assert.Contains(t, endNodes, "c")
}

func TestDAGGraph_CompiledImmutable(t *testing.T) {
	g := NewDAGGraph()
	_ = g.AddNode("a", AllPredecessor)
	_ = g.AddNode("b", AllPredecessor)
	_ = g.AddEdge("a", "b", ControlEdge)

	_, err := g.Compile()
	assert.Nil(t, err)

	// 编译后不能再添加节点
	err = g.AddNode("c", AllPredecessor)
	assert.Equal(t, ErrGraphCompiled, err)

	// 编译后不能再添加边
	err = g.AddEdge("a", "b", ControlEdge)
	assert.Equal(t, ErrGraphCompiled, err)
}

// ===== dagChannel 单元测试 =====

func TestDAGChannel_AllPredecessorReady(t *testing.T) {
	ch := newDAGChannel("c", []string{"a", "b"}, []string{"a"}, AllPredecessor)

	// 初始状态：未就绪
	assert.False(t, ch.isReady())

	// a 完成
	ch.reportDependencies([]string{"a"})
	ch.reportValues(map[string]any{"a": "data-a"})
	assert.False(t, ch.isReady()) // b 还未完成

	// b 完成
	ch.reportDependencies([]string{"b"})
	assert.True(t, ch.isReady())
	assert.True(t, ch.isDataReady())

	// 获取数据
	data, ready, err := ch.get()
	assert.Nil(t, err)
	assert.True(t, ready)
	assert.Equal(t, "data-a", data)
}

func TestDAGChannel_AnyPredecessorReady(t *testing.T) {
	ch := newDAGChannel("c", []string{"a", "b"}, []string{"a", "b"}, AnyPredecessor)

	// 初始状态：未就绪
	assert.False(t, ch.isReady())

	// a 完成
	ch.reportDependencies([]string{"a"})
	ch.reportValues(map[string]any{"a": "data-a"})
	assert.True(t, ch.isReady()) // 任一前驱完成即就绪
}

func TestDAGChannel_SkipPropagation(t *testing.T) {
	ch := newDAGChannel("c", []string{"a", "b"}, []string{"a"}, AllPredecessor)

	// a 跳过
	skipped := ch.reportSkip([]string{"a"})
	assert.False(t, skipped) // b 还未跳过，c 不跳过

	// b 也跳过
	skipped = ch.reportSkip([]string{"b"})
	assert.True(t, skipped) // 所有前驱跳过，c 也跳过
	assert.True(t, ch.isSkipped())
}

func TestDAGChannel_MultipleDataMerge(t *testing.T) {
	ch := newDAGChannel("c", []string{"a", "b"}, []string{"a", "b"}, AllPredecessor)

	ch.reportDependencies([]string{"a"})
	ch.reportValues(map[string]any{"a": map[string]any{"name": "foo"}})
	ch.reportDependencies([]string{"b"})
	ch.reportValues(map[string]any{"b": map[string]any{"age": 20}})

	data, ready, err := ch.get()
	assert.Nil(t, err)
	assert.True(t, ready)

	merged, ok := data.(map[string]any)
	assert.True(t, ok)
	assert.Equal(t, 2, len(merged))
}

func TestDAGChannel_Reset(t *testing.T) {
	ch := newDAGChannel("c", []string{"a"}, []string{"a"}, AllPredecessor)

	ch.reportDependencies([]string{"a"})
	ch.reportValues(map[string]any{"a": "data"})
	assert.True(t, ch.isReady())

	ch.reset()
	assert.False(t, ch.isReady())
	assert.False(t, ch.isSkipped())
}

// ===== Compile 编译测试 =====

func TestCompile_ValidDAG(t *testing.T) {
	g := NewDAGGraph()
	_ = g.AddNode("a", AllPredecessor)
	_ = g.AddNode("b", AllPredecessor)
	_ = g.AddNode("c", AllPredecessor)
	_ = g.AddEdge("a", "b", ControlAndDataEdge)
	_ = g.AddEdge("b", "c", ControlAndDataEdge)

	compiled, err := g.Compile()
	assert.Nil(t, err)
	assert.NotNil(t, compiled)
	assert.Equal(t, 1, len(compiled.GetStartNodes()))
	assert.Equal(t, 1, len(compiled.GetEndNodes()))
	assert.Equal(t, 3, len(compiled.GetTopoOrder()))
}

func TestCompile_CycleDetection(t *testing.T) {
	g := NewDAGGraph()
	_ = g.AddNode("a", AllPredecessor)
	_ = g.AddNode("b", AllPredecessor)
	_ = g.AddNode("c", AllPredecessor)
	_ = g.AddEdge("a", "b", ControlEdge)
	_ = g.AddEdge("b", "c", ControlEdge)
	_ = g.AddEdge("c", "a", ControlEdge)

	_, err := g.Compile()
	assert.NotNil(t, err)
	assert.True(t, strings.Contains(err.Error(), "loop") || strings.Contains(err.Error(), "cycle"))
}

func TestCompile_NoStartNode(t *testing.T) {
	g := NewDAGGraph()
	_ = g.AddNode("a", AllPredecessor)
	_ = g.AddNode("b", AllPredecessor)
	_ = g.AddEdge("a", "b", ControlEdge)
	_ = g.AddEdge("b", "a", ControlEdge) // 互依赖，无起始节点

	_, err := g.Compile()
	assert.NotNil(t, err)
}

func TestCompile_InvalidBranchEndNode(t *testing.T) {
	g := NewDAGGraph()
	_ = g.AddNode("a", AllPredecessor)
	_ = g.AddNode("b", AllPredecessor)
	_ = g.AddEdge("a", "b", ControlEdge)

	err := g.AddBranch("a", &Branch{
		Condition: func(ctx interface{}, input any) (string, error) { return "c", nil },
		EndNodes:  map[string]bool{"c": true}, // c 不存在
	})
	assert.Nil(t, err)

	_, err = g.Compile()
	assert.NotNil(t, err)
	assert.True(t, strings.Contains(err.Error(), "invalid end node"))
}

// ===== 控制/数据依赖分离集成测试 =====

func TestTask_ControlDataEdgeSeparation(t *testing.T) {
	task := NewTask("control-data-test")

	a := NewSubtask("a", noopExec).SetInput("data-a")
	b := NewSubtask("b", noopExec)
	c := NewSubtask("c", noopExec)

	_ = task.AddSubtask(a)
	_ = task.AddSubtask(b)
	_ = task.AddSubtask(c)

	// a --control--> b: b 等待 a 完成但不接收 a 的数据
	_ = task.AddControlEdge(a, b)
	// a --data--> c: c 接收 a 的数据，但 c 不因 a 的完成而阻塞
	// 注意：c 有数据前驱但没有控制前驱，数据前驱也会阻塞执行
	// 所以 c 需要等待 a 完成后才能获取数据
	_ = task.AddDataEdge(a, c)

	_, err := task.Compile()
	assert.Nil(t, err)

	// 初始：只有 a 可执行（c 有数据前驱 a 未完成）
	next := task.NextSubTasks()
	assert.Equal(t, 1, len(next))
	assert.Equal(t, "a", next[0].GetName())

	// a 完成后，b 的控制依赖满足，c 的数据依赖也满足
	_ = task.UpdateSubtaskState(a.GetID(), NodeSucceeded)
	next = task.NextSubTasks()
	assert.Equal(t, 2, len(next)) // b 和 c 都可执行
}

func TestTask_EdgeTypeDataFlow(t *testing.T) {
	task := NewTask("edge-data-flow-test")

	a := NewSubtask("a", noopExec).SetInput(`{"from":"a"}`)
	b := NewSubtask("b", noopExec).SetInput(`{"from":"b"}`)
	c := NewSubtask("c", noopExec).SetInput(`{"from":"c"}`)
	d := NewSubtask("d", noopExec).SetInput(`{"from":"d"}`)

	_ = task.AddSubtask(a)
	_ = task.AddSubtask(b)
	_ = task.AddSubtask(c)
	_ = task.AddSubtask(d)
	// a --control--> b：b 等 a 完成但不应接收 a 的数据
	_ = task.AddControlEdge(a, b)
	// a --data--> c：c 应接收 a 的数据
	_ = task.AddDataEdge(a, c)
	// a --control+data--> d：d 应接收 a 的数据
	_ = task.AddEdge(a, d)

	_, err := task.Compile()
	assert.Nil(t, err)

	// 验证前驱表
	// B: 有 control 前驱，无 data 前驱
	assert.Len(t, task.dag.controlPred[b.GetID()], 1)
	assert.Len(t, task.dag.dataPred[b.GetID()], 0)
	// C: 无 control 前驱，有 data 前驱
	assert.Len(t, task.dag.controlPred[c.GetID()], 0)
	assert.Len(t, task.dag.dataPred[c.GetID()], 1)
	// D: 有 control 前驱，有 data 前驱
	assert.Len(t, task.dag.controlPred[d.GetID()], 1)
	assert.Len(t, task.dag.dataPred[d.GetID()], 1)

	// 模拟 A 完成并产生输出
	aOutput := `{"result":"from-a"}`
	task.subtaskMap[a.GetID()].subtask.Output = aOutput

	// 模拟 dispatcher 的 Input 预计算逻辑
	for _, key := range []string{b.GetID(), c.GetID(), d.GetID()} {
		subtask := task.subtaskMap[key]
		dataPreds := task.dag.dataPred[key]
		if len(dataPreds) > 0 {
			preOutputs := make(map[string]string)
			for _, predID := range dataPreds {
				if s, ok := task.subtaskMap[predID]; ok {
					preOutputs[s.GetName()] = s.subtask.Output
				}
			}
			if len(preOutputs) == 1 {
				for _, v := range preOutputs {
					subtask.subtask.Input = v
					break
				}
			} else {
				merged := make(map[string]any, len(preOutputs))
				for k, v := range preOutputs {
					var parsed any
					if err := tools.Unmarshal([]byte(v), &parsed); err != nil {
						merged[k] = v
					} else {
						merged[k] = parsed
					}
				}
				if bytes, err := tools.ToByte(merged); err == nil {
					subtask.subtask.Input = string(bytes)
				}
			}
		}
	}

	// B 只有 ControlEdge -> Input 保持原值
	assert.Equal(t, `{"from":"b"}`, task.subtaskMap[b.GetID()].subtask.Input)
	// C 有 DataEdge -> Input 应为 A 的输出
	assert.Equal(t, aOutput, task.subtaskMap[c.GetID()].subtask.Input)
	// D 有 ControlAndDataEdge -> Input 应为 A 的输出
	assert.Equal(t, aOutput, task.subtaskMap[d.GetID()].subtask.Input)
}

func TestTask_AddEdge(t *testing.T) {
	task := NewTask("add-edge-test")

	a := NewSubtask("a", noopExec)
	b := NewSubtask("b", noopExec)

	_ = task.AddSubtask(a)
	_ = task.AddSubtask(b)

	// AddEdge 同时添加控制和数据依赖
	_ = task.AddEdge(a, b)

	_, err := task.Compile()
	assert.Nil(t, err)

	// a 先执行
	next := task.NextSubTasks()
	assert.Equal(t, 1, len(next))
	assert.Equal(t, "a", next[0].GetName())

	// a 完成后 b 可执行
	_ = task.UpdateSubtaskState(a.GetID(), NodeSucceeded)
	next = task.NextSubTasks()
	assert.Equal(t, 1, len(next))
	assert.Equal(t, "b", next[0].GetName())
}

// ===== 条件分支集成测试 =====

func TestTask_Branch(t *testing.T) {
	task := NewTask("branch-test")

	start := NewSubtask("start", noopExec)
	pathA := NewSubtask("pathA", noopExec)
	pathB := NewSubtask("pathB", noopExec)
	end := NewSubtask("end", noopExec)

	_ = task.AddSubtask(start)
	_ = task.AddSubtask(pathA)
	_ = task.AddSubtask(pathB)
	_ = task.AddSubtask(end)

	_ = task.AddEdge(start, pathA)
	_ = task.AddEdge(start, pathB)
	_ = task.AddEdge(pathA, end)
	_ = task.AddEdge(pathB, end)

	// 添加分支：选择 pathA（使用子任务 ID 作为 endNodes）
	_ = task.AddBranch(start, &Branch{
		Condition: func(ctx interface{}, input any) (string, error) {
			return pathA.GetID(), nil
		},
		EndNodes: map[string]bool{pathA.GetID(): true, pathB.GetID(): true},
	})

	_, err := task.Compile()
	assert.Nil(t, err)

	// start 先执行
	next := task.NextSubTasks()
	assert.Equal(t, 1, len(next))
	assert.Equal(t, "start", next[0].GetName())

	// start 完成，分支选择 pathA
	_ = task.UpdateSubtaskState(start.GetID(), NodeSucceeded)

	// 手动跳过 pathB（模拟分支选择逻辑）
	_ = task.SkipSubtask(pathB.GetID())

	next = task.NextSubTasks()
	assert.Equal(t, 1, len(next))
	assert.Equal(t, "pathA", next[0].GetName())

	// pathA 完成后，end 可执行（pathB 已跳过，不阻塞）
	_ = task.UpdateSubtaskState(pathA.GetID(), NodeSucceeded)
	next = task.NextSubTasks()
	assert.Equal(t, 1, len(next))
	assert.Equal(t, "end", next[0].GetName())
}

// TestTask_BranchWithNames 验证分支选择支持使用子任务 name（而非 ID）
// EndNodes 和 Condition 返回值都使用 name，内部自动解析为 ID
func TestTask_BranchWithNames(t *testing.T) {
	task := NewTask("branch-name-test")

	start := NewSubtask("start", noopExec)
	pathA := NewSubtask("pathA", noopExec)
	pathB := NewSubtask("pathB", noopExec)
	end := NewSubtask("end", noopExec)

	_ = task.AddSubtask(start)
	_ = task.AddSubtask(pathA)
	_ = task.AddSubtask(pathB)
	_ = task.AddSubtask(end)

	_ = task.AddEdge(start, pathA)
	_ = task.AddEdge(start, pathB)
	_ = task.AddEdge(pathA, end)
	_ = task.AddEdge(pathB, end)

	// 使用 name（而非 GetID()）设置分支
	_ = task.AddBranch(start, &Branch{
		Condition: func(ctx interface{}, input any) (string, error) {
			return "pathA", nil // return name instead of ID
		},
		EndNodes: map[string]bool{"pathA": true, "pathB": true}, // use names
	})

	_, err := task.Compile()
	assert.Nil(t, err)

	// start 先执行
	next := task.NextSubTasks()
	assert.Equal(t, 1, len(next))
	assert.Equal(t, "start", next[0].GetName())

	// start 完成
	_ = task.UpdateSubtaskState(start.GetID(), NodeSucceeded)

	// 手动跳过 pathB（模拟分支选择逻辑）
	_ = task.SkipSubtask(pathB.GetID())

	// pathA 可执行
	next = task.NextSubTasks()
	assert.Equal(t, 1, len(next))
	assert.Equal(t, "pathA", next[0].GetName())

	// pathA 完成后，end 可执行
	_ = task.UpdateSubtaskState(pathA.GetID(), NodeSucceeded)
	next = task.NextSubTasks()
	assert.Equal(t, 1, len(next))
	assert.Equal(t, "end", next[0].GetName())
}

// TestTask_BranchMixedNameAndID 验证分支选择同时支持 name 和 ID 混用
func TestTask_BranchMixedNameAndID(t *testing.T) {
	task := NewTask("branch-mixed-test")

	start := NewSubtask("start", noopExec)
	pathA := NewSubtask("pathA", noopExec)
	pathB := NewSubtask("pathB", noopExec)
	end := NewSubtask("end", noopExec)

	_ = task.AddSubtask(start)
	_ = task.AddSubtask(pathA)
	_ = task.AddSubtask(pathB)
	_ = task.AddSubtask(end)

	_ = task.AddEdge(start, pathA)
	_ = task.AddEdge(start, pathB)
	_ = task.AddEdge(pathA, end)
	_ = task.AddEdge(pathB, end)

	// 混合使用：EndNodes 用 name，Condition 返回 ID
	_ = task.AddBranch(start, &Branch{
		Condition: func(ctx interface{}, input any) (string, error) {
			return pathA.GetID(), nil // return ID
		},
		EndNodes: map[string]bool{"pathA": true, pathB.GetID(): true}, // mix name and ID
	})

	_, err := task.Compile()
	assert.Nil(t, err)

	// start 先执行
	next := task.NextSubTasks()
	assert.Equal(t, 1, len(next))
	assert.Equal(t, "start", next[0].GetName())
}

// TestTask_BranchConditionProviderWithName 验证 ConditionProvider 返回 name 时自动解析为 ID
func TestTask_BranchConditionProviderWithName(t *testing.T) {
	task := NewTask("branch-provider-name-test")

	start := NewSubtask("start", noopExec)
	pathA := NewSubtask("pathA", noopExec)
	pathB := NewSubtask("pathB", noopExec)
	end := NewSubtask("end", noopExec)

	_ = task.AddSubtask(start)
	_ = task.AddSubtask(pathA)
	_ = task.AddSubtask(pathB)
	_ = task.AddSubtask(end)

	_ = task.AddEdge(start, pathA)
	_ = task.AddEdge(start, pathB)
	_ = task.AddEdge(pathA, end)
	_ = task.AddEdge(pathB, end)

	// ConditionProvider 返回 name（而非 ID）
	branchProvider := executor.NewLocalExecutor(func(ctx context.Context, input map[string]any) (string, error) {
		return "pathA", nil // return name instead of ID
	})
	_ = task.AddBranch(start, NewBranch(branchProvider, map[string]bool{"pathA": true, "pathB": true}))

	_, err := task.Compile()
	assert.Nil(t, err)

	// start 先执行
	next := task.NextSubTasks()
	assert.Equal(t, 1, len(next))
	assert.Equal(t, "start", next[0].GetName())
}

// ===== Skip 传播集成测试 =====

func TestTask_SkipPropagation(t *testing.T) {
	task := NewTask("skip-test")

	a := NewSubtask("a", noopExec)
	b := NewSubtask("b", noopExec)
	c := NewSubtask("c", noopExec)

	_ = task.AddSubtask(a)
	_ = task.AddSubtask(b)
	_ = task.AddSubtask(c)

	_ = task.AddEdge(a, b)
	_ = task.AddEdge(b, c)

	_, err := task.Compile()
	assert.Nil(t, err)

	// 跳过 a
	_ = task.SkipSubtask(a.GetID())

	// a 跳过后，b 的唯一控制前驱跳过，b 也应跳过
	// 但需要通过 dagChannel 传播，这里需要手动触发
	// 因为 SkipSubtask 内部会调用 reportSkip
	// 检查 b 的通道状态
	channel := task.getCompiled().GetChannel(b.GetID())
	assert.True(t, channel.isSkipped())

	// b 跳过后，c 也应跳过
	_ = task.SkipSubtask(b.GetID())
	channel = task.getCompiled().GetChannel(c.GetID())
	assert.True(t, channel.isSkipped())
}

func TestTask_SkipDoesNotBlockConverge(t *testing.T) {
	task := NewTask("skip-converge-test")

	a := NewSubtask("a", noopExec)
	b := NewSubtask("b", noopExec)
	c := NewSubtask("c", noopExec)

	_ = task.AddSubtask(a)
	_ = task.AddSubtask(b)
	_ = task.AddSubtask(c)

	_ = task.AddEdge(a, c)
	_ = task.AddEdge(b, c)

	_, err := task.Compile()
	assert.Nil(t, err)

	// a 完成，b 跳过
	_ = task.UpdateSubtaskState(a.GetID(), NodeSucceeded)
	_ = task.SkipSubtask(b.GetID())

	// c 不应被跳过（a 已完成，b 跳过，但不是所有前驱都跳过）
	channel := task.getCompiled().GetChannel(c.GetID())
	assert.False(t, channel.isSkipped())
	assert.True(t, channel.isReady())
}

// ===== 字段映射集成测试 =====

func TestTask_FieldMapping(t *testing.T) {
	task := NewTask("field-mapping-test")

	a := NewSubtask("a", noopExec)
	b := NewSubtask("b", noopExec)

	_ = task.AddSubtask(a)
	_ = task.AddSubtask(b)

	// 添加带字段映射的数据边
	_ = task.AddDataEdge(a, b, &FieldMapping{
		SourceField: "result.name",
		TargetField: "displayName",
	})
	// 同时添加控制边
	_ = task.AddControlEdge(a, b)

	_, err := task.Compile()
	assert.Nil(t, err)

	// 验证边中有字段映射
	assert.Equal(t, 2, len(task.dag.edges))
	hasMapping := false
	for _, edge := range task.dag.edges {
		if edge.edgeType == DataEdge && len(edge.mappings) > 0 {
			hasMapping = true
			assert.Equal(t, "result.name", edge.mappings[0].SourceField)
			assert.Equal(t, "displayName", edge.mappings[0].TargetField)
		}
	}
	assert.True(t, hasMapping)
}

// ===== 回滚策略集成测试 =====

func TestTask_RollbackStrategyAll(t *testing.T) {
	task := NewTask("rollback-all-test")
	task.SetRollbackStrategy(StrategyRollbackAll)

	a := NewSubtask("a", noopExec)
	b := NewSubtask("b", noopExec)
	c := NewSubtask("c", noopExec)

	_ = task.AddSubtask(a)
	_ = task.AddSubtask(b)
	_ = task.AddSubtask(c)

	_ = task.AddEdge(a, b)
	_ = task.AddEdge(b, c)

	_, err := task.Compile()
	assert.Nil(t, err)

	// 模拟 a 和 b 成功，c 失败
	_ = task.UpdateSubtaskState(a.GetID(), NodeSucceeded)
	_ = task.UpdateSubtaskState(b.GetID(), NodeSucceeded)
	_ = task.UpdateSubtaskState(c.GetID(), NodeFailed)

	// RollbackAll 应返回所有已完成的子任务
	rollbackable := task.GetRollbackableSubtasks()
	assert.Equal(t, 3, len(rollbackable))
}

func TestTask_RollbackStrategyFailed(t *testing.T) {
	task := NewTask("rollback-failed-test")
	task.SetRollbackStrategy(StrategyRollbackFailed)

	a := NewSubtask("a", noopExec)
	b := NewSubtask("b", noopExec)
	c := NewSubtask("c", noopExec)

	_ = task.AddSubtask(a)
	_ = task.AddSubtask(b)
	_ = task.AddSubtask(c)

	_ = task.AddEdge(a, b)
	_ = task.AddEdge(b, c)

	_, err := task.Compile()
	assert.Nil(t, err)

	_ = task.UpdateSubtaskState(a.GetID(), NodeSucceeded)
	_ = task.UpdateSubtaskState(b.GetID(), NodeSucceeded)
	_ = task.UpdateSubtaskState(c.GetID(), NodeFailed)

	// RollbackFailed 只返回失败的子任务
	rollbackable := task.GetRollbackableSubtasks()
	assert.Equal(t, 1, len(rollbackable))
	assert.Equal(t, c.GetID(), rollbackable[0])
}

func TestTask_RollbackStrategyCustom(t *testing.T) {
	task := NewTask("rollback-custom-test")
	task.SetCustomRollbackFunc(func(completed []string, failed string) []string {
		// 自定义：只回滚失败的直接前驱
		return []string{"b"} // 只回滚 b
	})

	a := NewSubtask("a", noopExec)
	b := NewSubtask("b", noopExec)
	c := NewSubtask("c", noopExec)

	_ = task.AddSubtask(a)
	_ = task.AddSubtask(b)
	_ = task.AddSubtask(c)

	_ = task.AddEdge(a, b)
	_ = task.AddEdge(b, c)

	_, err := task.Compile()
	assert.Nil(t, err)

	_ = task.UpdateSubtaskState(a.GetID(), NodeSucceeded)
	_ = task.UpdateSubtaskState(b.GetID(), NodeSucceeded)
	_ = task.UpdateSubtaskState(c.GetID(), NodeFailed)

	rollbackable := task.GetRollbackableSubtasks()
	assert.Equal(t, 1, len(rollbackable))
	assert.Equal(t, "b", rollbackable[0])
}

// ===== Pre/Post Processor 集成测试 =====

func TestSubtask_PrePostProcessor(t *testing.T) {
	preCalled := false
	postCalled := false

	a := NewSubtask("a", noopExec)
	a.SetPreProcessor(func(ctx interface{}, data any) (any, error) {
		preCalled = true
		return data, nil
	})
	a.SetPostProcessor(func(ctx interface{}, data any) (any, error) {
		postCalled = true
		return data, nil
	})

	assert.False(t, preCalled)
	assert.False(t, postCalled)
	assert.NotNil(t, a.preProcessor)
	assert.NotNil(t, a.postProcessor)
}

// ===== Callback 集成测试 =====

func TestTask_Callback(t *testing.T) {
	callback := &testCallback{}
	task := NewTask("callback-test")
	task.SetCallback(callback)

	assert.NotNil(t, task.getCallback())
}

type testCallback struct {
	mu        sync.Mutex
	starts    []string
	completes []string
	fails     []string
	skips     []string
	branches  []string
}

func (tc *testCallback) OnSubtaskStart(ctx interface{}, key string, input any) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.starts = append(tc.starts, key)
}

func (tc *testCallback) OnSubtaskComplete(ctx interface{}, key string, output any) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.completes = append(tc.completes, key)
}

func (tc *testCallback) OnSubtaskFailed(ctx interface{}, key string, err error) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.fails = append(tc.fails, key)
}

func (tc *testCallback) OnSubtaskSkipped(ctx interface{}, key string) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.skips = append(tc.skips, key)
}

func (tc *testCallback) OnBranchSelected(ctx interface{}, fromNode string, selectedNode string) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.branches = append(tc.branches, fromNode+"->"+selectedNode)
}

// ===== 优先级调度集成测试 =====

func TestTask_PriorityScheduling(t *testing.T) {
	task := NewTask("priority-test")

	a := NewSubtask("a", noopExec)
	b := NewSubtask("b", noopExec).SetPriority(10)
	c := NewSubtask("c", noopExec).SetPriority(5)

	_ = task.AddSubtask(a)
	_ = task.AddSubtask(b)
	_ = task.AddSubtask(c)

	// a 是前驱，b 和 c 都依赖 a
	_ = task.AddEdge(a, b)
	_ = task.AddEdge(a, c)

	_, err := task.Compile()
	assert.Nil(t, err)

	// a 先执行
	_ = task.UpdateSubtaskState(a.GetID(), NodeSucceeded)

	// b 和 c 都可执行，b 优先级更高应排在前面
	next := task.NextSubTasks()
	assert.Equal(t, 2, len(next))
	assert.Equal(t, "b", next[0].GetName()) // 优先级 10
	assert.Equal(t, "c", next[1].GetName()) // 优先级 5
}

// ===== 节点触发模式集成测试 =====

func TestTask_AnyPredecessorTrigger(t *testing.T) {
	task := NewTask("any-trigger-test")

	a := NewSubtask("a", noopExec)
	b := NewSubtask("b", noopExec)
	c := NewSubtask("c", noopExec).SetTriggerMode(AnyPredecessor) // 任一前驱完成即触发

	_ = task.AddSubtask(a)
	_ = task.AddSubtask(b)
	_ = task.AddSubtask(c)

	_ = task.AddEdge(a, c)
	_ = task.AddEdge(b, c)

	_, err := task.Compile()
	assert.Nil(t, err)

	// a 完成后，c 的触发模式为 AnyPredecessor，应该可以执行
	_ = task.UpdateSubtaskState(a.GetID(), NodeSucceeded)

	channel := task.getCompiled().GetChannel(c.GetID())
	assert.True(t, channel.isReady())
}

func TestTask_AllPredecessorTrigger(t *testing.T) {
	task := NewTask("all-trigger-test")

	a := NewSubtask("a", noopExec)
	b := NewSubtask("b", noopExec)
	c := NewSubtask("c", noopExec).SetTriggerMode(AllPredecessor) // 所有前驱完成才触发

	_ = task.AddSubtask(a)
	_ = task.AddSubtask(b)
	_ = task.AddSubtask(c)

	_ = task.AddEdge(a, c)
	_ = task.AddEdge(b, c)

	_, err := task.Compile()
	assert.Nil(t, err)

	// a 完成后，c 不可执行（b 还未完成）
	_ = task.UpdateSubtaskState(a.GetID(), NodeSucceeded)

	channel := task.getCompiled().GetChannel(c.GetID())
	assert.False(t, channel.isReady())

	// b 也完成后，c 可执行
	_ = task.UpdateSubtaskState(b.GetID(), NodeSucceeded)
	assert.True(t, channel.isReady())
}

// ===== 图可视化测试 =====

func TestTask_GraphDOT(t *testing.T) {
	task := NewTask("dot-test")

	a := NewSubtask("a", noopExec)
	b := NewSubtask("b", noopExec)

	_ = task.AddSubtask(a)
	_ = task.AddSubtask(b)
	_ = task.AddEdge(a, b)

	_, err := task.Compile()
	assert.Nil(t, err)

	dot := task.GraphDOT()
	assert.True(t, strings.Contains(dot, "digraph"))
	assert.True(t, strings.Contains(dot, "control+data"))
}

func TestTask_GraphMermaid(t *testing.T) {
	task := NewTask("mermaid-test")

	a := NewSubtask("a", noopExec)
	b := NewSubtask("b", noopExec)

	_ = task.AddSubtask(a)
	_ = task.AddSubtask(b)
	_ = task.AddEdge(a, b)

	_, err := task.Compile()
	assert.Nil(t, err)

	mermaid := task.GraphMermaid()
	assert.True(t, strings.Contains(mermaid, "graph TD"))
}

// ===== 完整 DAG 执行流程集成测试 =====

func TestTask_FullExecution(t *testing.T) {
	task := NewTask("full-exec-test")

	step1 := NewSubtask("step1", noopExec).SetInput("input-data")
	step2 := NewSubtask("step2", noopExec)
	step3 := NewSubtask("step3", noopExec)
	step4 := NewSubtask("step4", noopExec)

	_ = task.AddSubtask(step1)
	_ = task.AddSubtask(step2)
	_ = task.AddSubtask(step3)
	_ = task.AddSubtask(step4)

	// step1 -> step2 -> step4
	// step1 -> step3 -> step4
	_ = task.AddEdge(step1, step2)
	_ = task.AddEdge(step1, step3)
	_ = task.AddEdge(step2, step4)
	_ = task.AddEdge(step3, step4)

	compiled, err := task.Compile()
	assert.Nil(t, err)
	assert.NotNil(t, compiled)

	// step1 先执行
	next := task.NextSubTasks()
	assert.Equal(t, 1, len(next))
	assert.Equal(t, "step1", next[0].GetName())

	// step1 完成
	_ = task.UpdateSubtaskState(step1.GetID(), NodeSucceeded)

	// step2 和 step3 可并行执行
	next = task.NextSubTasks()
	assert.Equal(t, 2, len(next))

	// step2 完成
	_ = task.UpdateSubtaskState(step2.GetID(), NodeSucceeded)

	// step4 还不能执行（step3 未完成）
	next = task.NextSubTasks()
	assert.Equal(t, 1, len(next))
	assert.Equal(t, "step3", next[0].GetName())

	// step3 完成
	_ = task.UpdateSubtaskState(step3.GetID(), NodeSucceeded)

	// step4 可执行
	next = task.NextSubTasks()
	assert.Equal(t, 1, len(next))
	assert.Equal(t, "step4", next[0].GetName())

	// step4 完成
	_ = task.UpdateSubtaskState(step4.GetID(), NodeSucceeded)

	// 任务完成
	assert.True(t, task.IsFinished())
}

func TestTask_FullExecutionWithFailure(t *testing.T) {
	task := NewTask("failure-test")

	a := NewSubtask("a", noopExec)
	b := NewSubtask("b", noopExec)
	c := NewSubtask("c", noopExec)

	_ = task.AddSubtask(a)
	_ = task.AddSubtask(b)
	_ = task.AddSubtask(c)

	_ = task.AddEdge(a, b)
	_ = task.AddEdge(b, c)

	_, err := task.Compile()
	assert.Nil(t, err)

	// a 完成
	_ = task.UpdateSubtaskState(a.GetID(), NodeSucceeded)

	// b 失败
	_ = task.UpdateSubtaskState(b.GetID(), NodeFailed)

	// c 不可执行
	next := task.NextSubTasks()
	assert.Equal(t, 0, len(next))

	// 任务未完成（c 还是 pending）
	assert.False(t, task.IsFinished())
}

// ===== 执行器注册隔离测试 =====

func TestTask_ExecutorIsolation(t *testing.T) {
	task1 := NewTask("task1")
	task2 := NewTask("task2")

	task1.em.registerProviders("task1", map[string]executor.ExecutorProvider{
		"step1": executor.NewLocalExecutor(func(ctx context.Context, input string) (string, error) {
			return "result1", nil
		}),
	})
	task2.em.registerProviders("task2", map[string]executor.ExecutorProvider{
		"step1": executor.NewLocalExecutor(func(ctx context.Context, input string) (string, error) {
			return "result2", nil
		}),
	})

	// 两个 Task 的执行器互不影响
	e1 := task1.em.getProvider("task1", "step1")
	e2 := task2.em.getProvider("task2", "step1")

	result1, _ := e1.Execute(context.Background(), &executor.TaskData{Input: ""})
	result2, _ := e2.Execute(context.Background(), &executor.TaskData{Input: ""})

	assert.Equal(t, "result1", result1)
	assert.Equal(t, "result2", result2)
}

// ===== Subtask 配置测试 =====

func TestSubtask_Configuration(t *testing.T) {
	s := NewSubtask("test", noopExec)

	s.SetTriggerMode(AnyPredecessor)
	assert.Equal(t, AnyPredecessor, s.triggerMode)

	s.SetPriority(10)
	assert.Equal(t, 10, s.priority)

	s.SetTimeout(30 * time.Second)
	assert.Equal(t, 30*time.Second, s.timeout)

	s.SetRetry(5)
	assert.Equal(t, int8(5), s.subtask.Retry)

	s.SetRetryInterval(10)
	assert.Equal(t, int32(10), s.subtask.RetryInterval)
}

// ===== 辅助函数 =====

func indexOf(slice []string, item string) int {
	for i, v := range slice {
		if v == item {
			return i
		}
	}
	return -1
}

// ===== 旧 API 兼容测试 =====

func TestTask_OldAPICompatibility(t *testing.T) {
	// 使用旧方式创建 Task 和 Subtask
	task := NewTask("compat-test").SetInput("test")
	stp1 := NewSubtask("stp1", noopExec).SetInput("stp1")
	stp2 := NewSubtask("stp2", noopExec).SetInput("stp2")
	stp3 := NewSubtask("stp3", noopExec).SetInput("stp3")

	_ = task.AddSubtask(stp1)
	_ = task.AddSubtask(stp2)
	_ = task.AddSubtask(stp3)

	// 使用 AddEdge 替代旧的 AddDirectedEdge
	_ = task.AddEdge(stp1, stp2)
	_ = task.AddEdge(stp1, stp3)

	_, err := task.Compile()
	assert.Nil(t, err)

	assert.Equal(t, 3, task.Size())
	fmt.Println(task.Graph())

	// step1 先执行
	next := task.NextSubTasks()
	assert.Equal(t, 1, len(next))
	assert.Equal(t, "stp1", next[0].GetName())

	// step1 完成
	_ = task.UpdateSubtaskState(stp1.GetID(), NodeSucceeded)

	// step2 和 step3 可并行
	next = task.NextSubTasks()
	assert.Equal(t, 2, len(next))

	// 全部完成
	_ = task.UpdateSubtaskState(stp2.GetID(), NodeSucceeded)
	_ = task.UpdateSubtaskState(stp3.GetID(), NodeSucceeded)

	// 没有更多可执行节点
	next = task.NextSubTasks()
	assert.Equal(t, 0, len(next))
}

// ===== 类型兼容性校验测试 =====

func TestCompile_TypeMismatch(t *testing.T) {
	task := NewTask("type-mismatch-test")

	// stepOne 输出 map[string]any，stepTwo 输入 string，类型不兼容
	one := NewSubtask("one", executor.NewLocalExecutor(func(ctx context.Context, input map[string]any) (map[string]any, error) {
		return input, nil
	}))
	two := NewSubtask("two", executor.NewLocalExecutor(func(ctx context.Context, input string) (string, error) {
		return input, nil
	}))

	_ = task.AddSubtask(one)
	_ = task.AddSubtask(two)
	_ = task.AddEdge(one, two)

	_, err := task.Compile()
	assert.NotNil(t, err, "should detect type mismatch")
	assert.Contains(t, err.Error(), "type mismatch")
}

func TestCompile_TypeMatch(t *testing.T) {
	task := NewTask("type-match-test")

	// stepOne 输出 map[string]any，stepTwo 输入 map[string]any，类型兼容
	one := NewSubtask("one", executor.NewLocalExecutor(func(ctx context.Context, input map[string]any) (map[string]any, error) {
		return input, nil
	}))
	two := NewSubtask("two", executor.NewLocalExecutor(func(ctx context.Context, input map[string]any) (map[string]any, error) {
		return input, nil
	}))

	_ = task.AddSubtask(one)
	_ = task.AddSubtask(two)
	_ = task.AddEdge(one, two)

	_, err := task.Compile()
	assert.Nil(t, err, "same type should pass validation")
}

func TestCompile_MapStringAnyAcceptsMapStringX(t *testing.T) {
	task := NewTask("map-any-accepts-test")

	// stepOne 输出 map[string]string，stepTwo 输入 map[string]any
	// map[string]any 作为输入应接受 map[string]string 输出
	one := NewSubtask("one", executor.NewLocalExecutor(func(ctx context.Context, input map[string]string) (map[string]string, error) {
		return input, nil
	}))
	two := NewSubtask("two", executor.NewLocalExecutor(func(ctx context.Context, input map[string]any) (map[string]any, error) {
		return input, nil
	}))

	_ = task.AddSubtask(one)
	_ = task.AddSubtask(two)
	_ = task.AddEdge(one, two)

	_, err := task.Compile()
	assert.Nil(t, err, "map[string]any input should accept map[string]string output")
}

func TestCompile_ControlEdgeSkipsTypeCheck(t *testing.T) {
	task := NewTask("control-edge-skip-test")

	// 控制边不传数据，不校验类型
	one := NewSubtask("one", executor.NewLocalExecutor(func(ctx context.Context, input map[string]any) (map[string]any, error) {
		return input, nil
	}))
	two := NewSubtask("two", executor.NewLocalExecutor(func(ctx context.Context, input string) (string, error) {
		return input, nil
	}))

	_ = task.AddSubtask(one)
	_ = task.AddSubtask(two)
	_ = task.AddControlEdge(one, two)

	_, err := task.Compile()
	assert.Nil(t, err, "control edge should skip type validation")
}

func TestCompile_NonTypedProviderSkipsTypeCheck(t *testing.T) {
	task := NewTask("non-typed-provider-skip-test")

	// 使用不实现 TypedProvider 的自定义执行器，类型信息缺失，跳过校验
	one := NewSubtask("one", &nonTypedExec{})
	two := NewSubtask("two", &nonTypedExec{})

	_ = task.AddSubtask(one)
	_ = task.AddSubtask(two)
	_ = task.AddEdge(one, two)

	_, err := task.Compile()
	assert.Nil(t, err, "non-TypedProvider should skip type validation")
}

// nonTypedExec 不实现 TypedProvider 的自定义执行器
type nonTypedExec struct{}

func (e *nonTypedExec) Execute(ctx context.Context, data *executor.TaskData) (any, error) {
	return nil, nil
}

func (e *nonTypedExec) Protocol() executor.ExecutorProtocol {
	return "non-typed"
}

func TestCompile_AssignableTypes(t *testing.T) {
	task := NewTask("assignable-types-test")

	type base struct{ Name string }
	type extended struct{ Name string }

	// 相同结构的不同类型定义在 Go 中不可赋值，这里测试可赋值场景
	one := NewSubtask("one", executor.NewLocalExecutor(func(ctx context.Context, input map[string]any) (map[string]any, error) {
		return input, nil
	}))
	two := NewSubtask("two", executor.NewLocalExecutor(func(ctx context.Context, input map[string]any) (map[string]any, error) {
		return input, nil
	}))
	three := NewSubtask("three", executor.NewLocalExecutor(func(ctx context.Context, input string) (string, error) {
		return input, nil
	}))

	_ = task.AddSubtask(one)
	_ = task.AddSubtask(two)
	_ = task.AddSubtask(three)
	_ = task.AddEdge(one, two)   // map[string]any -> map[string]any, OK
	_ = task.AddEdge(two, three) // map[string]any -> string, mismatch

	_, err := task.Compile()
	assert.NotNil(t, err, "should detect type mismatch on edge two -> three")
	assert.Contains(t, err.Error(), "type mismatch")
}
