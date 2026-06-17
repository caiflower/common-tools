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
	"bytes"
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/caiflower/common-tools/pkg/basic"
	golocalv1 "github.com/caiflower/common-tools/pkg/golocal/v1"
	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/caiflower/common-tools/pkg/tools"
	"github.com/caiflower/common-tools/taskx/dao/model"
	"github.com/caiflower/common-tools/taskx/executor"
)

// ErrExecutorNotFound 执行器未找到错误
var ErrExecutorNotFound = errors.New("executor not found")

// Task 是 DAG 任务的根节点
type Task struct {
	task model.Task

	// DAG 相关
	dag        *dagGraph
	compiled   *compiledDAG
	subtaskMap map[string]*Subtask

	// 执行器管理（实例级）
	em *executorManager

	// 回调和回滚策略
	callback         DAGCallback
	rollbackStrategy RollbackStrategy
	customRollback   func(completed []string, failed string) []string

	// 子图映射（用于子图嵌套）
	subGraphs map[string]*Task
}

// NewTask 创建新的 Task
func NewTask(taskName string) *Task {
	return &Task{
		task: model.Task{
			ID:            tools.GenerateId("t"),
			TaskName:      taskName,
			State:         string(TaskPending),
			Retry:         DefaultRetryCount,
			RetryInterval: DefaultRetryInterval,
			AffinityType:  string(AffinityRandom),
		},
		dag:              NewDAGGraph(),
		subtaskMap:       make(map[string]*Subtask),
		em:               newExecutorManager(),
		rollbackStrategy: StrategyRollbackAll,
		subGraphs:        make(map[string]*Task),
	}
}

// Subtask 是 DAG 中的节点，支持泛型类型
type Subtask struct {
	subtask model.Subtask

	// 节点配置
	triggerMode   NodeTriggerMode
	priority      int
	timeout       time.Duration
	preProcessor  Processor
	postProcessor Processor

	// 执行器（创建时绑定，无需通过 name 关联）
	provider executor.ExecutorProvider

	// 回滚执行器
	rollbackProvider executor.ExecutorProvider
}

// NewSubtask 创建新的 Subtask（运行时类型推断）
func NewSubtask(name string, provider executor.ExecutorProvider) *Subtask {
	return &Subtask{
		subtask: model.Subtask{
			ID:            tools.GenerateId("st"),
			TaskName:      name,
			State:         string(TaskPending),
			Retry:         DefaultRetryCount,
			RetryInterval: DefaultRetryInterval,
			Rollback:      string(RollbackPending),
		},
		triggerMode: AllPredecessor,
		provider:    provider,
	}
}

// GetID 获取子任务 ID
func (s *Subtask) GetID() string {
	return s.subtask.ID
}

// GetName 获取子任务名称
func (s *Subtask) GetName() string {
	return s.subtask.TaskName
}

