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
	"errors"
	"reflect"
	"strings"
	"time"

	"github.com/caiflower/common-tools/taskx/executor"
)

// EdgeType 边的类型
type EdgeType int

const (
	// ControlEdge 控制依赖边，决定执行顺序
	ControlEdge EdgeType = iota
	// DataEdge 数据依赖边，决定数据流向
	DataEdge
	// ControlAndDataEdge 同时是控制依赖和数据依赖
	ControlAndDataEdge
)

func (e EdgeType) String() string {
	switch e {
	case ControlEdge:
		return "control"
	case DataEdge:
		return "data"
	case ControlAndDataEdge:
		return "control+data"
	default:
		return "unknown"
	}
}

// NodeState 节点状态
type NodeState int

const (
	// NodePending 节点等待执行
	NodePending NodeState = iota
	// NodeRunning 节点正在执行
	NodeRunning
	// NodeSucceeded 节点执行成功
	NodeSucceeded
	// NodeFailed 节点执行失败
	NodeFailed
	// NodeSkipped 节点被跳过
	NodeSkipped
)

func (s NodeState) String() string {
	switch s {
	case NodePending:
		return "Pending"
	case NodeRunning:
		return "Running"
	case NodeSucceeded:
		return "Succeeded"
	case NodeFailed:
		return "Failed"
	case NodeSkipped:
		return "Skipped"
	default:
		return "Unknown"
	}
}

// IsFinished 判断节点是否已完成（成功或失败）
func (s NodeState) IsFinished() bool {
	return s == NodeSucceeded || s == NodeFailed
}

// NodeTriggerMode 节点触发模式
type NodeTriggerMode int

const (
	// AllPredecessor 所有前驱完成才触发
	AllPredecessor NodeTriggerMode = iota
	// AnyPredecessor 任一前驱完成即触发
	AnyPredecessor
)

func (m NodeTriggerMode) String() string {
	switch m {
	case AllPredecessor:
		return "all_predecessor"
	case AnyPredecessor:
		return "any_predecessor"
	default:
		return "unknown"
	}
}

// dependencyState 依赖状态
type dependencyState int

const (
	// depWaiting 等待前驱完成
	depWaiting dependencyState = iota
	// depReady 前驱已完成
	depReady
	// depSkipped 前驱已跳过
	depSkipped
)

func (d dependencyState) String() string {
	switch d {
	case depWaiting:
		return "waiting"
	case depReady:
		return "ready"
	case depSkipped:
		return "skipped"
	default:
		return "unknown"
	}
}

// RollbackStrategy 回滚策略
type RollbackStrategy int

const (
	// StrategyRollbackAll 回滚所有已执行子任务（默认）
	StrategyRollbackAll RollbackStrategy = iota
	// StrategyRollbackFailed 只回滚失败子任务
	StrategyRollbackFailed
	// StrategyRollbackCustom 自定义回滚逻辑
	StrategyRollbackCustom
)

func (s RollbackStrategy) String() string {
	switch s {
	case StrategyRollbackAll:
		return "rollback_all"
	case StrategyRollbackFailed:
		return "rollback_failed"
	case StrategyRollbackCustom:
		return "rollback_custom"
	default:
		return "unknown"
	}
}

// FieldMapping 字段映射，定义源字段到目标字段的映射关系
type FieldMapping struct {
	SourceField string // 源字段路径，如 "result.name"
	TargetField string // 目标字段路径，如 "displayName"
}

// FieldPath 字段路径，用于 SetStaticValue
type FieldPath []string

func (f FieldPath) join() string {
	return strings.Join(f, ".")
}

// Processor 处理器函数类型，用于 Pre/Post Processor
type Processor func(ctx interface{}, data any) (any, error)

// SubtaskSettings 子任务的可扩展 JSON 配置，存储在 DB 的 settings 字段中
type SubtaskSettings struct {
	BranchConfig *BranchConfig `json:"branch_config,omitempty"`
}

