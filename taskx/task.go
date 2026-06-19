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

// ErrExecutorNotFound is returned when an executor is not found
var ErrExecutorNotFound = errors.New("executor not found")

// Task is the root node of a DAG task
type Task struct {
	task model.Task

	// DAG related
	dag        *dagGraph
	compiled   *compiledDAG
	subtaskMap map[string]*Subtask

	// Executor management (instance-level)
	em *executorManager

	// Callback and rollback strategy
	callback         DAGCallback
	rollbackStrategy RollbackStrategy
	customRollback   func(completed []string, failed string) []string

	// Subgraph map (for subgraph nesting)
	subGraphs map[string]*Task
}

// NewTask creates a new Task
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

// Subtask is a node in the DAG, supporting generic types
type Subtask struct {
	subtask model.Subtask

	// Node configuration
	triggerMode   NodeTriggerMode
	priority      int
	timeout       time.Duration
	preProcessor  Processor
	postProcessor Processor

	// Executor (bound at creation time, no need to associate by name)
	provider executor.ExecutorProvider

	// Rollback executor
	rollbackProvider executor.ExecutorProvider
}

// NewSubtask creates a new Subtask (runtime type inference)
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

// GetID returns the subtask ID
func (s *Subtask) GetID() string {
	return s.subtask.ID
}

// GetName returns the subtask name
func (s *Subtask) GetName() string {
	return s.subtask.TaskName
}

// getPreSubtaskID returns the predecessor subtask ID list (parsed from PreSubtaskID string, internal use)
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

// GetState returns the subtask state (public, users may need to query state)
func (s *Subtask) GetState() string {
	return s.subtask.State
}

// SetTriggerMode sets the node trigger mode
func (s *Subtask) SetTriggerMode(mode NodeTriggerMode) *Subtask {
	s.triggerMode = mode
	return s
}

// SetPriority sets the priority
func (s *Subtask) SetPriority(priority int) *Subtask {
	s.priority = priority
	return s
}

// SetTimeout sets the timeout duration
func (s *Subtask) SetTimeout(timeout time.Duration) *Subtask {
	s.timeout = timeout
	return s
}

// SetPreProcessor sets the pre-processor.
// The pre-processor is called before the executor runs and can modify input data or perform validation.
// If the pre-processor returns an error, the executor will not run.
func (s *Subtask) SetPreProcessor(p Processor) *Subtask {
	s.preProcessor = p
	return s
}

// SetPostProcessor sets the post-processor.
// The post-processor is called after the executor runs and can modify output data or validate results.
// If the post-processor returns an error, that error will be returned as the final error.
func (s *Subtask) SetPostProcessor(p Processor) *Subtask {
	s.postProcessor = p
	return s
}

// Execute runs the subtask, wrapping the full flow: preProcessor -> provider.Execute -> postProcessor.
// Returns nil, ErrExecutorNotFound if no provider is set.
func (s *Subtask) Execute(ctx context.Context, data *executor.TaskData) (any, error) {
	if s.provider == nil {
		return nil, fmt.Errorf("subtask %s: %w", s.GetName(), ErrExecutorNotFound)
	}
	return executeWithProcessors(ctx, s.provider, s.preProcessor, s.postProcessor, data)
}

// executeWithProcessors is the unified execution flow: preProcessor -> provider.Execute -> postProcessor.
// Shared by Subtask.Execute and taskReceiver.exec to avoid logic duplication.
func executeWithProcessors(ctx context.Context, provider executor.ExecutorProvider, preProcessor, postProcessor Processor, data *executor.TaskData) (any, error) {
	// 1. Pre-processor
	if preProcessor != nil {
		processedInput, err := preProcessor(ctx, data)
		if err != nil {
			return nil, fmt.Errorf("preProcessor failed: %w", err)
		}
		// If the pre-processor returned a new TaskData, use it to replace the original data
		if newData, ok := processedInput.(*executor.TaskData); ok {
			data = newData
		}
	}

	// 2. Executor
	result, err := provider.Execute(ctx, data)
	if err != nil {
		return nil, err
	}

	// 3. Post-processor
	if postProcessor != nil {
		processedOutput, err := postProcessor(ctx, result)
		if err != nil {
			return nil, fmt.Errorf("postProcessor failed: %w", err)
		}
		result = processedOutput
	}

	return result, nil
}

