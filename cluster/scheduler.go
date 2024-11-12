package cluster

import (
	"context"
	"time"

	"github.com/caiflower/common-tools/global"
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
	MasterCall()
	SlaverCall(leaderName string)
}

type DefaultJobTracker struct {
	Cluster      ICluster
	Interval     int
	leaderCtx    context.Context
	leaderCancel context.CancelFunc
	workerCtx    context.Context
	workerCancel context.CancelFunc
	callers      []Caller
}

func NewDefaultJobTracker(interval int, config *Config, caller ...Caller) (*DefaultJobTracker, error) {
	cluster, err := NewCluster(config)
	if err != nil {
		return nil, err
	}

	if interval <= 0 {
		interval = 10
	}

	return &DefaultJobTracker{
		Interval: interval,
		Cluster:  cluster,
		callers:  caller,
	}, nil
}

func (t *DefaultJobTracker) Name() string {
	return "DefaultJobTracker"
}

func (t *DefaultJobTracker) OnStartedLeading() {
	ctx, cancel := context.WithCancel(context.Background())
	t.leaderCtx = ctx
	t.leaderCancel = cancel

	go func(ctx context.Context) {
		ticker := time.NewTicker(time.Second * time.Duration(t.Interval))
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				for _, caller := range t.callers {
					go caller.MasterCall()
				}
			default:
			}
		}
	}(t.leaderCtx)
}

func (t *DefaultJobTracker) OnStoppedLeading() {
	t.leaderCancel()
}

func (t *DefaultJobTracker) OnReleaseMaster() {
	t.workerCancel()
}

func (t *DefaultJobTracker) OnNewLeader(leaderName string) {
	ctx, cancel := context.WithCancel(context.Background())
	t.workerCtx = ctx
	t.workerCancel = cancel

	go func(ctx context.Context) {
		ticker := time.NewTicker(time.Second * time.Duration(t.Interval))
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				for _, caller := range t.callers {
					go caller.SlaverCall(leaderName)
				}
			default:
			}
		}
	}(t.workerCtx)
}

func (t *DefaultJobTracker) Start() {
	t.Cluster.AddJobTracker(t)
	t.Cluster.StartUp()
	global.DefaultResourceManger.Add(t)
}

func (t *DefaultJobTracker) Close() {
	t.leaderCancel()
	t.workerCancel()
}