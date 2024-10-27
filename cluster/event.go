package cluster

const (
	eventNameStartUp        = "StartUp"
	eventNameSignMaster     = "SignMaster"
	eventNameSignFollower   = "SignFollower"
	eventNameStopMaster     = "StopMaster"
	eventNameReleaseMaster  = "ReleaseMaster"
	eventNameElectionStart  = "ElectionStart"
	eventNameElectionFinish = "ElectionFinish"
	eventNameClose          = "Close"
)

type event struct {
	name        string
	clusterStat stat
	nodeName    string
	leaderName  string
}