// SetRetry sets the retry count
func (s *Subtask) SetRetry(retry int8) *Subtask {
	s.subtask.Retry = retry
	return s
}

// SetRetryInterval sets the retry interval
func (s *Subtask) SetRetryInterval(retryInterval int32) *Subtask {
	s.subtask.RetryInterval = retryInterval
	return s
}

// getInput returns the input (internal use)
func (s *Subtask) getInput() string {
	return s.subtask.Input
}

// SetInput sets the input
func (s *Subtask) SetInput(content interface{}) *Subtask {
	_tmp, err := tools.ToByte(content)
	if err != nil {
		logger.Error("SetInput serialize failed: %v", err)
		return s
	}
	s.subtask.Input = string(_tmp)
	return s
}

// SetRollbackExecutor sets the rollback executor
func (s *Subtask) SetRollbackExecutor(p executor.ExecutorProvider) *Subtask {
	s.rollbackProvider = p
	return s
}

// GetExecutor returns the executor
func (s *Subtask) GetExecutor() executor.ExecutorProvider {
	return s.provider
}

// unmarshalOutput deserializes the output (internal use)
func (s *Subtask) unmarshalOutput(v interface{}) error {
	return tools.DeByte([]byte(s.subtask.Output), v)
}

// IsFinished checks whether the subtask is finished
func (s *Subtask) IsFinished() bool {
	return s.subtask.State == string(TaskSucceeded) || s.subtask.State == string(TaskFailed)
}

// IsSkipped checks whether the subtask is skipped
func (s *Subtask) IsSkipped() bool {
	return s.subtask.State == string(TaskSkipped)
}

// isRollbackFinished checks whether the rollback is complete (internal use)
func (s *Subtask) isRollbackFinished() bool {
	rollback := s.getRollback()
	return TaskRollbackState(rollback) == RollbackFailed ||
		TaskRollbackState(rollback) == RollbackSucceeded ||
		TaskRollbackState(rollback) == NoneRollback
}

// getRollback returns the rollback state (internal use)
func (s *Subtask) getRollback() string {
	return s.subtask.Rollback
}

func (s *Subtask) hasRollbackExecutor() bool {
	return s.rollbackProvider != nil
}

// getRetryInterval returns the retry interval (internal use)
func (s *Subtask) getRetryInterval() int32 {
	return s.subtask.RetryInterval
}

// getLastRunTime returns the last run time (internal use)
func (s *Subtask) getLastRunTime() *basic.Time {
	return &s.subtask.LastRunTime
}

// getModel returns the underlying model (internal use)
func (s *Subtask) getModel() *model.Subtask {
	return &s.subtask
}

// ===== Task methods =====

// GetID returns the task ID
func (t *Task) GetID() string {
	return t.task.ID
}

// GetTaskName returns the task name
func (t *Task) GetTaskName() string {
	return t.task.TaskName
}

// SetRequestID sets the request ID
func (t *Task) SetRequestID(requestID string) *Task {
	t.task.RequestID = requestID
	return t
}

// SetInput sets the task input
func (t *Task) SetInput(content interface{}) *Task {
	_tmp, _ := tools.ToByte(content)
	t.task.Input = string(_tmp)
	return t
}

// GetInput returns the task input
func (t *Task) GetInput() string {
	return t.task.Input
}

// SetDescription sets the description
func (t *Task) SetDescription(description string) *Task {
	t.task.Description = description
	return t
}

// SetExecuteTime sets the execution time
func (t *Task) SetExecuteTime(executeTime time.Time) *Task {
	t.task.ExecuteTime = basic.Time(executeTime)
	return t
}

// SetAffinityType sets the affinity type
func (t *Task) SetAffinityType(affinityType TaskAffinityType) *Task {
	t.task.AffinityType = string(affinityType)
	return t
}

// SetUrgent marks the task as urgent
func (t *Task) SetUrgent() *Task {
	t.task.Urgent = true
	return t
}

// getAffinityType returns the affinity type (internal use)
func (t *Task) getAffinityType() TaskAffinityType {
	return TaskAffinityType(t.task.AffinityType)
}

