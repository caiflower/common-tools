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

package cluster

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/caiflower/common-tools/pkg/logger"
)

type JobTracker interface {
	Name() string
	// OnStartedLeading is called when a LeaderElector client starts leading
	OnStartedLeading()
	// OnStoppedLeading is called when a LeaderElector client stops leading
	OnStoppedLeading()
	// OnStartedFollowing is called when a LeaderElector client starts following
	// leaderName is the name of leader
	OnStartedFollowing(leaderName string)
	// OnStartedFollowing is called when a LeaderElector client stops following
	OnStoppedFollowing()
}

type Caller interface {
	OnStartedLeading()
	OnStoppedLeading()
	OnStartedFollowing(leaderName string)
	OnStoppedFollowing()
	MasterCall()
	SlaverCall(leaderName string)
}

type DefaultCaller struct {
}

func (dc *DefaultCaller) OnStartedLeading()                    {}
func (dc *DefaultCaller) OnStoppedLeading()                    {}
func (dc *DefaultCaller) OnStoppedFollowing()                  {}
func (dc *DefaultCaller) OnStartedFollowing(leaderName string) {}
func (dc *DefaultCaller) MasterCall()                          {}
func (dc *DefaultCaller) SlaverCall(leaderName string)         {}

// JobTrackerConfig configures DefaultJobTracker.
type JobTrackerConfig struct {
	Interval int
	// Blocking runs MasterCall and SlaverCall synchronously in the scheduler
	// goroutine. A slow call delays the next tick. The default is false, which
	// keeps the existing asynchronous behavior.
	Blocking bool
}

const (
	statusStopped uint32 = iota
	statusRunning
	statusClose
)

type DefaultJobTracker struct {
	Interval     int
	Blocking     bool
	leaderCtx    context.Context
	leaderCancel context.CancelFunc
	workerCtx    context.Context
	workerCancel context.CancelFunc
	callers      []Caller
	status       uint32
}

func NewDefaultJobTracker(interval int, caller ...Caller) *DefaultJobTracker {
	return NewJobTracker(JobTrackerConfig{Interval: interval}, caller...)
}

// NewJobTracker creates a tracker with explicit scheduling options.
func NewJobTracker(config JobTrackerConfig, caller ...Caller) *DefaultJobTracker {
	if config.Interval <= 0 {
		config.Interval = 10
	}
	return &DefaultJobTracker{
		Interval: config.Interval,
		Blocking: config.Blocking,
		callers:  caller,
	}
}

func (t *DefaultJobTracker) Name() string {
	return "DefaultJobTracker"
}

func (t *DefaultJobTracker) call(fn func()) {
	if t.Blocking {
		fn()
		return
	}
	go fn()
}

func (t *DefaultJobTracker) runTicker(ctx context.Context, call func(Caller)) {
	interval := time.Second * time.Duration(t.Interval)
	if interval <= 0 {
		interval = 10 * time.Second
	}

	timer := time.NewTimer(interval)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			for _, caller := range t.callers {
				call(caller)
			}
			timer.Reset(interval)
		}
	}
}

func (t *DefaultJobTracker) OnStartedLeading() {
	if !atomic.CompareAndSwapUint32(&t.status, statusStopped, statusRunning) {
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.leaderCtx = ctx
	t.leaderCancel = cancel
	for _, caller := range t.callers {
		go caller.OnStartedLeading()
	}

	go t.runTicker(t.leaderCtx, func(caller Caller) {
		t.call(caller.MasterCall)
	})
}

func (t *DefaultJobTracker) OnStoppedLeading() {
	if !atomic.CompareAndSwapUint32(&t.status, statusRunning, statusStopped) {
		return
	}

	if t.leaderCancel != nil {
		t.leaderCancel()
	}
	for _, caller := range t.callers {
		go caller.OnStoppedLeading()
	}
}

func (t *DefaultJobTracker) OnStoppedFollowing() {
	if !atomic.CompareAndSwapUint32(&t.status, statusRunning, statusStopped) {
		return
	}

	if t.workerCancel != nil {
		t.workerCancel()
	}
	for _, caller := range t.callers {
		go caller.OnStoppedFollowing()
	}
}

func (t *DefaultJobTracker) OnStartedFollowing(leaderName string) {
	if !atomic.CompareAndSwapUint32(&t.status, statusStopped, statusRunning) {
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.workerCtx = ctx
	t.workerCancel = cancel
	for _, caller := range t.callers {
		go caller.OnStartedFollowing(leaderName)
	}

	go t.runTicker(t.workerCtx, func(caller Caller) {
		t.call(func() {
			caller.SlaverCall(leaderName)
		})
	})
}

func (t *DefaultJobTracker) Close() {
	atomic.CompareAndSwapUint32(&t.status, statusRunning, statusClose)
	atomic.CompareAndSwapUint32(&t.status, statusStopped, statusClose)

	if t.leaderCancel != nil {
		t.leaderCancel()
	}
	if t.workerCancel != nil {
		t.workerCancel()
	}

	logger.Info("[DefaultJobTracker] close success.")
}
