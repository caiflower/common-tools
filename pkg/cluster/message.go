package cluster

const (
	messageAskLeaderReq = iota + 100
	messageAskVoteReq
	messageBroadcastLeaderReq
	messageHeartbeatReq

	messageAskLeaderRes
	messageAskVoteRes
	messageBroadcastLeaderRes
	messageHeartbeatRes
)

type Message struct {
	NodeName       string `json:"nodeName"`
	Term           int    `json:"term"`
	LeaderNodeName string `json:"leaderNodeName,omitempty"`
	VoteNodeName   string `json:"voteNodeName,omitempty"`
	Success        bool   `json:"success"`
}
