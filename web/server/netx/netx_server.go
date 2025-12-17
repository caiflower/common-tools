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
	"sync"
	"time"

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
	errIdleTimeout  = errs.New(errs.ErrIdleTimeout, errs.ErrorTypePrivate, nil)
	errParseRequest = errs.NewPrivate("parseRequest failed")
)

type HttpServer struct {
	*router.Handler

	options        *config.Options
	logger         logger.ILog
	transporter    network.Transporter
	requestCtxPool sync.Pool
}

func NewHttpServer(options config.Options) *HttpServer {
	_ = tools.DoTagFunc(&options, []tools.FnObj{{Fn: tools.SetDefaultValueIfNil}})

	s := &HttpServer{
		options:     &options,
		logger:      logger.DefaultLogger(),
		transporter: netpoll.NewTransporter(&options),
	}
	s.requestCtxPool.New = func() interface{} {
		wctx := &webctx.RequestCtx{
			XResponse: &protocol.Response{
				Headers: make(map[string][]string),
			},
		}
		wctx.XResponse.WriteHeader(http.StatusOK)
		return wctx
	}
	s.Handler = router.NewHandler(s.getHandlerCfg(), s.logger)

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
	go func() {
		if err = s.transporter.ListenAndServe(s.OnReq); err != nil {
			s.logger.Error("ListenAndServe failed. Error: %s", err.Error())
		}
	}()

	return err
}

func (s *HttpServer) Close() {
	s.logger.Info("      **** netx http server shutdown, wait at most 5s ****")

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
		Debug: options.Debug,
	}
}

func (s *HttpServer) OnReq(c context.Context, conn interface{}) (err error) {
	var (
		cancel         context.CancelFunc
		zr             network.Reader
		connRequestNum = uint64(0)
		req            *http.Request
		zw             network.Writer
	)

	if s.options.HandleTimeout != 0 {
		c, cancel = context.WithTimeout(c, s.options.HandleTimeout)
		defer cancel()
	}

	ctx := s.getRequestContext()
	ctx = ctx.SetConn(conn.(network.Conn)).SetContext(c)
	zr = ctx.GetReader()
	zw = ctx.GetWriter()

	defer func() {
		s.requestCtxPool.Put(ctx)
		_ = ctx.GetConn().Close()
	}()

	for {
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

		start := time.Now()
		req, err = ctx.GetHttpRequest()

		fmt.Println("demo-api", time.Since(start).Nanoseconds())
		if err != nil {
			s.logger.Error("GetHttpRequest failed. Error: %s", err.Error())
			err = errParseRequest
			return
		}

		ctx.Method = req.Method
		ctx.Params = req.URL.Query()
		ctx.Path = req.URL.Path
		ctx.Request = req
		ctx.Writer = ctx.XResponse

		s.Handler.ServeHTTPWithRequestContext(ctx, ctx.XResponse, req)
		err = writeResponse(ctx, zw)
		if err != nil {
			return
		}

		_ = zr.Release()
		ctx.Reset()
	}
}

func (s *HttpServer) getRequestContext() *webctx.RequestCtx {
	return s.requestCtxPool.Get().(*webctx.RequestCtx)
}

func writeResponse(ctx *webctx.RequestCtx, w network.Writer) error {
	bytes := ctx.XResponse.Bytes()

	_, err := w.WriteBinary(bytes)
	if err != nil {
		return err
	}

	err = w.Flush()

	return err
}
