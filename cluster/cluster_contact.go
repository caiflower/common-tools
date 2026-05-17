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
	"errors"

	"github.com/caiflower/common-tools/pkg/cache"
	"github.com/caiflower/common-tools/pkg/nio"
	"github.com/caiflower/common-tools/pkg/tools"
)

// getClientHandler client contact
func (c *Cluster) getClientHandler(nodeName string) *nio.Handler {
	handler := &nio.Handler{}

	handler.OnSessionConnected = func(session *nio.Session) {
		c.logger.Trace("[cluster] %s OnSessionConnected, now is alive.", nodeName)
		c.aliveNodes.Store(nodeName, c.GetNodeByName(nodeName))
	}

	handler.OnSessionClosed = func(session *nio.Session) {
		c.logger.Trace("[cluster] %s OnSessionClosed, now is lost.", nodeName)
		c.aliveNodes.Delete(nodeName)
	}

	handler.OnMessageReceived = func(session *nio.Session, message *nio.Msg) {
		// 其他协议，不受任期影响
		if message.Flag() == messageRemoteCallRes {
			rcMsg := new(remoteCallMessage)
			err := message.Unmarshal(rcMsg)
			if err != nil {
				c.logger.Warn("[cluster] unmarshal remoteCallMessage error: %s", err.Error())
			}

			c.logger.Trace("[cluster] remote [%s] func return", session.GetRemoteAddr())
			if f, ok := cache.LocalCache.Get(remoteCall + rcMsg.UUID); ok && f != nil {
				var _err error
				if rcMsg.Err != nil {
					bytes, bytesErr := tools.ToByte(rcMsg.Err)
					if bytesErr != nil {
						c.logger.Warn("[cluster] convert remoteCallMessage param 'err' to bytes failed. Error: %s", bytesErr.Error())
					}
					_err = errors.New(string(bytes))
				}

				f.(*FuncSpec).setResult(rcMsg.Result, _err)
				// remote return, then delete cache
				cache.LocalCache.Delete(remoteCall + rcMsg.UUID)
			}
			return
		}

		// 集群基本协议
		nodeMsg := new(Message)
		if err := message.Unmarshal(nodeMsg); err != nil {
			c.logger.Warn("[cluster] unmarshal msg error: %s", err.Error())
		}
		if nodeMsg.Term < c.GetMyTerm() {
			// 落后的消息直接忽略
			return
		}

		switch message.Flag() {
		case messageAskLeaderRes:
			c.logger.Trace("[cluster] messageAskLeaderRes nodeName=%s, get leaderNode=%s", nodeMsg.NodeName, nodeMsg.LeaderNodeName)
			if ch, ok := c.msgChan.Load().(chan *Message); ok && ch != nil {
				ch <- nodeMsg
			}
		case messageAskVoteRes:
			c.logger.Trace("[cluster] messageAskVoteRes term=%d nodeName=%s, vote for '%s'", nodeMsg.Term, nodeMsg.NodeName, nodeMsg.VoteNodeName)
			if ch, ok := c.msgChan.Load().(chan *Message); ok && ch != nil {
				ch <- nodeMsg
			}
		case messageBroadcastLeaderRes:
			if nodeMsg.Success {
				c.logger.Trace("[cluster] messageBroadcastLeaderRes nodeName=%s, leaderName=%s", nodeMsg.NodeName, nodeMsg.LeaderNodeName)
			}
			if ch, ok := c.msgChan.Load().(chan *Message); ok && ch != nil {
				ch <- nodeMsg
			}
		case messageHeartbeatRes:
			if senderNode := c.GetNodeByName(nodeMsg.NodeName); senderNode != nil {
				if nodeMsg.Success {
					senderNode.updateHeartbeat()
				} else if nodeMsg.LeaderNodeName != "" && nodeMsg.LeaderNodeName != c.GetLeaderName() {
					senderNode.lastHeartbeatOk = true
					senderNode.heartbeatFailures = 0
					c.logger.Debug("[cluster] heartbeat from %s, leader changed from %s to %s",
						nodeMsg.NodeName, c.GetLeaderName(), nodeMsg.LeaderNodeName)
				} else {
					senderNode.updateHeartbeatFailed()
				}
			}
			if ch, ok := c.msgChan.Load().(chan *Message); ok && ch != nil {
				ch <- nodeMsg
			}
		default:
			c.logger.Warn("[cluster] unknown message type: %s", message.Flag())
		}
	}

	return handler
}