// getPreSubtaskID 获取前驱子任务ID列表（从 PreSubtaskID 字符串解析，内部使用）
func (s *Subtask) getPreSubtaskID() []string {
	if s.subtask.PreSubtaskID == "" {
		return nil
	}
	parts := strings.Split(s.subtask.PreSubtaskID, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// GetState 获取子任务状态（公开，用户可能需要查询状态）
func (s *Subtask) GetState() string {
	return s.subtask.State
}

// SetTriggerMode 设置节点触发模式
func (s *Subtask) SetTriggerMode(mode NodeTriggerMode) *Subtask {
	s.triggerMode = mode
	return s
}

// SetPriority 设置优先级
func (s *Subtask) SetPriority(priority int) *Subtask {
	s.priority = priority
	return s
}

// SetTimeout 设置超时时间
func (s *Subtask) SetTimeout(timeout time.Duration) *Subtask {
	s.timeout = timeout
	return s
}

// SetPreProcessor 设置前置处理器
// 前置处理器在执行器运行前调用，可以修改输入数据或进行校验
// 如果前置处理器返回错误，执行器不会运行
func (s *Subtask) SetPreProcessor(p Processor) *Subtask {
	s.preProcessor = p
	return s
}

// SetPostProcessor 设置后置处理器
// 后置处理器在执行器运行后调用，可以修改输出数据或进行结果校验
// 如果后置处理器返回错误，该错误将作为最终错误返回
func (s *Subtask) SetPostProcessor(p Processor) *Subtask {
	s.postProcessor = p
	return s
}

// Execute 执行子任务，封装 preProcessor → provider.Execute → postProcessor 的完整流程
// 如果未设置 provider，返回 nil, ErrExecutorNotFound
func (s *Subtask) Execute(ctx context.Context, data *executor.TaskData) (any, error) {
	if s.provider == nil {
		return nil, fmt.Errorf("subtask %s: %w", s.GetName(), ErrExecutorNotFound)
	}
	return executeWithProcessors(ctx, s.provider, s.preProcessor, s.postProcessor, data)
}

// executeWithProcessors 统一执行流程：preProcessor → provider.Execute → postProcessor
// 供 Subtask.Execute 和 taskReceiver.exec 共用，避免逻辑分散
func executeWithProcessors(ctx context.Context, provider executor.ExecutorProvider, preProcessor, postProcessor Processor, data *executor.TaskData) (any, error) {
	// 1. 前置处理器
	if preProcessor != nil {
		processedInput, err := preProcessor(ctx, data)
		if err != nil {
			return nil, fmt.Errorf("preProcessor failed: %w", err)
		}
		// 如果前置处理器返回了新的 TaskData，使用它替换原始数据
		if newData, ok := processedInput.(*executor.TaskData); ok {
			data = newData
		}
	}

	// 2. 执行器
	result, err := provider.Execute(ctx, data)
	if err != nil {
		return nil, err
	}

	// 3. 后置处理器
	if postProcessor != nil {
		processedOutput, err := postProcessor(ctx, result)
		if err != nil {
			return nil, fmt.Errorf("postProcessor failed: %w", err)
		}
		result = processedOutput
	}

	return result, nil
}

// SetRetry 设置重试次数
func (s *Subtask) SetRetry(retry int8) *Subtask {
	s.subtask.Retry = retry
	return s
}

// SetRetryInterval 设置重试间隔
func (s *Subtask) SetRetryInterval(retryInterval int32) *Subtask {
	s.subtask.RetryInterval = retryInterval
	return s
}

// getInput 获取输入（内部使用）
func (s *Subtask) getInput() string {
	return s.subtask.Input
}

// SetInput 设置输入
func (s *Subtask) SetInput(content interface{}) *Subtask {
	_tmp, err := tools.ToByte(content)
	if err != nil {
		logger.Error("SetInput serialize failed: %v", err)
		return s
	}
	s.subtask.Input = string(_tmp)
	return s
}

// SetRollbackExecutor 设置回滚执行器
func (s *Subtask) SetRollbackExecutor(p executor.ExecutorProvider) *Subtask {
	s.rollbackProvider = p
	return s
}

// GetExecutor 获取执行器
func (s *Subtask) GetExecutor() executor.ExecutorProvider {
	return s.provider
}

// unmarshalOutput 反序列化输出（内部使用）
func (s *Subtask) unmarshalOutput(v interface{}) error {
	return tools.DeByte([]byte(s.subtask.Output), v)
}

// IsFinished 判断子任务是否完成
func (s *Subtask) IsFinished() bool {
	return s.subtask.State == string(TaskSucceeded) || s.subtask.State == string(TaskFailed)
}

// IsSkipped 判断子任务是否被跳过
func (s *Subtask) IsSkipped() bool {
	return s.subtask.State == string(TaskSkipped)
}

// isRollbackFinished 判断回滚是否完成（内部使用）
func (s *Subtask) isRollbackFinished() bool {
	rollback := s.getRollback()
	return TaskRollbackState(rollback) == RollbackFailed ||
		TaskRollbackState(rollback) == RollbackSucceeded ||
		TaskRollbackState(rollback) == NoneRollback
}

// getRollback 获取回滚状态（内部使用）
func (s *Subtask) getRollback() string {
	return s.subtask.Rollback
}

func (s *Subtask) hasRollbackExecutor() bool {
	return s.rollbackProvider != nil
}

// getRetryInterval 获取重试间隔（内部使用）
func (s *Subtask) getRetryInterval() int32 {
	return s.subtask.RetryInterval
}

// getLastRunTime 获取最后运行时间（内部使用）
func (s *Subtask) getLastRunTime() *basic.Time {
	return &s.subtask.LastRunTime
}

// getModel 获取底层模型（内部使用）
func (s *Subtask) getModel() *model.Subtask {
	return &s.subtask
}

// ===== Task 方法 =====

// GetID 获取任务 ID
func (t *Task) GetID() string {
	return t.task.ID
}

// GetTaskName 获取任务名称
func (t *Task) GetTaskName() string {
	return t.task.TaskName
}

// SetRequestID 设置请求 ID
func (t *Task) SetRequestID(requestID string) *Task {
	t.task.RequestID = requestID
	return t
}

// SetInput 设置任务输入
func (t *Task) SetInput(content interface{}) *Task {
	_tmp, _ := tools.ToByte(content)
	t.task.Input = string(_tmp)
	return t
}

// GetInput 获取任务输入
func (t *Task) GetInput() string {
	return t.task.Input
}

// SetDescription 设置描述
func (t *Task) SetDescription(description string) *Task {
	t.task.Description = description
	return t
}

// SetExecuteTime 设置执行时间
func (t *Task) SetExecuteTime(executeTime time.Time) *Task {
	t.task.ExecuteTime = basic.Time(executeTime)
	return t
}

// SetAffinityType 设置亲和性类型
func (t *Task) SetAffinityType(affinityType TaskAffinityType) *Task {
	t.task.AffinityType = string(affinityType)
	return t
}

// SetUrgent 设置为紧急任务
func (t *Task) SetUrgent() *Task {
	t.task.Urgent = true
	return t
}

// getAffinityType 获取亲和性类型（内部使用）
func (t *Task) getAffinityType() TaskAffinityType {
	return TaskAffinityType(t.task.AffinityType)
}

// getPrimaryWorker 获取主工作节点（内部使用）
func (t *Task) getPrimaryWorker() string {
	return t.task.PrimaryWorker
}

// SetCallback 设置 DAG 回调
func (t *Task) SetCallback(callback DAGCallback) *Task {
	t.callback = callback
	return t
}

// SetRollbackStrategy 设置回滚策略
func (t *Task) SetRollbackStrategy(strategy RollbackStrategy) *Task {
	t.rollbackStrategy = strategy
	return t
}

// SetCustomRollbackFunc 设置自定义回滚函数
func (t *Task) SetCustomRollbackFunc(fn func(completed []string, failed string) []string) *Task {
	t.customRollback = fn
	t.rollbackStrategy = StrategyRollbackCustom
	return t
}

// AddSubtask 添加子任务到 Task
// 如果 Subtask 已通过 SetExecutor 绑定执行器，会自动注册到 executorManager
// 同时也会注册到全局注册表，确保集群 receiver 能找到 provider。
// 注意：同一 taskName+subTaskName 的 provider 会被后注册的覆盖（全局注册表的覆盖语义），
// 同一 taskName 的不同 Task 实例应注册相同的 provider。
func (t *Task) AddSubtask(subtask *Subtask) error {
	err := t.dag.AddNode(subtask.GetID(), subtask.triggerMode)
	if err != nil {
		return err
	}
	// 同步 Subtask 的配置到 dagNode
	if node := t.dag.GetNode(subtask.GetID()); node != nil {
		node.priority = subtask.priority
	}
	t.subtaskMap[subtask.GetID()] = subtask
	// 自动注册执行器（Subtask 上直接绑定的 provider）
	if subtask.provider != nil {
		t.em.registerProvider(t.task.TaskName, subtask.GetName(), subtask.provider)
		registerProvider(t.task.TaskName, subtask.GetName(), subtask.provider)
		// 提取类型信息到 DAG 节点，用于编译时类型校验
		if tp, ok := subtask.provider.(executor.TypedProvider); ok {
			node := t.dag.GetNode(subtask.GetID())
			if node != nil {
				node.inputType = tp.InputType()
				node.outputType = tp.OutputType()
			}
		}
	}
	if subtask.rollbackProvider != nil {
		t.em.registerRollbackProvider(t.task.TaskName, subtask.GetName(), subtask.rollbackProvider)
		registerRollbackProvider(t.task.TaskName, subtask.GetName(), subtask.rollbackProvider)
	}
	// 自动注册前置/后置处理器（集群 receiver 恢复时需要）
	if subtask.preProcessor != nil {
		registerPreProcessor(t.task.TaskName, subtask.GetName(), subtask.preProcessor)
	}
	if subtask.postProcessor != nil {
		registerPostProcessor(t.task.TaskName, subtask.GetName(), subtask.postProcessor)
	}
	return nil
}

// AddControlEdge 添加控制依赖边（只控制执行顺序，不传数据）
func (t *Task) AddControlEdge(src, dst *Subtask) error {
	return t.dag.AddEdge(src.GetID(), dst.GetID(), ControlEdge)
}

// AddDataEdge 添加数据依赖边（传数据，可配合 AddControlEdge 使用）
func (t *Task) AddDataEdge(src, dst *Subtask, mappings ...*FieldMapping) error {
	return t.dag.AddEdge(src.GetID(), dst.GetID(), DataEdge, mappings...)
}

// AddEdge 添加同时包含控制和数据的依赖边
func (t *Task) AddEdge(src, dst *Subtask, mappings ...*FieldMapping) error {
	return t.dag.AddEdge(src.GetID(), dst.GetID(), ControlAndDataEdge, mappings...)
}

// AddBranch 添加条件分支
func (t *Task) AddBranch(node *Subtask, branch *Branch) error {
	registerBranch(t.task.TaskName, node.GetID(), branch)
	return t.dag.AddBranch(node.GetID(), branch)
}

// addSubtaskGraph 添加子图节点（内部使用）
func (t *Task) addSubtaskGraph(key string, subTask *Task) error {
	if _, exists := t.subGraphs[key]; exists {
		return fmt.Errorf("subtask graph key already exists: %s", key)
	}
	t.subGraphs[key] = subTask
	// 子图作为特殊节点添加到主图
	return t.dag.AddNode(key, AllPredecessor)
}

// Compile 编译 DAG
func (t *Task) Compile() (*compiledDAG, error) {
	compiled, err := t.dag.Compile()
	if err != nil {
		return nil, err
	}
	t.compiled = compiled
	return compiled, nil
}

// getCompiled 获取编译后的 DAG（内部使用）
func (t *Task) getCompiled() *compiledDAG {
	return t.compiled
}

// NextSubTasks 获取下一个可执行的子任务列表
func (t *Task) NextSubTasks() []*Subtask {
	if t.compiled == nil {
		return nil
	}

	executables := t.compiled.GetExecutableNodes()
	var result []*Subtask
	for _, node := range executables {
		if subtask, exists := t.subtaskMap[node.key]; exists {
			result = append(result, subtask)
		}
	}

	// 按优先级降序排列
	sort.Slice(result, func(i, j int) bool {
		return result[i].priority > result[j].priority
	})

	return result
}

// UpdateSubtaskState 更新子任务状态（非破坏性）
func (t *Task) UpdateSubtaskState(subtaskID string, state NodeState) error {
	err := t.dag.UpdateNodeState(subtaskID, state)
	if err != nil {
		return err
	}

	subtask := t.subtaskMap[subtaskID]
	if subtask == nil {
		return fmt.Errorf("subtask not found: %s", subtaskID)
	}

	switch state {
	case NodeSucceeded:
		subtask.subtask.State = string(TaskSucceeded)
	case NodeFailed:
		subtask.subtask.State = string(TaskFailed)
	case NodeSkipped:
		subtask.subtask.State = string(TaskSkipped)
	case NodeRunning:
		subtask.subtask.State = string(TaskRunning)
		now := time.Now()
		subtask.subtask.LastRunTime = basic.Time(now)
	}

	// 更新后继节点的通道状态
	if t.compiled != nil {
		switch state {
		case NodeSucceeded:
			// 通知所有后继节点：控制依赖就绪
			for _, succKey := range t.dag.controlAdj[subtaskID] {
				if ch := t.compiled.GetChannel(succKey); ch != nil {
					ch.reportDependencies([]string{subtaskID})
				}
			}
			// 通知所有后继节点：数据就绪
			for _, succKey := range t.dag.dataAdj[subtaskID] {
				if ch := t.compiled.GetChannel(succKey); ch != nil {
					ch.reportValues(map[string]any{subtaskID: subtask.subtask.Output})
				}
			}
		case NodeSkipped:
			// 通知所有后继节点：前驱跳过
			for _, succKey := range t.dag.controlAdj[subtaskID] {
				if ch := t.compiled.GetChannel(succKey); ch != nil {
					ch.reportSkip([]string{subtaskID})
				}
			}
			// 数据后继也标记跳过
			for _, succKey := range t.dag.dataAdj[subtaskID] {
				if ch := t.compiled.GetChannel(succKey); ch != nil {
					ch.reportValues(map[string]any{subtaskID: nil})
				}
			}
		}
	}

	return nil
}

// SkipSubtask 跳过子任务
func (t *Task) SkipSubtask(subtaskID string) error {
	return t.UpdateSubtaskState(subtaskID, NodeSkipped)
}

// IsFinished 判断任务是否完成
func (t *Task) IsFinished() bool {
	if t.task.State == string(TaskFailed) || t.task.State == string(TaskSucceeded) {
		return true
	}
	if t.compiled != nil {
		return t.compiled.IsAllFinished()
	}
	return false
}

// Size 返回子任务数量
func (t *Task) Size() int {
	return t.dag.Order()
}

// Graph 返回拓扑排序的文本表示
func (t *Task) Graph() string {
	if t.compiled == nil {
		return "DAG not compiled"
	}

	var buf bytes.Buffer
	topo := t.compiled.GetTopoOrder()

	for _, key := range topo {
		subtask := t.subtaskMap[key]
		if subtask == nil {
			continue
		}

		buf.WriteString(fmt.Sprintf("[%s](%s)", subtask.GetName(), subtask.GetState()))

		// 获取后继节点
		adj := t.dag.controlAdj[key]
		if len(adj) > 0 {
			var targets []string
			for _, adjKey := range adj {
				if adjSubtask := t.subtaskMap[adjKey]; adjSubtask != nil {
					targets = append(targets, adjSubtask.GetName())
				}
			}
			buf.WriteString(" => " + strings.Join(targets, ", "))
		}

		buf.WriteString("\n")
	}

	return buf.String()
}

// GraphDOT 返回 DOT 格式
func (t *Task) GraphDOT() string {
	if t.compiled == nil {
		return "// DAG not compiled"
	}
	return t.compiled.GraphDOT()
}

// GraphMermaid 返回 Mermaid 格式
func (t *Task) GraphMermaid() string {
	if t.compiled == nil {
		return "// DAG not compiled"
	}
	return t.compiled.GraphMermaid()
}

// RegisterTaskExecutor 注册任务执行器回调（FinishedTask/FailedTask）
// 注意：子任务执行器（ExecutorProvider）现在通过 Subtask.SetExecutor() 绑定，AddSubtask 时自动注册
func (t *Task) RegisterTaskExecutor(taskExecutor TaskExecutor) {
	t.em.registerTaskExecutor(taskExecutor)
	// 同时注册到全局注册表（集群框架：receiver 从数据库读取时需要）
	registerTaskExecutor(taskExecutor)
}

// RegisterTaskExecutor 包级函数：仅注册 TaskExecutor 到全局注册表
func RegisterTaskExecutor(taskExecutor TaskExecutor) {
	registerTaskExecutor(taskExecutor)
}

// RegisterBranchCondition 注册分支条件
func (t *Task) RegisterBranchCondition(nodeKey, branchKey string, condition func(ctx interface{}, input any) (string, error)) {
	t.em.registerBranchCondition(nodeKey, branchKey, condition)
}

// getProvider 获取子任务执行器（内部使用，用户应通过 Subtask.GetExecutor() 获取）
func (t *Task) getProvider(taskName, subTaskName string) executor.ExecutorProvider {
	return t.em.getProvider(taskName, subTaskName)
}

// getBranchCondition 获取分支条件（内部使用）
func (t *Task) getBranchCondition(nodeKey, branchKey string) func(ctx interface{}, input any) (string, error) {
	return t.em.getBranchCondition(nodeKey, branchKey)
}

// getCallback 获取回调（内部使用）
func (t *Task) getCallback() DAGCallback {
	return t.callback
}

// getRollbackStrategy 获取回滚策略（内部使用）
func (t *Task) getRollbackStrategy() RollbackStrategy {
	return t.rollbackStrategy
}

// GetRollbackableSubtasks 获取需要回滚的子任务列表
// 根据回滚策略返回需要回滚的子任务 ID 列表
func (t *Task) GetRollbackableSubtasks() []string {
	var completed []string
	var failedList []string

	// 收集已完成的子任务和失败的子任务
	for _, subtask := range t.subtaskMap {
		if subtask.subtask.State == string(TaskSucceeded) {
			completed = append(completed, subtask.GetID())
		} else if subtask.subtask.State == string(TaskFailed) {
			failedList = append(failedList, subtask.GetID())
		}
	}

	switch t.rollbackStrategy {
	case StrategyRollbackAll:
		// 返回所有已完成和失败的子任务（逆拓扑序），确保 failed 子任务也被回滚
		if t.compiled != nil {
			topo := t.compiled.GetTopoOrder()
			var result []string
			for i := len(topo) - 1; i >= 0; i-- {
				for _, id := range completed {
					if id == topo[i] {
						result = append(result, id)
						break
					}
				}
				for _, id := range failedList {
					if id == topo[i] {
						result = append(result, id)
						break
					}
				}
			}
			return result
		}
		return append(completed, failedList...)

	case StrategyRollbackFailed:
		// 只返回失败的子任务
		if len(failedList) > 0 {
			return failedList
		}
		return nil

	case StrategyRollbackCustom:
		// 使用自定义回滚函数
		if t.customRollback != nil {
			var failed string
			if len(failedList) > 0 {
				failed = failedList[0]
			}
			return t.customRollback(completed, failed)
		}
		return nil

	default:
		return nil
	}
}

// LeafRollbackSubtasks returns the leaf rollback subtasks in reverse topological order.
// A leaf is a rollbackable subtask whose forward dependents (subtasks that depend on it)
// have all completed their rollback. This ensures rollback proceeds from leaves toward roots.
//
// The rollbackable set and dependency graph are computed entirely from the Task's own state.
func (t *Task) LeafRollbackSubtasks() []*model.Subtask {
	rollbackableIDs := t.GetRollbackableSubtasks()
	rollbackableIDSet := make(map[string]bool, len(rollbackableIDs))
	for _, id := range rollbackableIDs {
		rollbackableIDSet[id] = true
	}

	// Build forward dependency map: subtaskID -> subtasks that depend on it
	dependsOn := make(map[string][]string)
	for _, subtask := range t.subtaskMap {
		if subtask.subtask.PreSubtaskID != "" {
			for _, preID := range strings.Split(subtask.subtask.PreSubtaskID, ",") {
				preID = strings.TrimSpace(preID)
				if preID != "" {
					dependsOn[preID] = append(dependsOn[preID], subtask.GetID())
				}
			}
		}
	}

	// Only keep rollbackable leaves whose dependencies have all completed rollback
	var leaves []*model.Subtask
	for _, id := range rollbackableIDs {
		subtask := t.subtaskMap[id]
		if subtask == nil {
			continue
		}
		isLeaf := true
		for _, depID := range dependsOn[id] {
			depSubtask := t.subtaskMap[depID]
			if depSubtask != nil && rollbackableIDSet[depID] && !depSubtask.isRollbackFinished() {
				isLeaf = false
				break
			}
		}
		if isLeaf {
			leaves = append(leaves, subtask.getModel())
		}
	}
	return leaves
}

// getCustomRollbackFunc 获取自定义回滚函数（内部使用）
func (t *Task) getCustomRollbackFunc() func(completed []string, failed string) []string {
	return t.customRollback
}

// DAGCallback DAG 生命周期回调接口
type DAGCallback interface {
	OnSubtaskStart(ctx interface{}, key string, input any)
	OnSubtaskComplete(ctx interface{}, key string, output any)
	OnSubtaskFailed(ctx interface{}, key string, err error)
	OnSubtaskSkipped(ctx interface{}, key string)
	OnBranchSelected(ctx interface{}, fromNode string, selectedNode string)
}

// NoOpCallback 空回调实现
type NoOpCallback struct{}

func (n *NoOpCallback) OnSubtaskStart(ctx interface{}, key string, input any)                  {}
func (n *NoOpCallback) OnSubtaskComplete(ctx interface{}, key string, output any)              {}
func (n *NoOpCallback) OnSubtaskFailed(ctx interface{}, key string, err error)                 {}
func (n *NoOpCallback) OnSubtaskSkipped(ctx interface{}, key string)                           {}
func (n *NoOpCallback) OnBranchSelected(ctx interface{}, fromNode string, selectedNode string) {}

// ===== TaskExecutor 接口 =====

// TaskExecutor 任务执行器接口
type TaskExecutor interface {
	Name() string
	//// FinishedTask 任务完成时的回调
	//FinishedTask(data *TaskData) error
	//// FailedTask 任务失败时的回调
	//FailedTask(data *TaskData) error
}

// executorManager 执行器管理器（实例级）
type executorManager struct {
	mu sync.RWMutex

	taskExecutors     map[string]TaskExecutor
	subtaskProviders  map[string]map[string]executor.ExecutorProvider
	branchConditions  map[string]map[string]func(ctx interface{}, input any) (string, error)
	rollbackProviders map[string]executor.ExecutorProvider // key: taskName/subTaskName
}

func newExecutorManager() *executorManager {
	return &executorManager{
		taskExecutors:     make(map[string]TaskExecutor),
		subtaskProviders:  make(map[string]map[string]executor.ExecutorProvider),
		branchConditions:  make(map[string]map[string]func(ctx interface{}, input any) (string, error)),
		rollbackProviders: make(map[string]executor.ExecutorProvider),
	}
}

func (em *executorManager) registerTaskExecutor(executor TaskExecutor) {
	em.mu.Lock()
	defer em.mu.Unlock()

	em.taskExecutors[executor.Name()] = executor
}

// registerProviders 批量注册子任务执行器
func (em *executorManager) registerProviders(taskName string, providers map[string]executor.ExecutorProvider) {
	em.mu.Lock()
	defer em.mu.Unlock()

	if em.subtaskProviders[taskName] == nil {
		em.subtaskProviders[taskName] = make(map[string]executor.ExecutorProvider)
	}
	for name, p := range providers {
		em.subtaskProviders[taskName][name] = p
	}
}

// registerProvider 注册单个子任务执行器
func (em *executorManager) registerProvider(taskName, subTaskName string, p executor.ExecutorProvider) {
	em.mu.Lock()
	defer em.mu.Unlock()

	if em.subtaskProviders[taskName] == nil {
		em.subtaskProviders[taskName] = make(map[string]executor.ExecutorProvider)
	}
	em.subtaskProviders[taskName][subTaskName] = p
}

func (em *executorManager) registerBranchCondition(nodeKey, branchKey string, condition func(ctx interface{}, input any) (string, error)) {
	em.mu.Lock()
	defer em.mu.Unlock()

	if em.branchConditions[nodeKey] == nil {
		em.branchConditions[nodeKey] = make(map[string]func(ctx interface{}, input any) (string, error))
	}
	em.branchConditions[nodeKey][branchKey] = condition
}

func (em *executorManager) registerRollbackProvider(taskName, subTaskName string, p executor.ExecutorProvider) {
	em.mu.Lock()
	defer em.mu.Unlock()

	em.rollbackProviders[taskName+"/"+subTaskName] = p
}

func (em *executorManager) getTaskExecutor(taskName string) TaskExecutor {
	em.mu.RLock()
	defer em.mu.RUnlock()

	return em.taskExecutors[taskName]
}

// getProvider 查找子任务执行器（实例级）
func (em *executorManager) getProvider(taskName, subTaskName string) executor.ExecutorProvider {
	em.mu.RLock()
	defer em.mu.RUnlock()

	if providers, ok := em.subtaskProviders[taskName]; ok {
		return providers[subTaskName]
	}
	return nil
}

func (em *executorManager) getBranchCondition(nodeKey, branchKey string) func(ctx interface{}, input any) (string, error) {
	em.mu.RLock()
	defer em.mu.RUnlock()

	if conditions, ok := em.branchConditions[nodeKey]; ok {
		return conditions[branchKey]
	}
	return nil
}

func (em *executorManager) getRollbackProvider(taskName, subTaskName string) executor.ExecutorProvider {
	em.mu.RLock()
	defer em.mu.RUnlock()

	return em.rollbackProviders[taskName+"/"+subTaskName]
}

// TaskData 任务执行时的数据传递结构
type TaskData struct {
	RequestId   string
	TaskId      string
	SubTaskId   string
	Input       string
	MergedInput map[string]any // 字段映射合并后的输入
	Subtasks    map[string]Output
}

// convert2Bean 将 Task 转换为数据库模型（含边缘数据和边信息）
func (t *Task) convert2Bean() (*model.Task, []model.Subtask, []model.TaskEdge) {
	task := t.task
	task.RollbackStrategy = t.rollbackStrategy.toDBString()
	task.Status = 1
	if task.RequestID == "" {
		task.RequestID = golocalv1.GetTraceID()
		if task.RequestID == "" {
			task.RequestID = tools.UUID()
		}
	}

	// 构建反向映射：nodeKey -> [前驱nodeKey列表]
	predecessors := make(map[string]map[string]struct{})
	for nodeKey := range t.subtaskMap {
		predecessors[nodeKey] = make(map[string]struct{})
	}
	// 从控制边收集前驱（from -> to, 所以 to 的前驱包含 from。用于 PreSubtaskID 兼容旧数据）
	// 注意：exec 阶段会通过 task_edge 表过滤控制前驱，不会传数据
	for from, successors := range t.dag.controlAdj {
		for _, to := range successors {
			predecessors[to][from] = struct{}{}
		}
	}
	// 从数据边收集前驱
	for from, successors := range t.dag.dataAdj {
		for _, to := range successors {
			predecessors[to][from] = struct{}{}
		}
	}
	subtaskBeans := make([]model.Subtask, 0, len(t.subtaskMap))
	for _, subtask := range t.subtaskMap {
		bean := *subtask.getModel()
		bean.TaskID = task.ID
		bean.TriggerMode = subtask.triggerMode.toDBString()
		bean.Priority = subtask.priority
		bean.Timeout = int(subtask.timeout.Seconds())
		if preds := predecessors[subtask.GetID()]; len(preds) > 0 {
			ids := make([]string, 0, len(preds))
			for k := range preds {
				ids = append(ids, k)
			}
			bean.PreSubtaskID = strings.Join(ids, ",")
		}
		bean.Status = 1
		subtaskBeans = append(subtaskBeans, bean)
	}

	// 构建边表
	edgeBeans := make([]model.TaskEdge, 0, len(t.dag.edges))
	for _, edge := range t.dag.edges {
		var mappingsJSON string
		if len(edge.mappings) > 0 {
			mappingsJSON = tools.ToJson(edge.mappings)
		}
		edgeBeans = append(edgeBeans, model.TaskEdge{
			ID:            tools.GenerateId("te"),
			TaskID:        task.ID,
			FromSubtaskID: edge.from,
			ToSubtaskID:   edge.to,
			EdgeType:      edge.edgeType.toDBString(),
			FieldMappings: mappingsJSON,
		})
	}

	return &task, subtaskBeans, edgeBeans
}

// initByBean 从数据库模型初始化 Task
//
// 说明：以下信息已从 task_edge 表（边类型/字段映射）和 subtask 表（triggerMode/priority/timeout）持久化恢复：
//   - 边类型（ControlEdge/DataEdge/ControlAndDataEdge）从 task_edge.edge_type 恢复
//   - triggerMode（AllPredecessor/AnyPredecessor）从 subtask.trigger_mode 恢复
//   - priority 从 subtask.priority 恢复
//   - timeout 从 subtask.timeout 恢复
//   - rollbackStrategy 从 task.rollback_strategy 恢复
//   - fieldMappings 从 task_edge.field_mappings JSON 恢复
//
// 无条件节点（Branch.Condition）和执行器（ExecutorProvider）为代码层概念，通过全局注册表恢复。
func (t *Task) initByBean(taskBean *model.Task, subtaskBeans []model.Subtask, edges []model.TaskEdge) (*Task, error) {
	t.task = *taskBean
	t.rollbackStrategy = rollbackStrategyFromDBString(taskBean.RollbackStrategy)
	t.subtaskMap = make(map[string]*Subtask)

	// 初始化 executorManager 并从全局注册表恢复执行器
	t.em = newExecutorManager()
	if te := getTaskExecutor(taskBean.TaskName); te != nil {
		t.em.registerTaskExecutor(te)
	}

	for i := range subtaskBeans {
		subtask := &Subtask{
			subtask: subtaskBeans[i],
		}
		// 从 DB 恢复 triggerMode、priority、timeout
		subtask.triggerMode = triggerModeFromDBString(subtaskBeans[i].TriggerMode)
		subtask.priority = subtaskBeans[i].Priority
		subtask.timeout = time.Duration(subtaskBeans[i].Timeout) * time.Second
		t.subtaskMap[subtask.GetID()] = subtask

		// 从全局注册表恢复 provider 和 rollbackProvider
		if p := getProvider(taskBean.TaskName, subtask.GetName()); p != nil {
			subtask.provider = p
			t.em.registerProvider(taskBean.TaskName, subtask.GetName(), p)
		}
		if p := getRollbackProvider(taskBean.TaskName, subtask.GetName()); p != nil {
			subtask.rollbackProvider = p
			t.em.registerRollbackProvider(taskBean.TaskName, subtask.GetName(), p)
		}
		// 从全局注册表恢复 preProcessor 和 postProcessor
		if p := getPreProcessor(taskBean.TaskName, subtask.GetName()); p != nil {
			subtask.preProcessor = p
		}
		if p := getPostProcessor(taskBean.TaskName, subtask.GetName()); p != nil {
			subtask.postProcessor = p
		}
	}

	// 重新构建 DAG
	t.dag = NewDAGGraph()
	for _, subtask := range t.subtaskMap {
		if err := t.dag.AddNode(subtask.GetID(), subtask.triggerMode); err != nil {
			return nil, err
		}
		// 从 Subtask 恢复 dagNode 的 priority、timeout、preProcessor、postProcessor
		if node := t.dag.GetNode(subtask.GetID()); node != nil {
			node.priority = subtask.priority
			node.timeout = subtask.timeout
			node.preProcessor = subtask.preProcessor
			node.postProcessor = subtask.postProcessor
		}
	}
	// 重建边（优先从 task_edge 表恢复精确类型，回退到 pre_subtask_id 推断）
	if len(edges) > 0 {
		for _, edge := range edges {
			edgeType := edgeTypeFromDBString(edge.EdgeType)
			var mappings []*FieldMapping
			if edge.FieldMappings != "" {
				_ = tools.Unmarshal([]byte(edge.FieldMappings), &mappings)
			}
			_ = t.dag.AddEdge(edge.FromSubtaskID, edge.ToSubtaskID, edgeType, mappings...)
		}
	} else {
		// 无 task_edge 记录时回退到 pre_subtask_id 推断（兼容旧数据）
		for _, subtask := range t.subtaskMap {
			preIDs := subtask.getPreSubtaskID()
			if len(preIDs) > 0 {
				for _, preID := range preIDs {
					if preID == "" {
						continue
					}
					if _, exists := t.subtaskMap[preID]; exists {
						_ = t.dag.AddEdge(preID, subtask.GetID(), ControlAndDataEdge)
					}
				}
			}
		}
	}
	// 从全局注册表恢复分支信息（分支的 Condition 函数无法持久化到 DB，必须从全局注册表恢复）
	if registeredBranches := getRegisteredBranches(taskBean.TaskName); len(registeredBranches) > 0 {
		for nodeKey, branches := range registeredBranches {
			t.dag.branches[nodeKey] = branches
		}
	}
	// 编译 DAG
	if _, err := t.Compile(); err != nil {
		return nil, err
	}
	// 根据 DB 状态恢复 DAG 节点状态（关键：否则已完成节点无法通知后继节点）
	for _, subtask := range t.subtaskMap {
		switch subtask.subtask.State {
		case string(TaskSucceeded):
			_ = t.dag.UpdateNodeState(subtask.GetID(), NodeSucceeded)
			if ch := t.compiled.GetChannel(subtask.GetID()); ch != nil {
				ch.reportDependencies(nil)
				ch.reportValues(map[string]any{subtask.GetID(): subtask.subtask.Output})
			}
			// 通知后继节点
			for _, succKey := range t.dag.controlAdj[subtask.GetID()] {
				if ch := t.compiled.GetChannel(succKey); ch != nil {
					ch.reportDependencies([]string{subtask.GetID()})
				}
			}
			for _, succKey := range t.dag.dataAdj[subtask.GetID()] {
				if ch := t.compiled.GetChannel(succKey); ch != nil {
					ch.reportValues(map[string]any{subtask.GetID(): subtask.subtask.Output})
				}
			}
		case string(TaskFailed):
			_ = t.dag.UpdateNodeState(subtask.GetID(), NodeFailed)
		case string(TaskSkipped):
			_ = t.dag.UpdateNodeState(subtask.GetID(), NodeSkipped)
		case string(TaskRunning):
			_ = t.dag.UpdateNodeState(subtask.GetID(), NodeRunning)
		}
	}
	return t, nil
}

// getState 获取任务状态（内部使用）
func (t *Task) getState() string {
	return string(t.task.State)
}

// ===== DB 字符串转换辅助函数 =====

// toDBString 将 EdgeType 转换为 DB 存储的字符串
func (e EdgeType) toDBString() string {
	switch e {
	case ControlEdge:
		return EdgeTypeControl
	case DataEdge:
		return EdgeTypeData
	case ControlAndDataEdge:
		return EdgeTypeControlAndData
	default:
		return EdgeTypeControlAndData
	}
}

// edgeTypeFromDBString 从 DB 字符串恢复 EdgeType
func edgeTypeFromDBString(s string) EdgeType {
	switch s {
	case EdgeTypeControl:
		return ControlEdge
	case EdgeTypeData:
		return DataEdge
	case EdgeTypeControlAndData:
		return ControlAndDataEdge
	default:
		return ControlAndDataEdge
	}
}

// toDBString 将 NodeTriggerMode 转换为 DB 存储的字符串
func (m NodeTriggerMode) toDBString() string {
	switch m {
	case AllPredecessor:
		return TriggerModeAllPredecessor
	case AnyPredecessor:
		return TriggerModeAnyPredecessor
	default:
		return TriggerModeAllPredecessor
	}
}

// triggerModeFromDBString 从 DB 字符串恢复 NodeTriggerMode
func triggerModeFromDBString(s string) NodeTriggerMode {
	switch s {
	case TriggerModeAllPredecessor:
		return AllPredecessor
	case TriggerModeAnyPredecessor:
		return AnyPredecessor
	default:
		return AllPredecessor
	}
}

// toDBString 将 RollbackStrategy 转换为 DB 存储的字符串
func (s RollbackStrategy) toDBString() string {
	switch s {
	case StrategyRollbackAll:
		return RollbackStrategyAll
	case StrategyRollbackFailed:
		return RollbackStrategyFailed
	case StrategyRollbackCustom:
		return RollbackStrategyCustom
	default:
		return RollbackStrategyAll
	}
}

// rollbackStrategyFromDBString 从 DB 字符串恢复 RollbackStrategy
func rollbackStrategyFromDBString(s string) RollbackStrategy {
	switch s {
	case RollbackStrategyAll:
		return StrategyRollbackAll
	case RollbackStrategyFailed:
		return StrategyRollbackFailed
	case RollbackStrategyCustom:
		return StrategyRollbackCustom
	default:
		return StrategyRollbackAll
	}
}
