package cluster

type JobTracker interface {
	Name() string
	// OnStartedLeading is called when a LeaderElector client starts leading
	OnStartedLeading()
	// OnStoppedLeading is called when a LeaderElector client stops leading
	OnStoppedLeading()
	// OnReleaseMaster is called when a LeaderElector client release master
	OnReleaseMaster()
	// OnNewLeader is called when the client observes a leader that is
	// not the previously observed leader. This includes the first observed
	// leader when the client starts.
	OnNewLeader(leaderName string)
}
