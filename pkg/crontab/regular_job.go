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

 package crontab

import (
	"context"
	"fmt"
	"runtime/debug"
	"time"

	"github.com/caiflower/common-tools/pkg/logger"
)

var DefaultInterval = time.Second * 5

type RegularJob interface {
	Run()
	Stop()
}

type Opt func(job *regularJob)

type regularJob struct {
	interval time.Duration
	// ignorePanic run again when panic
	ignorePanic bool
	// immediately do job immediately
	immediately bool
	delay       time.Duration
	running     bool
	ctx         context.Context
	cancel      context.CancelFunc
	fn          func()
	name        string
}

func NewRegularJob(name string, fn func(), opts ...Opt) RegularJob {
	ctx, cancel := context.WithCancel(context.Background())
	job := &regularJob{
		name:   name,
		ctx:    ctx,
		cancel: cancel,
		fn:     fn,
	}

	for _, opt := range opts {
		opt(job)
	}

	if job.interval == 0 {
		job.interval = DefaultInterval
	}

	return job
}

// WithInterval the execution interval time of job
func WithInterval(interval time.Duration) Opt {
	return func(job *regularJob) {
		job.interval = interval
	}
}

//// WithDelay the delay time of begin job
//func WithDelay(delay time.Duration) Opt {
//	return func(job *regularJob) {
//		job.delay = delay
//	}
//}

// WithIgnorePanic run again when panic
func WithIgnorePanic() Opt {
	return func(job *regularJob) {
		job.ignorePanic = true
	}
}

func WithImmediately() Opt {
	return func(job *regularJob) {
		job.immediately = true
	}
}

func (j *regularJob) Run() {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("%s [ERROR] - Got a runtime error %s. %s\n%s", time.Now().Format("2006-01-02 15:04:05"), "exec regular job", r, string(debug.Stack()))
				if j.ignorePanic {
					j.Run()
				}
			}
		}()

		j.running = true
		defer func() {
			j.running = false
		}()

		logger.Info("%s regular job start", j.name)

		if j.immediately {
			go j.fn()
		}

		ticker := time.NewTicker(j.interval)
		defer ticker.Stop()
	f:
		for {
			select {
			case <-j.ctx.Done():
				break f
			case <-ticker.C:
				j.fn()
			}
		}

		logger.Info("%s regular job stop", j.name)
	}()
}

func (j *regularJob) Stop() {
	if j.running {
		j.cancel()
	}
}

func (j *regularJob) Close() {
	j.Stop()
}
