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
	"reflect"
	"sort"
	"strings"
)

// compiledDAG 编译后的 DAG，包含不可变的图结构和通道
type compiledDAG struct {
	graph      *dagGraph
	channels   map[string]*dagChannel // 每个节点的通道
	branches   map[string][]*Branch   // 节点的分支
	startNodes []string
	endNodes   []string

	// 用于执行顺序优化的拓扑排序结果
	topoOrder []string
}

// Compile 编译 DAG，返回编译后的结果
// 编译时进行以下校验：
// 1. 环检测
// 2. 起始节点和终止节点检查
// 3. 分支目标节点存在性校验
// 4. 类型兼容性校验
func (g *dagGraph) Compile() (*compiledDAG, error) {
	if g.compiled {
		return nil, ErrGraphCompiled
	}

	// 校验环
	if err := validateDAG(g); err != nil {
		return nil, err
	}

	// 校验起始节点
	startNodes := g.GetStartNodes()
	if len(startNodes) == 0 {
		return nil, fmt.Errorf("no start nodes found (no nodes with zero in-degree)")
	}

	// 校验终止节点
	endNodes := g.GetEndNodes()
	if len(endNodes) == 0 {
		return nil, fmt.Errorf("no end nodes found (no nodes with zero out-degree)")
	}

	// 校验分支目标节点
	if err := validateBranches(g); err != nil {
		return nil, err
	}

	// 校验数据边类型兼容性
	if err := validateTypeCompatibility(g); err != nil {
		return nil, err
	}

	// 拓扑排序
	topoOrder, err := g.TopologicalSort()
	if err != nil {
		return nil, fmt.Errorf("topological sort failed: %w", err)
	}

	// 构建节点通道
	channels := make(map[string]*dagChannel)
	for key, node := range g.nodes {
		controlPreds := g.controlPred[key]
		dataPreds := g.dataPred[key]
		channels[key] = newDAGChannel(key, controlPreds, dataPreds, node.triggerMode)
	}

	result := &compiledDAG{
		graph:      g,
		channels:   channels,
		branches:   g.branches,
		startNodes: startNodes,
		endNodes:   endNodes,
		topoOrder:  topoOrder,
	}

	// 标记为已编译
	g.compiled = true

	return result, nil
}

// validateDAG 校验 DAG 是否有环
func validateDAG(g *dagGraph) error {
	inDegree := make(map[string]int)
	for key := range g.nodes {
		// 合并控制前驱和数据前驱的入度
		predSet := make(map[string]struct{})
		for _, p := range g.controlPred[key] {
			predSet[p] = struct{}{}
		}
		for _, p := range g.dataPred[key] {
			predSet[p] = struct{}{}
		}
		inDegree[key] = len(predSet)
	}

	var queue []string
	for key, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, key)
		}
	}

	var sorted []string
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		sorted = append(sorted, node)

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

	if len(sorted) != len(g.nodes) {
		// 找出环中的节点
		var loopNodes []string
		for key, degree := range inDegree {
			if degree > 0 {
				loopNodes = append(loopNodes, key)
			}
		}
		return fmt.Errorf("%w: %s", ErrDAGInvalidLoop, formatLoopNodes(loopNodes))
	}

	return nil
}

// validateBranches 校验分支目标节点是否存在
func validateBranches(g *dagGraph) error {
	for nodeKey, branches := range g.branches {
		for _, branch := range branches {
			for endNode := range branch.EndNodes {
				if _, exists := g.nodes[endNode]; !exists {
					return fmt.Errorf("branch from node %s has invalid end node: %s", nodeKey, endNode)
				}
			}
		}
	}
	return nil
}

// validateTypeCompatibility 校验数据边的类型兼容性
// 对于包含数据流的边（DataEdge 或 ControlAndDataEdge），检查源节点的输出类型是否可赋值给目标节点的输入类型
// 如果任一端类型信息缺失（非 TypedProvider），则跳过该校验
func validateTypeCompatibility(g *dagGraph) error {
	for _, edge := range g.edges {
		// 只校验包含数据流的边
		if edge.edgeType == ControlEdge {
			continue
		}

		srcNode := g.nodes[edge.from]
		dstNode := g.nodes[edge.to]
		if srcNode == nil || dstNode == nil {
			continue
		}

		srcOutputType := srcNode.outputType
		dstInputType := dstNode.inputType

		// 类型信息缺失则跳过（非 TypedProvider 的执行器）
		if srcOutputType == nil || dstInputType == nil {
			continue
		}

		// 类型完全匹配
		if srcOutputType == dstInputType {
			continue
		}

		// 源输出可赋值给目标输入
		if srcOutputType.AssignableTo(dstInputType) {
			continue
		}

		// 特殊处理：map[string]any 作为输入类型时，接受任何 map[string]X 输出
		// 因为 map[string]any 是最宽泛的 map 类型，可以容纳任意值
		if dstInputType.Kind() == reflect.Map && srcOutputType.Kind() == reflect.Map {
			dstKeyType := dstInputType.Key()
			srcKeyType := srcOutputType.Key()
			if dstKeyType == srcKeyType && dstInputType.Elem().Kind() == reflect.Interface {
				continue
			}
		}

		return fmt.Errorf("type mismatch on edge %s -> %s: source output type %s is not compatible with destination input type %s",
			edge.from, edge.to, srcOutputType.String(), dstInputType.String())
	}
	return nil
}

