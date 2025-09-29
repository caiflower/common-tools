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

 package nio

import (
	"bytes"
	"strings"
	"time"

	"github.com/caiflower/common-tools/pkg/e"
	"github.com/caiflower/common-tools/pkg/logger"
)

type Handler struct {
	codec              ICodec
	logger             logger.ILog
	OnSessionConnected func(session *Session)
	OnSessionClosed    func(session *Session)
	OnMessageReceived  func(session *Session, message *Msg)
	OnException        func(session *Session)
}

func (h *Handler) SetCodec(codec ICodec) {
	h.codec = codec
}

func (h *Handler) readIO(socket *socket, session *Session) {
	conn := session.connection

	// 连续失败次数
	failTimes := 0

	// 缓存本次解码的消息，这个消息可能分多次解码才能完整
	lastMsg := &Msg{}

	// 数据流缓冲
	buf := bytes.NewBuffer([]byte{})

	// 每次读入的新数据
	ioData := make([]byte, 1024*4)
	for {

		if socket.closed {
			h.logger.Info("%s [%d] Connection has closed. remote=%s. i will return.", socket.addr, session.id, conn.RemoteAddr())
			session.Close()
			return
		}

		// 向缓冲区读入io流
		l, err := conn.Read(ioData)

		// 错误判断
		if err != nil {

			// 判断是否远程连接被关闭
			if err.Error() == "EOF" {
				h.logger.Info("%s [%d] Connection close. Cause of remote connection has closed. remote=%s", socket.addr, session.id, conn.RemoteAddr())
				session.Close()
				return
			}
			if strings.Contains(err.Error(), "closed by the remote host") {
				h.logger.Info("%s [%d] Connection close. Cause of remote connection has interrupted. remote=%s", socket.addr, session.id, conn.RemoteAddr())
				session.Close()
				return
			}
			if strings.Contains(err.Error(), "use of closed network") {
				h.logger.Info("%s [%d] Connection close. Cause of connection has closed. remote=%s", socket.addr, session.id, conn.RemoteAddr())
				session.Close()
				return
			}
			if strings.Contains(err.Error(), "connection reset by peer") {
				h.logger.Info("%s [%d] Connection close. Cause of connection has closed by server. remote=%s", socket.addr, session.id, conn.RemoteAddr())
				session.Close()
				return
			}

			// 错误后重试3次，3次后还错误则关闭连接
			failTimes++
			h.logger.Error("%s [%d] Read io error. times=%d. remote=%s. Cause of %s", socket.addr, session.id, failTimes, conn.RemoteAddr(), err.Error())
			if socket.handler.OnException != nil {
				go func() {
					defer e.OnError("")
					socket.handler.OnException(session)
				}()
			}
			if failTimes >= 3 {
				h.logger.Error("%s [%d] Connection close. remote=%s. Error finished.", socket.addr, session.id, conn.RemoteAddr())
				session.Close()
				return
			}
			time.Sleep(200 * time.Millisecond)
			continue
		} else {
			failTimes = 0
		}

		// 如果读到了数据
		if l > 0 {

			buf.Write(ioData[0:l])

			// 缓冲区中的数据，如果粘包了则可以解码出多条消息
			for {

				// 解码成功
				if ok := socket.handler.codec.Decode(lastMsg, buf); ok {

					// 复制一份消息数据
					copyMsg := *lastMsg

					// 回调
					if socket.handler.OnMessageReceived != nil {
						if socket.ordered {
							socket.handler.OnMessageReceived(session, &copyMsg)
						} else {
							go func(msg *Msg) {
								defer e.OnError("")
								socket.handler.OnMessageReceived(session, msg)
							}(&copyMsg)
						}
					}

					// 下次要重新初始化
					lastMsg = &Msg{}

					// 断包了，待下次再解码
				} else {
					break
				}
			}
		}
	}
}