// BranchConfig 分支配置（持久化到 DB，从全局注册表恢复 ConditionProvider）
type BranchConfig struct {
	EndNodes          []string `json:"end_nodes"`          // 分支目标节点名称列表
	ConditionProvider string   `json:"condition_provider"` // 条件 provider 标识（用于全局注册表查找）
}

// Branch 条件分支
type Branch struct {
	// ConditionProvider 可持久化的条件执行器，Execute 返回选中的目标节点 key (string)
	ConditionProvider executor.ExecutorProvider
	// Condition 向后兼容的闭包条件函数，不持久化到 DB
	Condition func(ctx interface{}, input any) (string, error)
	// EndNodes 分支目标节点集合
	EndNodes map[string]bool
}

// NewBranch 创建基于 ExecutorProvider 的条件分支（可持久化）
func NewBranch(provider executor.ExecutorProvider, endNodes map[string]bool) *Branch {
	return &Branch{
		ConditionProvider: provider,
		EndNodes:          endNodes,
	}
}

// NewBranchFunc 创建基于闭包的条件分支（向后兼容，不持久化）
func NewBranchFunc(condition func(ctx interface{}, input any) (string, error), endNodes map[string]bool) *Branch {
	return &Branch{
		Condition: condition,
		EndNodes:  endNodes,
	}
}

// dagNode DAG 节点
type dagNode struct {
	key           string
	triggerMode   NodeTriggerMode
	state         NodeState
	priority      int
	timeout       time.Duration
	preProcessor  Processor
	postProcessor Processor

	// 编译时确定的类型信息
	inputType  reflect.Type
	outputType reflect.Type
}

// NewDAGNode 创建 DAG 节点
func NewDAGNode(key string, triggerMode NodeTriggerMode) *dagNode {
	return &dagNode{
		key:         key,
		triggerMode: triggerMode,
		state:       NodePending,
		priority:    0,
	}
}

// dagEdge DAG 边
type dagEdge struct {
	from     string
	to       string
	edgeType EdgeType
	mappings []*FieldMapping
}

// dagGraph DAG 图结构
type dagGraph struct {
	nodes map[string]*dagNode
	edges []*dagEdge

	// 邻接表（编译后不可变）
	controlAdj  map[string][]string // 控制依赖邻接表：key -> [后继列表]
	controlPred map[string][]string // 控制依赖前驱表：key -> [前驱列表]
	dataAdj     map[string][]string // 数据依赖邻接表：key -> [后继列表]
	dataPred    map[string][]string // 数据依赖前驱表：key -> [前驱列表]

	branches map[string][]*Branch // 节点的分支列表

	compiled bool // 是否已编译
}

// NewDAGGraph 创建新的 DAG 图
func NewDAGGraph() *dagGraph {
	return &dagGraph{
		nodes:       make(map[string]*dagNode),
		edges:       make([]*dagEdge, 0),
		controlAdj:  make(map[string][]string),
		controlPred: make(map[string][]string),
		dataAdj:     make(map[string][]string),
		dataPred:    make(map[string][]string),
		branches:    make(map[string][]*Branch),
	}
}

// AddNode 添加节点到图中
func (g *dagGraph) AddNode(key string, triggerMode NodeTriggerMode) error {
	if g.compiled {
		return ErrGraphCompiled
	}
	if key == "" {
		return errors.New("node key cannot be empty")
	}
	if _, exists := g.nodes[key]; exists {
		return errors.New("node already exists: " + key)
	}
	g.nodes[key] = NewDAGNode(key, triggerMode)
	return nil
}

// GetNode 获取节点
func (g *dagGraph) GetNode(key string) *dagNode {
	return g.nodes[key]
}

// GetNodesByState 根据状态获取节点列表
func (g *dagGraph) GetNodesByState(state NodeState) []*dagNode {
	var result []*dagNode
	for _, node := range g.nodes {
		if node.state == state {
			result = append(result, node)
		}
	}
	return result
}