// getPrimaryWorker returns the primary worker node (internal use)
func (t *Task) getPrimaryWorker() string {
	return t.task.PrimaryWorker
}

// SetCallback sets the DAG callback
func (t *Task) SetCallback(callback DAGCallback) *Task {
	t.callback = callback
	return t
}

// SetRollbackStrategy sets the rollback strategy
func (t *Task) SetRollbackStrategy(strategy RollbackStrategy) *Task {
	t.rollbackStrategy = strategy
	return t
}

// SetCustomRollbackFunc sets a custom rollback function
func (t *Task) SetCustomRollbackFunc(fn func(completed []string, failed string) []string) *Task {
	t.customRollback = fn
	t.rollbackStrategy = StrategyRollbackCustom
	// Register to global registry (cluster framework: dispatcher restores Task from DB)
	registerCustomRollback(t.task.TaskName, fn)
	return t
}

// AddSubtask adds a subtask to the Task.
// If the Subtask has an executor bound via SetExecutor, it will be automatically registered to the executorManager.
// It is also registered to the global registry to ensure the cluster receiver can find the provider.
// Note: providers with the same taskName+subTaskName will be overwritten by later registrations (global registry override semantics);
// different Task instances with the same taskName should register identical providers.
func (t *Task) AddSubtask(subtask *Subtask) error {
	err := t.dag.AddNode(subtask.GetID(), subtask.triggerMode)
	if err != nil {
		return err
	}
	// Sync Subtask configuration to dagNode
	if node := t.dag.GetNode(subtask.GetID()); node != nil {
		node.priority = subtask.priority
	}
	t.subtaskMap[subtask.GetID()] = subtask
	// Auto-register executor (provider directly bound on Subtask)
	if subtask.provider != nil {
		t.em.registerProvider(t.task.TaskName, subtask.GetName(), subtask.provider)
		registerProvider(t.task.TaskName, subtask.GetName(), subtask.provider)
		// Extract type info to DAG node for compile-time type checking
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
	// Auto-register pre/post processors (needed when cluster receiver restores from DB)
	if subtask.preProcessor != nil {
		registerPreProcessor(t.task.TaskName, subtask.GetName(), subtask.preProcessor)
	}
	if subtask.postProcessor != nil {
		registerPostProcessor(t.task.TaskName, subtask.GetName(), subtask.postProcessor)
	}
	return nil
}

// AddControlEdge adds a control dependency edge (controls execution order only, no data passing)
func (t *Task) AddControlEdge(src, dst *Subtask) error {
	return t.dag.AddEdge(src.GetID(), dst.GetID(), ControlEdge)
}

// AddDataEdge adds a data dependency edge (passes data, can be used with AddControlEdge)
func (t *Task) AddDataEdge(src, dst *Subtask, mappings ...*FieldMapping) error {
	return t.dag.AddEdge(src.GetID(), dst.GetID(), DataEdge, mappings...)
}

// AddEdge adds a dependency edge that includes both control and data
func (t *Task) AddEdge(src, dst *Subtask, mappings ...*FieldMapping) error {
	return t.dag.AddEdge(src.GetID(), dst.GetID(), ControlAndDataEdge, mappings...)
}

// AddBranch adds a conditional branch.
// EndNodes and Condition/ConditionProvider return values support both subtask name and ID.
// Internally, names are automatically resolved to IDs, transparent to the DAG layer.
// If the Branch uses a ConditionProvider, it is automatically registered to the global registry for DB restoration.
func (t *Task) AddBranch(node *Subtask, branch *Branch) error {
	// Resolve EndNodes: name -> ID (already-ID keys remain unchanged)
	resolvedEndNodes := make(map[string]bool, len(branch.EndNodes))
	for key := range branch.EndNodes {
		resolvedEndNodes[t.resolveSubtaskKey(key)] = true
	}
	branch.EndNodes = resolvedEndNodes

	// Wrap Condition: translate returned name to ID
	if branch.Condition != nil {
		origCond := branch.Condition
		branch.Condition = func(ctx interface{}, input any) (string, error) {
			selected, err := origCond(ctx, input)
			if err != nil {
				return "", err
			}
			return t.resolveSubtaskKey(selected), nil
		}
	}

	// Wrap ConditionProvider: translate returned name to ID
	if branch.ConditionProvider != nil {
		origProvider := branch.ConditionProvider
		branch.ConditionProvider = &nameResolvingProvider{
			inner: origProvider,
			resolve: func(nameOrID string) string {
				return t.resolveSubtaskKey(nameOrID)
			},
		}
	}

	registerBranch(t.task.TaskName, node.GetID(), branch)
	if branch.ConditionProvider != nil {
		registerBranchConditionProvider(t.task.TaskName, node.GetID(), branch.ConditionProvider)
	}
	return t.dag.AddBranch(node.GetID(), branch)
}

// resolveSubtaskKey resolves a name or ID to the ID used internally by the DAG.
// If key is already a subtask ID, it is returned directly; otherwise it is looked up by name.
// If not found, the key is returned as-is (the caller is responsible for subsequent validation).
func (t *Task) resolveSubtaskKey(key string) string {
	// Check if it is already an ID first
	if _, exists := t.subtaskMap[key]; exists {
		return key
	}
	// Look up by name
	for id, s := range t.subtaskMap {
		if s.GetName() == key {
			return id
		}
	}
	return key
}

// nameResolvingProvider wraps an ExecutorProvider, translating the name returned by Execute to an ID
type nameResolvingProvider struct {
	inner   executor.ExecutorProvider
	resolve func(string) string
}

func (p *nameResolvingProvider) Execute(ctx context.Context, data *executor.TaskData) (any, error) {
	result, err := p.inner.Execute(ctx, data)
	if err != nil {
		return nil, err
	}
	if s, ok := result.(string); ok {
		return p.resolve(s), nil
	}
	return result, nil
}

func (p *nameResolvingProvider) Protocol() executor.ExecutorProtocol {
	return p.inner.Protocol()
}

// addSubtaskGraph adds a subgraph node (internal use)
func (t *Task) addSubtaskGraph(key string, subTask *Task) error {
	if _, exists := t.subGraphs[key]; exists {
		return fmt.Errorf("subtask graph key already exists: %s", key)
	}
	t.subGraphs[key] = subTask
	// Add subgraph as a special node to the main graph
	return t.dag.AddNode(key, AllPredecessor)
}

// Compile compiles the DAG
func (t *Task) Compile() (*compiledDAG, error) {
	compiled, err := t.dag.Compile()
	if err != nil {
		return nil, err
	}
	t.compiled = compiled
	return compiled, nil
}

// getCompiled returns the compiled DAG (internal use)
func (t *Task) getCompiled() *compiledDAG {
	return t.compiled
}

// NextSubTasks returns the next executable subtask list
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

	// Sort by priority descending
	sort.Slice(result, func(i, j int) bool {
		return result[i].priority > result[j].priority
	})

	return result
}

