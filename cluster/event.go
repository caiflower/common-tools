package cluster

const (
	eventNameStartUp        = "StartUp"
	eventNameSignMaster     = "SignMaster"
	eventNameSignFollower   = "SignFollower"
	eventNameReleaseMaster  = "ReleaseMaster"
	eventNameElectionStart  = "ElectionStart"
	eventNameElectionFinish = "ElectionFinish"
	eventNameClose          = "Close"
)

type event struct {
	name        string
	clusterStat stat
}
