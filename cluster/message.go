package cluster

const (
	messageAskLeaderReq = iota + 100
	messageAskLeaderRes
	messageAskVoteReq
	messageAskVoteRes
	messageBroadcastLeaderReq
	messageBroadcastLeaderRes
	messageHeartbeatReq
	messageHeartbeatRes
	messageRemoteCallReq
	messageRemoteCallRes
)

type Message struct {
	NodeName       string `json:"nodeName"`
	Term           int    `json:"term"`
	LeaderNodeName string `json:"leaderNodeName,omitempty"`
	VoteNodeName   string `json:"voteNodeName,omitempty"`
	Success        bool   `json:"success"`
}

type remoteCallMessage struct {
	TraceID  string      `json:"traceId"`
	UUID     string      `json:"uuid"`
	FuncName string      `json:"funcName"`
	Param    interface{} `json:"param"`
	Sync     bool        `json:"sync"`
	Result   interface{} `json:"result"`
	Err      error       `json:"err"`
}
