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
	"time"
)

type JobTracker interface {
	Name() string
	// OnStartedLeading is called when a LeaderElector client starts leading
	OnStartedLeading()
	// OnStoppedLeading is called when a LeaderElector client stops leading
	OnStoppedLeading()
	// OnReleaseMaster is called when a client release master
	OnReleaseMaster()
	// OnNewLeader is called when the client observes a leader that is
	// not the previously observed leader. This includes the first observed
	// leader when the client starts.
	OnNewLeader(leaderName string)
}

type Caller interface {
	OnStartedLeading()
	OnStoppedLeading()
	OnReleaseMaster()
	OnNewLeader(leaderName string)
	MasterCall()
	SlaverCall(leaderName string)
}

type DefaultCaller struct {
}

func (dc *DefaultCaller) OnStartedLeading()             {}
func (dc *DefaultCaller) OnStoppedLeading()             {}
func (dc *DefaultCaller) OnReleaseMaster()              {}
func (dc *DefaultCaller) OnNewLeader(leaderName string) {}
func (dc *DefaultCaller) MasterCall()                   {}
func (dc *DefaultCaller) SlaverCall(leaderName string)  {}

type DefaultJobTracker struct {
	Cluster      ICluster
	Interval     int
	leaderCtx    context.Context
	leaderCancel context.CancelFunc
	workerCtx    context.Context
	workerCancel context.CancelFunc
	callers      []Caller
}

func NewDefaultJobTracker(interval int, cluster ICluster, caller ...Caller) *DefaultJobTracker {
	if interval <= 0 {
		interval = 10
	}

	return &DefaultJobTracker{
		Interval: interval,
		Cluster:  cluster,
		callers:  caller,
	}
}

func (t *DefaultJobTracker) Name() string {
	return "DefaultJobTracker"
}

func (t *DefaultJobTracker) OnStartedLeading() {
	ctx, cancel := context.WithCancel(context.Background())
	t.leaderCtx = ctx
	t.leaderCancel = cancel
	for _, caller := range t.callers {
		go caller.OnStartedLeading()
	}

	go func(ctx context.Context) {
		ticker := time.NewTicker(time.Second * time.Duration(t.Interval))
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				for _, caller := range t.callers {
					go caller.MasterCall()
				}
			}
		}
	}(t.leaderCtx)
}

func (t *DefaultJobTracker) OnStoppedLeading() {
	if t.leaderCancel != nil {
		t.leaderCancel()
		t.leaderCancel = nil
	}
	for _, caller := range t.callers {
		go caller.OnStoppedLeading()
	}
}

func (t *DefaultJobTracker) OnReleaseMaster() {
	if t.workerCancel != nil {
		t.workerCancel()
		t.workerCancel = nil
	}
	for _, caller := range t.callers {
		go caller.OnReleaseMaster()
	}
}

func (t *DefaultJobTracker) OnNewLeader(leaderName string) {
	ctx, cancel := context.WithCancel(context.Background())
	t.workerCtx = ctx
	t.workerCancel = cancel
	for _, caller := range t.callers {
		go caller.OnNewLeader(leaderName)
	}

	go func(ctx context.Context) {
		ticker := time.NewTicker(time.Second * time.Duration(t.Interval))
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				for _, caller := range t.callers {
					go caller.SlaverCall(leaderName)
				}
			}
		}
	}(t.workerCtx)
}

func (t *DefaultJobTracker) Start() error {
	return t.Cluster.AddJobTracker(t)
}

func (t *DefaultJobTracker) Close() {
	if t.leaderCancel != nil {
		t.leaderCancel()
		t.leaderCancel = nil
	}
	if t.workerCancel != nil {
		t.workerCancel()
		t.workerCancel = nil
	}
}