// UpdateSubtaskState updates the subtask state (non-destructive)
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

	// Update successor node channel state
	if t.compiled != nil {
		switch state {
		case NodeSucceeded:
			// Notify all successor nodes: control dependency ready
			for _, succKey := range t.dag.controlAdj[subtaskID] {
				if ch := t.compiled.GetChannel(succKey); ch != nil {
					ch.reportDependencies([]string{subtaskID})
				}
			}
			// Notify all successor nodes: data ready
			for _, succKey := range t.dag.dataAdj[subtaskID] {
				if ch := t.compiled.GetChannel(succKey); ch != nil {
					ch.reportValues(map[string]any{subtaskID: subtask.subtask.Output})
				}
			}
		case NodeSkipped:
			// Notify all successor nodes: predecessor skipped
			for _, succKey := range t.dag.controlAdj[subtaskID] {
				if ch := t.compiled.GetChannel(succKey); ch != nil {
					ch.reportSkip([]string{subtaskID})
				}
			}
			// Mark data successors as skipped too
			for _, succKey := range t.dag.dataAdj[subtaskID] {
				if ch := t.compiled.GetChannel(succKey); ch != nil {
					ch.reportValues(map[string]any{subtaskID: nil})
				}
			}
		}
	}

	return nil
}

// SkipSubtask skips a subtask
func (t *Task) SkipSubtask(subtaskID string) error {
	return t.UpdateSubtaskState(subtaskID, NodeSkipped)
}