// AddEdge 添加边到图中
func (g *dagGraph) AddEdge(from, to string, edgeType EdgeType, mappings ...*FieldMapping) error {
	if g.compiled {
		return ErrGraphCompiled
	}
	if from == "" || to == "" {
		return errors.New("edge from/to cannot be empty")
	}
	if _, exists := g.nodes[from]; !exists {
		return errors.New("node not found: " + from)
	}
	if _, exists := g.nodes[to]; !exists {
		return errors.New("node not found: " + to)
	}

	edge := &dagEdge{
		from:     from,
		to:       to,
		edgeType: edgeType,
		mappings: mappings,
	}
	g.edges = append(g.edges, edge)

	// 更新邻接表
	switch edgeType {
	case ControlEdge:
		g.controlAdj[from] = append(g.controlAdj[from], to)
		g.controlPred[to] = append(g.controlPred[to], from)
	case DataEdge:
		g.dataAdj[from] = append(g.dataAdj[from], to)
		g.dataPred[to] = append(g.dataPred[to], from)
	case ControlAndDataEdge:
		g.controlAdj[from] = append(g.controlAdj[from], to)
		g.controlPred[to] = append(g.controlPred[to], from)
		g.dataAdj[from] = append(g.dataAdj[from], to)
		g.dataPred[to] = append(g.dataPred[to], from)
	}

	return nil
}

// AddBranch 添加分支
func (g *dagGraph) AddBranch(nodeKey string, branch *Branch) error {
	if g.compiled {
		return ErrGraphCompiled
	}
	if _, exists := g.nodes[nodeKey]; !exists {
		return errors.New("node not found: " + nodeKey)
	}
	g.branches[nodeKey] = append(g.branches[nodeKey], branch)
	return nil
}

// UpdateNodeState 更新节点状态（非破坏性）
func (g *dagGraph) UpdateNodeState(key string, state NodeState) error {
	node := g.nodes[key]
	if node == nil {
		return errors.New("node not found: " + key)
	}
	node.state = state
	return nil
}

// TopologicalSort 拓扑排序，返回节点 key 的排序列表
func (g *dagGraph) TopologicalSort() ([]string, error) {
	inDegree := make(map[string]int)
	for _, node := range g.nodes {
		// 合并控制前驱和数据前驱的入度
		predSet := make(map[string]struct{})
		for _, p := range g.controlPred[node.key] {
			predSet[p] = struct{}{}
		}
		for _, p := range g.dataPred[node.key] {
			predSet[p] = struct{}{}
		}
		inDegree[node.key] = len(predSet)
	}

	var queue []string
	for key, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, key)
		}
	}

	var result []string
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		result = append(result, node)

		// 合并控制后继和数据后继
		succSet := make(map[string]struct{})
		for _, s := range g.controlAdj[node] {
			succSet[s] = struct{}{}
		}
		for _, s := range g.dataAdj[node] {
			succSet[s] = struct{}{}
		}
		for successor := range succSet {
			inDegree[successor]--
			if inDegree[successor] == 0 {
				queue = append(queue, successor)
			}
		}
	}

	if len(result) != len(g.nodes) {
		return nil, errors.New("graph contains cycle")
	}

	return result, nil
}

// GetStartNodes 获取起始节点（无控制前驱的节点）
func (g *dagGraph) GetStartNodes() []string {
	var startNodes []string
	for key := range g.nodes {
		if len(g.controlPred[key]) == 0 {
			startNodes = append(startNodes, key)
		}
	}
	return startNodes
}

// GetEndNodes 获取终止节点（无控制后继的节点）
func (g *dagGraph) GetEndNodes() []string {
	var endNodes []string
	for key := range g.nodes {
		if len(g.controlAdj[key]) == 0 {
			endNodes = append(endNodes, key)
		}
	}
	return endNodes
}

// Order 返回节点数量
func (g *dagGraph) Order() int {
	return len(g.nodes)
}

// ErrGraphCompiled 图已编译，无法修改
var ErrGraphCompiled = errors.New("graph has been compiled, cannot be modified")

// ErrGraphNotCompiled 图未编译
var ErrGraphNotCompiled = errors.New("graph has not been compiled")
