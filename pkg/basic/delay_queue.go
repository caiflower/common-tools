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

package basic

import (
	"fmt"
	"sync"
	"time"

	"github.com/caiflower/common-tools/pkg/syncx"
)

type delayItem struct {
	priority int64
	value    interface{}
}

func (di delayItem) String() string {
	return fmt.Sprintf("%d", di.priority)
}

func (di delayItem) getDelay() time.Duration {
	return time.Duration(di.priority - time.Now().UnixNano())
}

type DelayQueue struct {
	lock  sync.Locker
	timer *time.Timer
	heap  *ObjectPriorityQueue[delayItem]
}

func NewDelayQueue() *DelayQueue {
	return &DelayQueue{
		lock:  syncx.NewSpinLock(),
		timer: time.NewTimer(30 * time.Second),
		heap:  &ObjectPriorityQueue[delayItem]{},
	}
}

func (dq *DelayQueue) Add(value interface{}, delay time.Time) {
	dq.lock.Lock()
	defer dq.lock.Unlock()

	item := delayItem{priority: delay.UnixNano(), value: value}
	dq.heap.Offer(item)
	first, err := dq.heap.Peek()
	if err == nil && first.String() == item.String() {
		dq.timer.Reset(first.getDelay())
	}
}

//func (dq *DelayQueue) Remove() {
//	dq.lock.Lock()
//	defer dq.lock.Unlock()
//
//}
//
//func (dq *DelayQueue) Update(value interface{}, delay time.Time) {
//	dq.lock.Lock()
//	defer dq.lock.Unlock()
//}

func (dq *DelayQueue) Take() (value interface{}) {
	return dq.TakeWithStop(nil)
}

// TakeWithStop 从延迟队列中取出元素，支持通过 stopChan 中断阻塞等待。
// 当 stopChan 被关闭时，返回 nil。
func (dq *DelayQueue) TakeWithStop(stopChan <-chan struct{}) (value interface{}) {
	for {
		var ok bool
		func() {
			// 加锁
			dq.lock.Lock()
			defer dq.lock.Unlock()

			// 延期时间
			var delay time.Duration

			// 获取队列中第一个元素，并未真正取出。
			// 如果没有取到，说明队列是空的，等待30秒
			// 取到了，计算延期时间，时间小于0代表已到期。大于0代表未到期，未到期就按延期时间进行等待
			if first, err := dq.heap.Peek(); err != nil {
				delay = time.Second * 30
			} else {
				delay = first.getDelay()
				if delay <= time.Microsecond {
					poll, _ := dq.heap.Poll()
					value, ok = poll.value, true
				}
			}

			// 重置延期时间
			dq.timer.Reset(delay)
		}()

		// 取到了则返回，取不到则等待
		if ok {
			return
		}

		select {
		case <-dq.timer.C:
			// 定时器触发，继续循环尝试取出
		case <-stopChan:
			// 收到停止信号，立即返回
			return nil
		}
	}
}

func (dq *DelayQueue) Size() int {
	return dq.heap.Size()
}