// IsFinished checks whether the task is finished
func (t *Task) IsFinished() bool {
	if t.task.State == string(TaskFailed) || t.task.State == string(TaskSucceeded) {
		return true
	}
	if t.compiled != nil {
		return t.compiled.IsAllFinished()
	}
	return false
}

// Size returns the number of subtasks
func (t *Task) Size() int {
	return t.dag.Order()
}

// Graph returns the topological sort text representation
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

		// Get successor nodes
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

// GraphDOT returns the DOT format representation
func (t *Task) GraphDOT() string {
	if t.compiled == nil {
		return "// DAG not compiled"
	}
	return t.compiled.GraphDOT()
}

// GraphMermaid returns the Mermaid format representation
func (t *Task) GraphMermaid() string {
	if t.compiled == nil {
		return "// DAG not compiled"
	}
	return t.compiled.GraphMermaid()
}

// RegisterTaskExecutor registers a task executor callback (FinishedTask/FailedTask).
// Note: subtask executors (ExecutorProvider) are now bound via Subtask.SetExecutor() and auto-registered during AddSubtask.
func (t *Task) RegisterTaskExecutor(taskExecutor TaskExecutor) {
	t.em.registerTaskExecutor(taskExecutor)
	// Also register to global registry (cluster framework: needed when receiver reads from DB)
	registerTaskExecutor(taskExecutor)
}

// RegisterTaskExecutor package-level function: registers TaskExecutor to the global registry only
func RegisterTaskExecutor(taskExecutor TaskExecutor) {
	registerTaskExecutor(taskExecutor)
}

// RegisterBranchCondition registers a branch condition
func (t *Task) RegisterBranchCondition(nodeKey, branchKey string, condition func(ctx interface{}, input any) (string, error)) {
	t.em.registerBranchCondition(nodeKey, branchKey, condition)
}

// getProvider returns the subtask executor (internal use; users should use Subtask.GetExecutor())
func (t *Task) getProvider(taskName, subTaskName string) executor.ExecutorProvider {
	return t.em.getProvider(taskName, subTaskName)
}

// getBranchCondition returns the branch condition (internal use)
func (t *Task) getBranchCondition(nodeKey, branchKey string) func(ctx interface{}, input any) (string, error) {
	return t.em.getBranchCondition(nodeKey, branchKey)
}

// getCallback returns the callback (internal use)
func (t *Task) getCallback() DAGCallback {
	return t.callback
}

// getRollbackStrategy returns the rollback strategy (internal use)
func (t *Task) getRollbackStrategy() RollbackStrategy {
	return t.rollbackStrategy
}

