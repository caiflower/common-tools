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

package netx

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"sync"
	"time"

	golocalv1 "github.com/caiflower/common-tools/pkg/golocal/v1"
	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/caiflower/common-tools/pkg/tools"
	"github.com/caiflower/common-tools/web/common/config"
	errs "github.com/caiflower/common-tools/web/common/errors"
	"github.com/caiflower/common-tools/web/common/webctx"
	"github.com/caiflower/common-tools/web/network"
	"github.com/caiflower/common-tools/web/network/netpoll"
	"github.com/caiflower/common-tools/web/protocol"
	"github.com/caiflower/common-tools/web/router"
)

var (
	errIdleTimeout = errs.New(errs.ErrIdleTimeout, errs.ErrorTypePrivate, nil)
)

type HttpServer struct {
	*router.Handler

	options        *config.Options
	logger         logger.ILog
	transporter    network.Transporter
	ctx            context.Context
	cancel         context.CancelFunc
	requestCtxPool sync.Pool
}

func NewHttpServer(options config.Options) *HttpServer {
	_ = tools.DoTagFunc(&options, nil, []func(reflect.StructField, reflect.Value, interface{}) error{tools.SetDefaultValueIfNil})

	ctx, cancel := context.WithCancel(context.Background())

	s := &HttpServer{
		options:     &options,
		logger:      logger.DefaultLogger(),
		ctx:         ctx,
		cancel:      cancel,
		transporter: netpoll.NewTransporter(&options),
	}
	s.requestCtxPool.New = func() interface{} {
		return &webctx.RequestCtx{
			XRequest: &protocol.Request{},
			XResponse: &protocol.Response{
				Headers: make(map[string][]string),
			},
		}
	}

	// 初始化公共处理器
	router.InitHandler(s.getHandlerCfg(), s.logger)
	s.Handler = router.CommonHandler

	return s
}

func (s *HttpServer) Name() string {
	return fmt.Sprintf("NETPOLL_HTTP_SERVER:%s", s.options.Name)
}

func (s *HttpServer) Start() error {
	s.logger.Info(
		"\n***************************** netx http server startup ***************************************\n"+
			"************* web service [name:%s] [rootPath:%s] listening on %s *********\n"+
			"*************************************************************************************************", s.options.Name, s.options.RootPath, s.options.Addr)

	var err error
	if err = s.transporter.ListenAndServe(s.OnReq); err != nil {
		s.logger.Error("ListenAndServe failed. Error: %s", err.Error())
	}

	return err
}

func (s *HttpServer) Close() {
	s.logger.Info("      **** netx http server shutdown ****")

	if s.cancel != nil {
		s.cancel()
	}

	timeout, cancelFunc := context.WithTimeout(context.Background(), time.Second*5)
	defer cancelFunc()
	if err := s.transporter.Shutdown(timeout); err != nil {
		s.logger.Error("http server shutdown failed. Error: %s", err.Error())
	}
}

func (s *HttpServer) getHandlerCfg() router.HandlerCfg {
	options := s.options
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
	}
}

func (s *HttpServer) OnReq(c context.Context, conn interface{}) (err error) {
	var (
		cancel         context.CancelFunc
		zr             network.Reader
		connRequestNum = uint64(0)
		req            *http.Request
		headerID       = s.options.HeaderTraceID
		zw             network.Writer
	)

	if s.options.HandleTimeout != 0 {
		c, cancel = context.WithTimeout(c, s.options.HandleTimeout)
		defer cancel()
	}

	ctx := s.getRequestContext()
	ctx = ctx.SetConn(conn.(network.Conn)).SetContext(c)

	defer func() {
		s.requestCtxPool.Put(ctx)
		ctx.GetConn().Close()
	}()

	for {
		if zr == nil {
			zr = ctx.GetReader()
		}
		if zw == nil {
			zw = ctx.GetWriter()
		}

		connRequestNum++

		// If this is a keep-alive connection we want to try and read the first bytes
		// within the idle time.
		if connRequestNum > 1 {
			_ = ctx.GetConn().SetReadTimeout(s.options.KeepAliveTimeout)

			_, err = zr.Peek(4)
			// This is not the first request, and we haven't read a single byte
			// of a new request yet. This means it's just a keep-alive connection
			// closing down either because the remote closed it or because
			// or a read timeout on our side. Either way just close the connection
			// and don't return any error response.
			if err != nil {
				err = errIdleTimeout
				return
			}

			// Reset the real read timeout for the coming request
			_ = ctx.GetConn().SetReadTimeout(s.options.ReadTimeout)
		}

		req, err = ctx.GetHttpRequest()
		if err != nil {
			s.logger.Error("GetHttpRequest failed. Error: %s", err.Error())
			return
		}

		ctx.Method = req.Method
		ctx.Params = req.URL.Query()
		ctx.Path = req.URL.Path

		var traceID string
		if headerID != "" {
			traceID = req.Header.Get(headerID)
		}
		if traceID == "" {
			traceID = tools.UUID()
			if headerID != "" {
				req.Header.Set(headerID, traceID)
			}
		}
		golocalv1.PutTraceID(traceID)
		golocalv1.Put(router.BeginTime, time.Now())
		golocalv1.PutContext(c)

		s.Handler.Dispatch(ctx, ctx.XResponse, req)
		err = writeResponse(ctx, zw)
		if err != nil {
			return
		}

		_ = zr.Release()
		golocalv1.Clean()
		ctx.Reset()
	}
}

func (s *HttpServer) getRequestContext() *webctx.RequestCtx {
	return s.requestCtxPool.Get().(*webctx.RequestCtx)
}

func writeResponse(ctx *webctx.RequestCtx, w network.Writer) error {
	bytes := ctx.XResponse.ByteBuffer.Bytes()

	_, err := w.WriteBinary(bytes)
	if err != nil {
		return err
	}

	w.Flush()

	return err
}
