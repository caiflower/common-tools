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

package netpoll

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/caiflower/common-tools/pkg/tools"
	"github.com/caiflower/common-tools/web/common/bytestr"
	requstoption "github.com/caiflower/common-tools/web/common/config"
	"github.com/caiflower/common-tools/web/common/utils"
	"github.com/caiflower/common-tools/web/common/webctx"
	"github.com/caiflower/common-tools/web/network"
	"github.com/caiflower/common-tools/web/network/netpoll"
	"github.com/caiflower/common-tools/web/network/standard"
	"github.com/caiflower/common-tools/web/protocol"
	"github.com/caiflower/common-tools/web/protocol/http1"
	"github.com/caiflower/common-tools/web/protocol/http2"
	"github.com/caiflower/common-tools/web/protocol/suit"
	"github.com/caiflower/common-tools/web/router"
	"github.com/caiflower/common-tools/web/router/param"
	"github.com/caiflower/common-tools/web/server/config"
)

type HttpServer struct {
	*router.Handler

	config.Options
	logger          logger.ILog
	transporter     network.Transporter
	requestCtxPool  sync.Pool
	startLock       sync.Mutex
	protocolServers map[string]protocol.Server
}

func NewHttpServer(options config.Options) *HttpServer {
	_ = tools.DoTagFunc(&options, []tools.FnObj{{Fn: tools.SetDefaultValueIfNil}})

	s := &HttpServer{
		Options:         options,
		logger:          logger.DefaultLogger(),
		protocolServers: make(map[string]protocol.Server),
	}
	s.requestCtxPool.New = func() interface{} {
		ctx := &webctx.RequestCtx{
			Paths: make(param.Params, 0, 10),
		}
		ctx.Request.SetOptions(
			requstoption.WithReadTimeout(options.ReadTimeout),
			requstoption.WithWriteTimeout(options.WriteTimeout),
		)
		return ctx
	}
	s.Handler = router.NewHandler(s.getHandlerCfg(), s.logger)

	// http1
	s.protocolServers[suit.HTTP1] = &http1.Server{
		Core:    s,
		Options: options,
	}

	if s.H2C {
		s.AddProtocol(suit.HTTP2, &http2.Server{
			BaseEngine: http2.BaseEngine{
				Options: options,
				Core:    s,
			},
		})
	}

	if s.TLS != nil {
		if !s.H2C {
			s.AddProtocol(suit.HTTP2, &http2.Server{
				BaseEngine: http2.BaseEngine{
					Options: options,
					Core:    s,
				},
			})
		}
		s.transporter = standard.NewTransporter(&options)
	} else {
		s.transporter = netpoll.NewTransporter(&options)
	}

	return s
}

func (s *HttpServer) AddProtocol(protocol string, core protocol.Server) {
	s.protocolServers[protocol] = core
}

func (s *HttpServer) Name() string {
	return fmt.Sprintf("NETPOLL_HTTP_SERVER:%s", s.Options.Name)
}

func (s *HttpServer) Start() error {
	s.logger.Info(
		"\n***************************** netpoll http server startup ***************************************\n"+
			"************* web service [name:%s] [rootPath:%s] listening on %s *********\n"+
			"*************************************************************************************************", s.Name, s.RootPath, s.Addr)
	if s.IsRunning() {
		return nil
	}

	s.startLock.Lock()
	defer s.startLock.Unlock()
	if s.IsRunning() {
		return nil
	}

	s.Handler.SortInterceptors()
	s.SetRunning(true)
	var err error
	go func() {
		if err = s.transporter.ListenAndServe(s.OnReq); err != nil {
			s.logger.Error("ListenAndServe failed. Error: %s", err.Error())
		}
	}()

	return err
}

func (s *HttpServer) Close() {
	s.logger.Info("      **** netpoll http server shutdown, wait at most 5s ****")

	s.SetRunning(false)
	timeout, cancelFunc := context.WithTimeout(context.Background(), time.Second*5)
	defer cancelFunc()
	if err := s.transporter.Shutdown(timeout); err != nil {
		s.logger.Error("http server shutdown failed. Error: %s", err.Error())
	}
}

func (s *HttpServer) getHandlerCfg() router.HandlerCfg {
	options := s.Options
	return router.HandlerCfg{
		Name:                  options.Name,
		RootPath:              options.RootPath,
		HeaderTraceID:         options.HeaderTraceID,
		ControllerRootPkgName: options.ControllerRootPkgName,
		EnablePprof:           options.EnablePprof,
		WebLimiter: router.LimiterConfig{
			Enable: options.LimiterEnabled,
			Qos:    options.Qps,
		},
		EnableMetrics:       options.EnableMetrics,
		DisableOptimization: options.DisableOptimization,
	}
}

func (s *HttpServer) OnReq(c context.Context, cc interface{}) (err error) {
	conn := cc.(network.Conn)
	// H2C path
	if s.H2C {
		// protocol sniffer
		buf, _ := conn.Peek(len(bytestr.StrClientPreface))
		if bytes.Equal(buf, bytestr.StrClientPreface) && s.protocolServers[suit.HTTP2] != nil {
			return s.protocolServers[suit.HTTP2].Serve(c, conn)
		}
		s.logger.Warn("HTTP2 server is not loaded, request is going to fallback to HTTP1 server")
	}

	if s.ALPN && s.TLS != nil {
		proto, err1 := s.getNextProto(conn)
		if err1 != nil {
			// The client closes the connection when handshake. So just ignore it.
			if err1 == io.EOF {
				return nil
			}
			if re, ok := err1.(tls.RecordHeaderError); ok && re.Conn != nil && utils.TLSRecordHeaderLooksLikeHTTP(re.RecordHeader) {
				_, _ = io.WriteString(re.Conn, "HTTP/1.0 400 Bad Request\r\n\r\nClient sent an HTTP request to an HTTPS server.\n")
				_ = re.Conn.Close()
				return re
			}
			return err1
		}
		if server, ok := s.protocolServers[proto]; ok {
			return server.Serve(c, conn)
		}
	}

	// HTTP1 path
	err = s.protocolServers[suit.HTTP1].Serve(c, conn)

	return
}

func (s *HttpServer) getNextProto(conn network.Conn) (proto string, err error) {
	if tlsConn, ok := conn.(network.ConnTLSer); ok {
		if s.ReadTimeout > 0 {
			if err := conn.SetReadTimeout(s.ReadTimeout); err != nil {
				s.logger.Error("BUG: error in SetReadDeadline=%s: error=%s", s.ReadTimeout, err)
			}
		}
		err = tlsConn.Handshake()
		if err == nil {
			proto = tlsConn.ConnectionState().NegotiatedProtocol
		}
	}
	return
}

func (s *HttpServer) getRequestContext() *webctx.RequestCtx {
	return s.requestCtxPool.Get().(*webctx.RequestCtx)
}

func (s *HttpServer) GetCtxPool() *sync.Pool {
	return &s.requestCtxPool
}

func (s *HttpServer) GetLogger() logger.ILog {
	return s.logger
}