// GetRollbackableSubtasks returns the list of subtask IDs that need to be rolled back.
// Returns subtask IDs based on the rollback strategy.
func (t *Task) GetRollbackableSubtasks() []string {
	var completed []string
	var failedList []string

	// Collect completed and failed subtasks
	for _, subtask := range t.subtaskMap {
		if subtask.subtask.State == string(TaskSucceeded) {
			completed = append(completed, subtask.GetID())
		} else if subtask.subtask.State == string(TaskFailed) {
			failedList = append(failedList, subtask.GetID())
		}
	}

	switch t.rollbackStrategy {
	case StrategyRollbackAll:
		// Return all completed and failed subtasks (reverse topo order), ensuring failed subtasks are also rolled back
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
		// Return only failed subtasks
		if len(failedList) > 0 {
			return failedList
		}
		return nil

	case StrategyRollbackCustom:
		// Use custom rollback function
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

// getCustomRollbackFunc returns the custom rollback function (internal use)
func (t *Task) getCustomRollbackFunc() func(completed []string, failed string) []string {
	return t.customRollback
}

// DAGCallback is the DAG lifecycle callback interface
type DAGCallback interface {
	OnSubtaskStart(ctx interface{}, key string, input any)
	OnSubtaskComplete(ctx interface{}, key string, output any)
	OnSubtaskFailed(ctx interface{}, key string, err error)
	OnSubtaskSkipped(ctx interface{}, key string)
	OnBranchSelected(ctx interface{}, fromNode string, selectedNode string)
}

// NoOpCallback is a no-op callback implementation
type NoOpCallback struct{}

func (n *NoOpCallback) OnSubtaskStart(ctx interface{}, key string, input any)                  {}
func (n *NoOpCallback) OnSubtaskComplete(ctx interface{}, key string, output any)              {}
func (n *NoOpCallback) OnSubtaskFailed(ctx interface{}, key string, err error)                 {}
func (n *NoOpCallback) OnSubtaskSkipped(ctx interface{}, key string)                           {}
func (n *NoOpCallback) OnBranchSelected(ctx interface{}, fromNode string, selectedNode string) {}

// ===== TaskExecutor interface =====

// TaskExecutor is the task executor interface
type TaskExecutor interface {
	Name() string
	//// FinishedTask callback when task completes
	//FinishedTask(data *TaskData) error
	//// FailedTask callback when task fails
	//FailedTask(data *TaskData) error
}

// executorManager is the executor manager (instance-level)
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

// registerProviders batch-registers subtask executors
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

// registerProvider registers a single subtask executor
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

// getProvider looks up the subtask executor (instance-level)
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

// TaskData is the data passing structure during task execution
type TaskData struct {
	RequestId   string
	TaskId      string
	SubTaskId   string
	Input       string
	MergedInput map[string]any // Merged input after field mapping
	Subtasks    map[string]Output
}

// convert2Bean converts Task to database model (including edge data and edge info)
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

	// Build reverse mapping: nodeKey -> [predecessor nodeKey list]
	predecessors := make(map[string]map[string]struct{})
	for nodeKey := range t.subtaskMap {
		predecessors[nodeKey] = make(map[string]struct{})
	}
	// Collect predecessors from control edges (from -> to, so to's predecessors include from. Used for PreSubtaskID backward compatibility)
	// Note: the exec phase filters control predecessors via the task_edge table and does not pass data
	for from, successors := range t.dag.controlAdj {
		for _, to := range successors {
			predecessors[to][from] = struct{}{}
		}
	}
	// Collect predecessors from data edges
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
		// Serialize branch config to settings JSON
		if branches, ok := t.dag.branches[subtask.GetID()]; ok && len(branches) > 0 {
			settings := SubtaskSettings{}
			for _, br := range branches {
				endNodeNames := make([]string, 0, len(br.EndNodes))
				for k := range br.EndNodes {
					if s, exists := t.subtaskMap[k]; exists {
						endNodeNames = append(endNodeNames, s.GetName())
					}
				}
				providerName := ""
				if br.ConditionProvider != nil {
					providerName = string(br.ConditionProvider.Protocol())
				}
				settings.BranchConfig = &BranchConfig{
					EndNodes:          endNodeNames,
					ConditionProvider: providerName,
				}
			}
			bean.Settings = tools.ToJson(settings)
		}
		subtaskBeans = append(subtaskBeans, bean)
	}

	// Build edge table
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

// initByBean initializes Task from database model.
//
// Note: the following info has been restored from the task_edge table (edge type/field mappings) and subtask table (triggerMode/priority/timeout):
//   - Edge type (ControlEdge/DataEdge/ControlAndDataEdge) restored from task_edge.edge_type
//   - triggerMode (AllPredecessor/AnyPredecessor) restored from subtask.trigger_mode
//   - priority restored from subtask.priority
//   - timeout restored from subtask.timeout
//   - rollbackStrategy restored from task.rollback_strategy
//   - fieldMappings restored from task_edge.field_mappings JSON
//
// Branch condition nodes (Branch.Condition) and executors (ExecutorProvider) are code-level concepts, restored via the global registry.
func (t *Task) initByBean(taskBean *model.Task, subtaskBeans []model.Subtask, edges []model.TaskEdge) (*Task, error) {
	t.task = *taskBean
	t.rollbackStrategy = rollbackStrategyFromDBString(taskBean.RollbackStrategy)
	t.subtaskMap = make(map[string]*Subtask)
	// Restore custom rollback function from global registry (cluster framework)
	if fn := getCustomRollback(taskBean.TaskName); fn != nil {
		t.customRollback = fn
	}

	// Initialize executorManager and restore executors from global registry
	t.em = newExecutorManager()
	if te := getTaskExecutor(taskBean.TaskName); te != nil {
		t.em.registerTaskExecutor(te)
	}

	for i := range subtaskBeans {
		subtask := &Subtask{
			subtask: subtaskBeans[i],
		}
		// Restore triggerMode, priority, timeout from DB
		subtask.triggerMode = triggerModeFromDBString(subtaskBeans[i].TriggerMode)
		subtask.priority = subtaskBeans[i].Priority
		subtask.timeout = time.Duration(subtaskBeans[i].Timeout) * time.Second
		t.subtaskMap[subtask.GetID()] = subtask

		// Restore provider and rollbackProvider from global registry
		if p := getProvider(taskBean.TaskName, subtask.GetName()); p != nil {
			subtask.provider = p
			t.em.registerProvider(taskBean.TaskName, subtask.GetName(), p)
		}
		if p := getRollbackProvider(taskBean.TaskName, subtask.GetName()); p != nil {
			subtask.rollbackProvider = p
			t.em.registerRollbackProvider(taskBean.TaskName, subtask.GetName(), p)
		}
		// Restore preProcessor and postProcessor from global registry
		if p := getPreProcessor(taskBean.TaskName, subtask.GetName()); p != nil {
			subtask.preProcessor = p
		}
		if p := getPostProcessor(taskBean.TaskName, subtask.GetName()); p != nil {
			subtask.postProcessor = p
		}
	}

	// Rebuild DAG
	t.dag = NewDAGGraph()
	for _, subtask := range t.subtaskMap {
		if err := t.dag.AddNode(subtask.GetID(), subtask.triggerMode); err != nil {
			return nil, err
		}
		// Restore dagNode's priority, timeout, preProcessor, postProcessor from Subtask
		if node := t.dag.GetNode(subtask.GetID()); node != nil {
			node.priority = subtask.priority
			node.timeout = subtask.timeout
			node.preProcessor = subtask.preProcessor
			node.postProcessor = subtask.postProcessor
		}
	}
	// Rebuild edges (prefer exact types from task_edge table, fall back to pre_subtask_id inference)
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
		// Fall back to pre_subtask_id inference when no task_edge records exist (backward compatibility)
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
	// Restore branch info from global registry
	// Prefer ConditionProvider (persistable), fall back to Condition closure (backward compatibility)
	if registeredBranches := getRegisteredBranches(taskBean.TaskName); len(registeredBranches) > 0 {
		for nodeKey, branches := range registeredBranches {
			for i, br := range branches {
				// If Branch has a ConditionProvider, restore from the new registry
				if br.ConditionProvider == nil && br.Condition == nil {
					if p := getBranchConditionProvider(taskBean.TaskName, nodeKey, i); p != nil {
						br.ConditionProvider = p
					}
				}
			}
			t.dag.branches[nodeKey] = branches
		}
	}
	// Compile DAG
	if _, err := t.Compile(); err != nil {
		return nil, err
	}
	// Restore DAG node state based on DB state (critical: otherwise completed nodes cannot notify successors)
	for _, subtask := range t.subtaskMap {
		switch subtask.subtask.State {
		case string(TaskSucceeded):
			_ = t.dag.UpdateNodeState(subtask.GetID(), NodeSucceeded)
			if ch := t.compiled.GetChannel(subtask.GetID()); ch != nil {
				ch.reportDependencies(nil)
				ch.reportValues(map[string]any{subtask.GetID(): subtask.subtask.Output})
			}
			// Notify successor nodes
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

// getState returns the task state (internal use)
func (t *Task) getState() string {
	return string(t.task.State)
}

// ===== DB string conversion helpers =====

// toDBString converts EdgeType to a DB storage string
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

// edgeTypeFromDBString restores EdgeType from a DB string
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

// toDBString converts NodeTriggerMode to a DB storage string
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

// triggerModeFromDBString restores NodeTriggerMode from a DB string
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

// toDBString converts RollbackStrategy to a DB storage string
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

// rollbackStrategyFromDBString restores RollbackStrategy from a DB string
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