// ErrDAGInvalidLoop DAG 包含环
var ErrDAGInvalidLoop = fmt.Errorf("DAG is invalid: contains loop")

// formatLoopNodes 格式化环节点列表
func formatLoopNodes(nodes []string) string {
	if len(nodes) == 0 {
		return ""
	}
	return "[" + strings.Join(nodes, " -> ") + "]"
}

// GetChannel 获取节点的通道
func (c *compiledDAG) GetChannel(nodeKey string) *dagChannel {
	return c.channels[nodeKey]
}

// GetBranches 获取节点的分支列表
func (c *compiledDAG) GetBranches(nodeKey string) []*Branch {
	return c.branches[nodeKey]
}

// GetBranchesMap 获取所有节点的分支映射
func (c *compiledDAG) GetBranchesMap() map[string][]*Branch {
	return c.branches
}

// GetStartNodes 获取起始节点列表
func (c *compiledDAG) GetStartNodes() []string {
	return c.startNodes
}

// GetEndNodes 获取终止节点列表
func (c *compiledDAG) GetEndNodes() []string {
	return c.endNodes
}

// GetExecutableNodes 获取当前可执行的节点列表
// 按优先级降序排列
func (c *compiledDAG) GetExecutableNodes() []*dagNode {
	var executables []*dagNode

	for key, channel := range c.channels {
		node := c.graph.nodes[key]
		if node.state != NodePending {
			continue
		}

		if channel.isReady() && channel.isDataReady() {
			executables = append(executables, node)
		}
	}

	// 按优先级降序排列
	sort.Slice(executables, func(i, j int) bool {
		return executables[i].priority > executables[j].priority
	})

	return executables
}

// IsAllFinished 判断 DAG 是否全部完成
func (c *compiledDAG) IsAllFinished() bool {
	for _, node := range c.graph.nodes {
		if !node.state.IsFinished() && node.state != NodeSkipped {
			return false
		}
	}
	return true
}

// GetTopoOrder 获取拓扑排序顺序
func (c *compiledDAG) GetTopoOrder() []string {
	return c.topoOrder
}

// GraphDOT 返回 Graphviz DOT 格式的图描述
func (c *compiledDAG) GraphDOT() string {
	var sb strings.Builder
	sb.WriteString("digraph DAG {\n")
	sb.WriteString("  rankdir=TB;\n")
	sb.WriteString("  node [shape=box];\n\n")

	// 写入节点
	for key, node := range c.graph.nodes {
		sb.WriteString(fmt.Sprintf("  \"%s\" [label=\"%s\\n(%s)\"];\n", key, key, node.state.String()))
	}

	sb.WriteString("\n")

	// 写入边
	for _, edge := range c.graph.edges {
		label := ""
		switch edge.edgeType {
		case ControlEdge:
			label = "control"
		case DataEdge:
			label = "data"
		case ControlAndDataEdge:
			label = "control+data"
		}

		if len(edge.mappings) > 0 {
			var mappingStrs []string
			for _, m := range edge.mappings {
				mappingStrs = append(mappingStrs, fmt.Sprintf("%s->%s", m.SourceField, m.TargetField))
			}
			label += ":" + strings.Join(mappingStrs, ",")
		}

		if label != "" {
			sb.WriteString(fmt.Sprintf("  \"%s\" -> \"%s\" [label=\"%s\"];\n", edge.from, edge.to, label))
		} else {
			sb.WriteString(fmt.Sprintf("  \"%s\" -> \"%s\";\n", edge.from, edge.to))
		}
	}

	sb.WriteString("}\n")
	return sb.String()
}

// GraphMermaid 返回 Mermaid 格式的图描述
func (c *compiledDAG) GraphMermaid() string {
	var sb strings.Builder
	sb.WriteString("```mermaid\n")
	sb.WriteString("graph TD\n")

	// 写入节点（只定义节点，不输出边）
	for key := range c.graph.nodes {
		sb.WriteString(fmt.Sprintf("    %s[%s]\n", key, key))
	}

	sb.WriteString("\n")

	// 写入边
	edgeMap := make(map[string]map[string]string) // from -> to -> label
	for _, edge := range c.graph.edges {
		if edgeMap[edge.from] == nil {
			edgeMap[edge.from] = make(map[string]string)
		}

		var labels []string
		switch edge.edgeType {
		case ControlEdge:
			labels = append(labels, "control")
		case DataEdge:
			labels = append(labels, "data")
		case ControlAndDataEdge:
			labels = append(labels, "control+data")
		}

		for _, m := range edge.mappings {
			labels = append(labels, fmt.Sprintf("%s→%s", m.SourceField, m.TargetField))
		}

		edgeMap[edge.from][edge.to] = strings.Join(labels, ",")
	}

	for from, tos := range edgeMap {
		for to, label := range tos {
			if label != "" {
				sb.WriteString(fmt.Sprintf("    %s -->|%s| %s\n", from, label, to))
			} else {
				sb.WriteString(fmt.Sprintf("    %s --> %s\n", from, to))
			}
		}
	}

	sb.WriteString("```\n")
	return sb.String()
}
