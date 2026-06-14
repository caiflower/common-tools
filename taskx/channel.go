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

import "sync"

// dagChannel DAG 通道，管理节点的依赖状态和数据传递
type dagChannel struct {
	mu sync.Mutex

	nodeKey string

	// 控制依赖前驱及其状态
	controlPredecessors map[string]dependencyState

	// 数据依赖前驱及其就绪状态
	dataPredecessors map[string]bool

	// 前驱节点传递的值
	values map[string]any

	// 当前节点是否被跳过
	skipped bool

	// 触发模式
	triggerMode NodeTriggerMode
}

// newDAGChannel 创建 DAG 通道
func newDAGChannel(nodeKey string, controlPreds []string, dataPreds []string, triggerMode NodeTriggerMode) *dagChannel {
	ch := &dagChannel{
		nodeKey:             nodeKey,
		triggerMode:         triggerMode,
		controlPredecessors: make(map[string]dependencyState),
		dataPredecessors:    make(map[string]bool),
		values:              make(map[string]any),
		skipped:             false,
	}

	for _, pred := range controlPreds {
		ch.controlPredecessors[pred] = depWaiting
	}

	for _, pred := range dataPreds {
		ch.dataPredecessors[pred] = false
	}

	return ch
}

// reportValues 前驱节点完成后报告输出值
func (ch *dagChannel) reportValues(values map[string]any) {
	ch.mu.Lock()
	defer ch.mu.Unlock()

	if ch.skipped {
		return
	}

	for key, value := range values {
		if _, ok := ch.dataPredecessors[key]; ok {
			ch.dataPredecessors[key] = true
			ch.values[key] = value
		}
	}
}

// reportDependencies 前驱节点完成后报告控制依赖就绪
func (ch *dagChannel) reportDependencies(deps []string) {
	ch.mu.Lock()
	defer ch.mu.Unlock()

	if ch.skipped {
		return
	}

	for _, dep := range deps {
		if _, ok := ch.controlPredecessors[dep]; ok {
			ch.controlPredecessors[dep] = depReady
		}
	}
}

// reportSkip 前驱节点跳过时报告
// 返回当前节点是否也应该被跳过（当所有控制前驱都被跳过时）
func (ch *dagChannel) reportSkip(keys []string) bool {
	ch.mu.Lock()
	defer ch.mu.Unlock()

	if ch.skipped {
		return true
	}

	for _, key := range keys {
		if _, ok := ch.controlPredecessors[key]; ok {
			ch.controlPredecessors[key] = depSkipped
		}
		if _, ok := ch.dataPredecessors[key]; ok {
			ch.dataPredecessors[key] = true // 数据依赖也标记为就绪（但值为 nil）
			ch.values[key] = nil
		}
	}

	// 检查是否所有控制前驱都被跳过了
	allSkipped := true
	for _, state := range ch.controlPredecessors {
		if state != depSkipped {
			allSkipped = false
			break
		}
	}
	ch.skipped = allSkipped

	return allSkipped
}

// isReady 检查当前节点是否满足执行条件
// 根据 triggerMode 决定是所有前驱完成还是任一前驱完成
func (ch *dagChannel) isReady() bool {
	ch.mu.Lock()
	defer ch.mu.Unlock()

	if ch.skipped {
		return false
	}

	if ch.triggerMode == AnyPredecessor {
		// 任一前驱完成即可
		for _, state := range ch.controlPredecessors {
			if state == depReady {
				return true
			}
		}
		return false
	}

	// 所有控制前驱必须完成或跳过
	for _, state := range ch.controlPredecessors {
		if state == depWaiting {
			return false
		}
	}

	return true
}

// isDataReady 检查数据依赖是否就绪
func (ch *dagChannel) isDataReady() bool {
	ch.mu.Lock()
	defer ch.mu.Unlock()

	for _, ready := range ch.dataPredecessors {
		if !ready {
			return false
		}
	}
	return true
}

// get 获取合并后的输入数据
// 返回值、是否就绪、错误
func (ch *dagChannel) get() (any, bool, error) {
	ch.mu.Lock()
	defer ch.mu.Unlock()

	if ch.skipped {
		return nil, false, nil
	}

	if !ch.isReadyLocked() {
		return nil, false, nil
	}

	if !ch.isDataReadyLocked() {
		return nil, false, nil
	}

	// 合并所有前驱的值
	if len(ch.values) == 0 {
		return nil, true, nil
	}

	// 如果只有一个前驱的值，直接返回
	if len(ch.values) == 1 {
		for _, v := range ch.values {
			return v, true, nil
		}
	}

	// 多个前驱时，返回值映射
	result := make(map[string]any)
	for k, v := range ch.values {
		result[k] = v
	}

	return result, true, nil
}

// isReadyLocked 检查当前节点是否满足执行条件（调用方已持锁）
func (ch *dagChannel) isReadyLocked() bool {
	if ch.skipped {
		return false
	}

	if ch.triggerMode == AnyPredecessor {
		for _, state := range ch.controlPredecessors {
			if state == depReady {
				return true
			}
		}
		return false
	}

	for _, state := range ch.controlPredecessors {
		if state == depWaiting {
			return false
		}
	}

	return true
}

// isDataReadyLocked 检查数据依赖是否就绪（调用方已持锁）
func (ch *dagChannel) isDataReadyLocked() bool {
	for _, ready := range ch.dataPredecessors {
		if !ready {
			return false
		}
	}
	return true
}

// reset 重置通道状态（用于重新执行）
func (ch *dagChannel) reset() {
	ch.mu.Lock()
	defer ch.mu.Unlock()

	for key := range ch.controlPredecessors {
		ch.controlPredecessors[key] = depWaiting
	}
	for key := range ch.dataPredecessors {
		ch.dataPredecessors[key] = false
	}
	ch.values = make(map[string]any)
	ch.skipped = false
}

// isSkipped 返回节点是否被跳过
func (ch *dagChannel) isSkipped() bool {
	ch.mu.Lock()
	defer ch.mu.Unlock()

	return ch.skipped
}

// getControlPredecessorStates 获取控制前驱状态
func (ch *dagChannel) getControlPredecessorStates() map[string]dependencyState {
	ch.mu.Lock()
	defer ch.mu.Unlock()

	result := make(map[string]dependencyState, len(ch.controlPredecessors))
	for k, v := range ch.controlPredecessors {
		result[k] = v
	}
	return result
}
