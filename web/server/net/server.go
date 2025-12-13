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
	"reflect"
	"time"

	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/caiflower/common-tools/pkg/tools"
	"github.com/caiflower/common-tools/web/router"
)

// 基于云生http.server实现的http服务器

type HttpServer struct {
	logger logger.ILog
	server *http.Server
	*router.Handler
	cfg NormalConfig
}

type NormalConfig struct {
	router.HandlerCfg
	Port          uint  `yaml:"port" default:"8080"`
	ReadTimeout   uint  `yaml:"readTimeout" default:"20"`
	WriteTimeout  uint  `yaml:"writeTimeout" default:"35"`
	HandleTimeout *uint `yaml:"handleTimeout" default:"60"` // 请求总处理超时时间
}

func NewHttpServer(config NormalConfig) *HttpServer {
	_ = tools.DoTagFunc(&config, nil, []func(reflect.StructField, reflect.Value, interface{}) error{tools.SetDefaultValueIfNil})

	httpServer := &HttpServer{
		logger: logger.DefaultLogger(),
	}

	// 初始化公共处理器
	router.InitHandler(config.HandlerCfg, httpServer.logger)

	httpServer.Handler = router.CommonHandler
	return httpServer
}

func (s *HttpServer) Name() string {
	return fmt.Sprintf("HTTP_SERVER:%s", s.cfg.Name)
}

func (s *HttpServer) Start() error {
	s.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", s.cfg.Port),
		ReadTimeout:  time.Duration(s.cfg.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(s.cfg.WriteTimeout) * time.Second,
		Handler:      router.CommonHandler,
		ConnContext: func(ctx context.Context, c net.Conn) context.Context {
			if s.cfg.HandleTimeout != nil {
				ctx, _ = context.WithTimeout(ctx, time.Duration(*s.cfg.HandleTimeout)*time.Second)
			}
			return ctx
		},
	}

	s.Handler.SortInterceptors()

	s.logger.Info(
		"\n***************************** http server startup ***********************************************\n"+
			"************* web service [name:%s] [rootPath:%s] listening on %d *********\n"+
			"*************************************************************************************************", s.cfg.Name, s.cfg.RootPath, s.cfg.Port)

	go func() {
		if err := s.server.ListenAndServe(); err != nil {
			if err.Error() != "http: Server closed" {
				panic(err)
			}
		}
	}()

	return nil
}

func (s *HttpServer) Close() {
	s.logger.Info("      **** http server shutdown ****")
	if s.server != nil {
		// 30秒超时
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
		defer cancel()
		if err := s.server.Shutdown(ctx); err != nil {
			s.logger.Warn(" **** http server shutdown error **** \n"+
				"**** error:%s ****", err.Error())
		}
		s.logger.Info(" **** http server gracefully shutdown ****")
	}
	s.server = nil
}