// getServerHandler server contact
func (c *Cluster) getServerHandler() *nio.Handler {
	handler := &nio.Handler{}

	handler.OnMessageReceived = func(session *nio.Session, message *nio.Msg) {
		// 其他协议，不受任期影响
		if message.Flag() == messageRemoteCallReq {
			rcMsg := new(remoteCallMessage)
			err := message.Unmarshal(rcMsg)
			if err != nil {
				c.logger.Warn("[cluster] unmarshal remoteCallMessage error: %s", err.Error())
			}
			f := &FuncSpec{
				traceId:  rcMsg.TraceID,
				uuid:     rcMsg.UUID,
				funcName: rcMsg.FuncName,
				param:    rcMsg.Param,
				sync:     rcMsg.Sync,
			}
			msg := new(remoteCallMessage)
			msg.TraceID = f.traceId
			msg.UUID = f.uuid
			msg.FuncName = f.funcName
			msg.Param = f.param
			msg.Sync = f.sync

			c.logger.Trace("[cluster] '%s' exec remote func", c.GetMyName())
			c.callLocalFunc(f)
			msg.Result = f.result
			msg.Err = f.err

			if err = session.WriteMsg(nio.NewMsg(messageRemoteCallRes, msg)); err != nil {
				c.logger.Error("[cluster] [remote call] %s failed. %s Cause of %s.", f.uuid, f.funcName, err)
			}
			return
		}

		// 集群基本协议
		nodeMsg := new(Message)
		if err := message.Unmarshal(nodeMsg); err != nil {
			c.logger.Warn("[cluster] unmarshal msg error: %s", err.Error())
		}

		switch message.Flag() {
		case messageAskLeaderReq:
			// 如果当前集群就绪且当前周期大于等于消息的周期
			msg := new(Message)
			msg.NodeName = c.GetMyName()
			msg.Term = nodeMsg.Term
			msg.Success = false

			if c.IsReady() && nodeMsg.Term <= c.GetMyTerm() {
				msg.Term = c.GetMyTerm()
				msg.LeaderNodeName = c.GetLeaderName()
				msg.Success = true
			}
			if _, ok := c.aliveNodes.Load(nodeMsg.NodeName); !ok {
				// 重新建立集群连接
				go c.reconnect()
			}

			if err := session.WriteMsg(nio.NewMsg(messageAskLeaderRes, msg)); err != nil {
				c.logger.Warn("[cluster] write msg error: %s", err.Error())
			}
		case messageAskVoteReq:
			msg := new(Message)
			msg.NodeName = c.GetMyName()
			msg.Term = nodeMsg.Term
			msg.Success = true
			msg.VoteNodeName = c.getVoteNodeName(nodeMsg.Term, nodeMsg.NodeName)

			if !c.IsReady() || nodeMsg.Term > c.GetMyTerm() {
				c.releaseLeader()
				c.logger.Info("[cluster] cluster status %v,accept term %d ask vote req, releaseLeader finished.", c.IsReady(), nodeMsg.Term)
			} else {
				msg.Success = false
			}
			c.logger.Info("[cluster] accept term %d ask vote req from '%s', vote for '%s'.", nodeMsg.Term, nodeMsg.NodeName, msg.VoteNodeName)

			if err := session.WriteMsg(nio.NewMsg(messageAskVoteRes, msg)); err != nil {
				c.logger.Warn("[cluster] write msg error: %s", err.Error())
			}
		case messageBroadcastLeaderReq:
			msg := new(Message)
			msg.NodeName = c.GetMyName()
			msg.Term = nodeMsg.Term
			msg.Success = false

			if c.GetMyTerm() > nodeMsg.Term {
				msg.Term = c.GetMyTerm()
			} else {
				// 优先从 aliveNodes 查，找不到则从 allNode 查（连接可能还未建立）
				var leaderNode *Node
				if v, ok := c.aliveNodes.Load(nodeMsg.LeaderNodeName); ok {
					leaderNode = v.(*Node)
				} else if nodeMsg.LeaderNodeName == c.GetMyName() {
					leaderNode = c.curNode
				} else if v, ok := c.allNode.Load(nodeMsg.LeaderNodeName); ok {
					leaderNode = v.(*Node)
					// 触发后台重连，让后续心跳能正常通信
					go c.reconnect()
				}
				if leaderNode != nil {
					msg.Success = true
					c.signLeader(leaderNode, nodeMsg.Term)
					msg.LeaderNodeName = nodeMsg.LeaderNodeName
				}
			}

			c.logger.Trace("[cluster] accept broadcast from '%s', success '%v'.", nodeMsg.NodeName, msg.Success)
			if err := session.WriteMsg(nio.NewMsg(messageBroadcastLeaderRes, msg)); err != nil {
				c.logger.Warn("[cluster] write msg error: %s", err.Error())
			}
		case messageHeartbeatReq:
			msg := new(Message)
			msg.NodeName = c.GetMyName()
			msg.Term = nodeMsg.Term
			msg.LeaderNodeName = c.GetLeaderName()

			if c.GetMyTerm() > nodeMsg.Term {
				msg.Term = c.GetMyTerm()
				msg.Success = false
			} else if c.GetLeaderName() != nodeMsg.NodeName {
				msg.Success = false
				c.logger.Debug("[cluster] heartbeat from %s, but leader is now '%s'", nodeMsg.NodeName, c.GetLeaderName())
			} else {
				msg.Success = true
				c.curNode.updateHeartbeat()
			}

			if err := session.WriteMsg(nio.NewMsg(messageHeartbeatRes, msg)); err != nil {
				c.logger.Warn("[cluster] write msg error: %s", err.Error())
			}
		default:
			c.logger.Warn("[cluster] unknown message type: %s", message.Flag())
		}
	}

	return handler
}
