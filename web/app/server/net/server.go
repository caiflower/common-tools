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

package net

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/caiflower/common-tools/pkg/tools"
	appserver "github.com/caiflower/common-tools/web/app/server"
	"github.com/caiflower/common-tools/web/protocol"
	"github.com/caiflower/common-tools/web/router"
)

// 基于云生http.server实现的http服务器

type HttpServer struct {
	logger logger.ILog
	server *http.Server
	*router.Handler
	appserver.Daemon
	cfg NormalConfig
}

type NormalConfig struct {
	router.HandlerCfg
	Addr          string        `yaml:"addr" default:":8080"`
	ReadTimeout   time.Duration `yaml:"readTimeout" default:"20s"`
	WriteTimeout  time.Duration `yaml:"writeTimeout" default:"35s"`
	HandleTimeout time.Duration `yaml:"handleTimeout" default:"60s"` // 请求总处理超时时间
	// Listener, when set, is used instead of binding Addr.
	Listener net.Listener `yaml:"-"`
}

func NewHttpServer(config NormalConfig) *HttpServer {
	_ = tools.DoTagFunc(&config, []tools.FnObj{{Fn: tools.SetDefaultValueIfNil}})

	httpServer := &HttpServer{
		logger: logger.DefaultLogger(),
		cfg:    config,
	}

	httpServer.Handler = router.NewHandler(config.HandlerCfg, httpServer.logger)
	return httpServer
}

func (s *HttpServer) Name() string {
	return fmt.Sprintf("HTTP_SERVER:%s", s.cfg.Name)
}

func (s *HttpServer) Start() error {
	s.server = &http.Server{
		Addr:         s.cfg.Addr,
		ReadTimeout:  s.cfg.ReadTimeout,
		WriteTimeout: s.cfg.WriteTimeout,
		Handler:      s.Handler,
		ConnContext: func(ctx context.Context, c net.Conn) context.Context {
			if s.cfg.HandleTimeout != 0 {
				ctx, _ = context.WithTimeout(ctx, s.cfg.HandleTimeout*time.Second)
			}
			return ctx
		},
	}

	if !s.SetRunning(true) {
		return nil
	}

	s.Handler.SortInterceptors()

	s.logger.Info(
		"\n***************************** http server startup ***********************************************\n"+
			"************* web service [name:%s] [rootPath:%s] listening on %s *********\n"+
			"*************************************************************************************************", s.cfg.Name, s.cfg.RootPath, s.cfg.Addr)

	go func() {
		var err error
		if s.cfg.Listener != nil {
			err = s.server.Serve(s.cfg.Listener)
		} else {
			err = s.server.ListenAndServe()
		}
		if err != nil && err.Error() != "http: Server closed" {
			panic(err)
		}
	}()

	return nil
}

func (s *HttpServer) Close() {
	s.logger.Info("      **** http server shutdown ****")
	if !s.SetRunning(false) {
		return
	}

	if s.server != nil {
		// 30秒超时
		const waitTimeout = time.Second * 30

		// 先停止接收新请求并等待在途请求完成，避免下游资源（kafka/redis/db）
		// 在请求仍被处理时被关闭。drain 和 transport 关闭各持有独立超时，
		// 避免 drain 耗尽预算后 transport 来不及清理。
		drainCtx, drainCancel := context.WithTimeout(context.Background(), waitTimeout)
		if err := s.Handler.Drain(drainCtx); err != nil {
			s.logger.Warn(" **** http server drain error **** error:%s", err.Error())
		}
		drainCancel()

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), waitTimeout)
		if err := s.server.Shutdown(shutdownCtx); err != nil {
			s.logger.Warn(" **** http server shutdown error **** \n"+
				"**** error:%s ****", err.Error())
		}
		shutdownCancel()

		s.logger.Info(" **** http server gracefully shutdown ****")
	}
	s.server = nil
}

func (s *HttpServer) GetLogger() logger.ILog {
	return s.logger
}

func (s *HttpServer) AddProtocol(protocol string, core protocol.Server) {
	panic("not support")
}
