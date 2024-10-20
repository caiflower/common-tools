package nio

import (
	"bytes"
	"github.com/caiflower/common-tools/pkg/e"
	golocalv1 "github.com/caiflower/common-tools/pkg/golocal/v1"
	"github.com/caiflower/common-tools/pkg/logger"
	"strconv"
	"strings"
	"time"
)

type Handler struct {
	codec              ICodec
	OnSessionConnected func(session *Session)
	OnSessionClosed    func(session *Session)
	OnMessageReceived  func(session *Session, message *Msg)
	OnException        func(session *Session)
}

func (h *Handler) readIO(socket *socket, session *Session) {
	golocalv1.PutTraceID("nio.handleSession" + strconv.Itoa(int(session.id)))
	defer golocalv1.Clean()

	conn := session.connection

	// 连续失败次数
	fail_times := 0

	// 缓存本次解码的消息，这个消息可能分多次解码才能完整
	last_msg := &Msg{}

	// 数据流缓冲
	buf := bytes.NewBuffer([]byte{})

	// 每次读入的新数据
	ioData := make([]byte, 1024*4)
	for {

		if socket.closed {
			logger.Error("%s [%d] Connection has closed. remote=%s. i will return.", socket.addr, session.id, conn.RemoteAddr())
			return
		}

		// 向缓冲区读入io流
		len, err := conn.Read(ioData)

		// 错误判断
		if err != nil {

			// 判断是否远程连接被关闭
			if err.Error() == "EOF" {
				logger.Info("%s [%d] Connection close. Cause of remote connection has closed. remote=%s", socket.addr, session.id, conn.RemoteAddr())
				session.Close()
				return
			}
			if strings.Contains(err.Error(), "closed by the remote host") {
				logger.Info("%s [%d] Connection close. Cause of remote connection has interrupted. remote=%s", socket.addr, session.id, conn.RemoteAddr())
				session.Close()
				return
			}
			if strings.Contains(err.Error(), "use of closed network") {
				logger.Info("%s [%d] Connection close. Cause of connection has closed. remote=%s", socket.addr, session.id, conn.RemoteAddr())
				session.Close()
				return
			}

			// 错误后重试3次，3次后还错误则关闭连接
			fail_times++
			logger.Error("%s [%d] Read io error. times=%d. remote=%s. Cause of %s", socket.addr, session.id, fail_times, conn.RemoteAddr(), err.Error())
			if socket.handler.OnException != nil {
				go func() {
					defer e.OnError("")
					socket.handler.OnException(session)
				}()
			}
			if fail_times >= 3 {
				logger.Error("%s [%d] Connection close. remote=%s. Error finished.", socket.addr, session.id, conn.RemoteAddr())
				session.Close()
				return
			}
			time.Sleep(200 * time.Millisecond)
			continue
		} else {
			fail_times = 0
		}

		// 如果读到了数据
		if len > 0 {

			buf.Write(ioData[0:len])

			// 缓冲区中的数据，如果粘包了则可以解码出多条消息
			for {

				// 解码成功
				if ok := socket.handler.codec.Decode(last_msg, buf); ok {

					// 复制一份消息数据
					copyMsg := *last_msg

					// 回调
					if socket.handler.OnMessageReceived != nil {
						if socket.ordered {
							socket.handler.OnMessageReceived(session, &copyMsg)
						} else {
							go func(cmsg *Msg) {
								defer e.OnError("")
								socket.handler.OnMessageReceived(session, cmsg)
							}(&copyMsg)
						}
					}

					// 下次要重新初始化
					last_msg = &Msg{}

					// 断包了，待下次再解码
				} else {
					break
				}
			}
		}
	}
}
