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
	"encoding/binary"
	golocalv1 "github.com/caiflower/common-tools/pkg/golocal/v1"
	"net"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/caiflower/common-tools/pkg/e"
	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/caiflower/common-tools/pkg/syncx"
)

type socket struct {
	addr    string   //对于服务端:代表监听的地址，对于客户端:代表要连接的地址
	handler *Handler //业务处理器，由使用方自行实现，必须设置
	closed  bool     //服务端或客户端是否已关闭
	ordered bool     //true消息处理顺序请求，false并发请求
	timeout int
}

type Config struct {
	Addr    string `yaml:"server.addr"`
	Timeout int    `yaml:"client.timeout"`
}

type IServer interface {
	Open() error
	Close()
	GetSessionCount() int
}

type Server struct {
	socket
	buildSessionID int64
	lock           sync.Locker
	sessions       map[int64]*Session
	sessionCnt     int
	acceptor       net.Listener
	logger         logger.ILog
}

func NewServer(config *Config, handler *Handler) *Server {
	return NewServerWithAllArgs(config, syncx.NewSpinLock(), logger.DefaultLogger(), handler)
}

func NewServerWithAllArgs(config *Config, locker sync.Locker, logger logger.ILog, handler *Handler) *Server {
	if logger == nil {
		panic("[nio] logger must not be nil. ")
	}
	if locker == nil {
		panic("[nio] locker must not be nil. ")
	}
	if handler == nil {
		panic("[nio] handler must not be nil. ")
	}
	if config == nil {
		config = &Config{Addr: "127.0.0.1:8801"}
	}
	if handler.codec == nil {
		handler.codec = GetZipCodec(logger)
	}
	handler.logger = logger
	return &Server{
		socket: socket{
			addr:    config.Addr,
			handler: handler,
			closed:  true,
		},
		lock:       locker,
		sessions:   make(map[int64]*Session),
		sessionCnt: 0,
		logger:     logger,
	}
}

func (s *Server) Open() error {
	s.logger.Info("[server] Open socket %s acceptor and listening...", s.addr)

	listen, err := net.Listen("tcp", s.socket.addr)
	if err != nil {
		s.logger.Error("[server] Open socket %s err: %s .", s.addr, err.Error())
		return err
	}

	s.acceptor = listen
	s.closed = false
	s.logger.Info("[server] Open socket %s success. ", s.addr)

	go s.handlerConnection()
	return nil
}

func (s *Server) Close() {
	s.logger.Info("[server] Close socket %s acceptor. ", s.addr)

	for i := 0; i < 3; i++ {
		if err := s.acceptor.Close(); err != nil {
			s.logger.Warn("[server] Close socket %s err: %s .", s.addr, err.Error())
		} else {
			s.closed = true
			break
		}
	}

	if s.closed {
		s.logger.Info("[server] Close socket %s success.", s.addr)
	} else {
		s.logger.Error("[server] Close socket %s failed. Force stop.", s.addr)
		s.closed = true
	}
}

func (s *Server) handlerConnection() {
	golocalv1.PutTraceID("nio.handlerConnection")
	defer golocalv1.Clean()

	for {
		if s.closed {
			s.logger.Info("[server] socket is closed and stop handlerConnection.")
			return
		}

		connect, err := s.acceptor.Accept()
		if err != nil {
			if strings.Contains(err.Error(), "use of closed network") {
				s.logger.Info("[server] socket is closed and stop handlerConnection.")
			} else {
				s.logger.Error("[server] socket is closed. accept client err: %s", err.Error())
			}
			return
		}

		sessionId := atomic.AddInt64(&s.buildSessionID, 1)
		session := &Session{
			id:         sessionId,
			connection: connect,
			attribute:  make(map[string]interface{}),
			codec:      GetZipCodec(s.logger),
			logger:     s.logger,
			server:     s,
		}
		if err = s.syncSession(session); err != nil {
			connect.Close()
			continue
		}

		s.addSession(session)

		if s.handler.OnSessionConnected != nil {
			func() {
				defer e.OnError("")
				s.handler.OnSessionConnected(session)
			}()
		}

		// 会话处理
		go s.handler.readIO(&s.socket, session)
	}
}

func (s *Server) syncSession(session *Session) error {
	buf := new(bytes.Buffer)
	if err := binary.Write(buf, binary.BigEndian, session.id); err != nil {
		s.logger.Error("[server] sync session err: %s", err.Error())
		return err
	}
	if _, ioErr := session.connection.Write(buf.Bytes()); ioErr != nil {
		s.logger.Error("[server] sync session failed. write err: %s", ioErr.Error())
		return ioErr
	}
	return nil
}

func (s *Server) addSession(session *Session) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.sessions[session.id] = session
	s.sessionCnt++
}

func (s *Server) removeSession(sessionID int64) {
	s.lock.Lock()
	defer s.lock.Unlock()

	session := s.sessions[sessionID]
	s.sessions[sessionID] = nil
	s.sessionCnt--
	if s.handler.OnSessionClosed != nil {
		defer e.OnError("")
		s.handler.OnSessionClosed(session)
	}
}

func (s *Server) GetSessionCount() int {
	return s.sessionCnt
}
