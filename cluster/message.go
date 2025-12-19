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
	Err      interface{} `json:"err"`
}
