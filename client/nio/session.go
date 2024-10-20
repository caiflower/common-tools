package nio

import (
	"errors"
	"github.com/caiflower/common-tools/pkg/logger"
	"net"
)

type ISession interface {
	Close()
}

type Session struct {
	id         int64                  // sessionId
	connection net.Conn               // 会话连接
	attribute  map[string]interface{} // 缓存一些属性，不会同步到服务端或客户端，只在本侧有效
	codec      ICodec                 // 消息编解码器
	logger     logger.ILog
}

func (s *Session) Close() {
	if err := s.connection.Close(); err != nil {
		s.logger.Error("%d close session err: %s", s.id, err.Error())
	}
}

func (s *Session) WriteMsg(msg *Msg) error {
	if s.connection == nil {
		return errors.New("not connected")
	}
	if _, err := s.connection.Write(s.codec.Encode(msg).Bytes()); err != nil {
		return err
	}
	return nil
}
