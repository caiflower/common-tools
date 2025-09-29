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
	"errors"
	"net"

	"github.com/caiflower/common-tools/pkg/logger"
)

type ISession interface {
	WriteMsg(msg *Msg) error
	Put(key string, v interface{})
	Get(key string) interface{}
	GetRemoteAddr() net.Addr
	Close()
}

type Session struct {
	id         int64                  // sessionId, 8个字节
	connection net.Conn               // 会话连接
	attribute  map[string]interface{} // 缓存一些属性，不会同步到服务端或客户端，只在本侧有效
	codec      ICodec                 // 消息编解码器
	logger     logger.ILog            // 日志框架
	server     *Server
	client     *Client
}

func (s *Session) Close() {
	if s.server != nil {
		s.server.removeSession(s.id)
	} else {
		s.client.Close()
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

func (s *Session) Put(key string, v interface{}) {
	s.attribute[key] = v
}

func (s *Session) Get(key string) interface{} {
	return s.attribute[key]
}

func (s *Session) GetRemoteAddr() net.Addr {
	if s.connection != nil {
		return s.connection.RemoteAddr()
	}
	return nil
}
