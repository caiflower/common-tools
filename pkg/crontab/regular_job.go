package crontab

import (
	"context"
	"time"

	"github.com/caiflower/common-tools/global"
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
	delay    time.Duration
	running  bool
	ctx      context.Context
	cancel   context.CancelFunc
	fn       func()
	name     string
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

	global.DefaultResourceManger.Add(job)
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

func (j *regularJob) Run() {
	go func() {
		j.running = true
		defer func() {
			j.running = false
		}()

		logger.Info("%s regular job start", j.name)

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
