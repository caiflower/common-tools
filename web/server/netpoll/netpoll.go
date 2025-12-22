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
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/caiflower/common-tools/pkg/tools"
	requstoption "github.com/caiflower/common-tools/web/common/config"
	errs "github.com/caiflower/common-tools/web/common/errors"
	"github.com/caiflower/common-tools/web/common/webctx"
	"github.com/caiflower/common-tools/web/network"
	"github.com/caiflower/common-tools/web/network/netpoll"
	"github.com/caiflower/common-tools/web/protocol/http1/req"
	"github.com/caiflower/common-tools/web/router"
	"github.com/caiflower/common-tools/web/server/config"
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
		ctx := &webctx.RequestCtx{}
		ctx.HttpRequest.SetOptions(
			requstoption.WithReadTimeout(options.ReadTimeout),
			requstoption.WithWriteTimeout(options.WriteTimeout),
		)
		return ctx
	}
	s.Handler = router.NewHandler(s.getHandlerCfg(), s.logger)

	return s
}

func (s *HttpServer) Name() string {
	return fmt.Sprintf("NETPOLL_HTTP_SERVER:%s", s.options.Name)
}

func (s *HttpServer) Start() error {
	s.logger.Info(
		"\n***************************** netpoll http server startup ***************************************\n"+
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
	s.logger.Info("      **** netpoll http server shutdown, wait at most 5s ****")

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
		EnableMetrics:       options.EnableMetrics,
		DisableOptimization: options.DisableOptimization,
	}
}

func (s *HttpServer) OnReq(c context.Context, cc interface{}) (err error) {
	var (
		cancel         context.CancelFunc
		zr             network.Reader
		connRequestNum = uint64(0)
		request        *http.Request
		zw             network.Writer
		conn           network.Conn
	)
	conn = cc.(network.Conn)

	if s.options.HandleTimeout != 0 {
		c, cancel = context.WithTimeout(c, s.options.HandleTimeout)
		defer cancel()
	}

	ctx := s.getRequestContext()
	ctx = ctx.SetConn(conn).SetContext(c)
	zr = ctx.GetReader()
	zw = ctx.GetWriter()

	defer func() {
		s.requestCtxPool.Put(ctx)
		_ = conn.Close()
	}()

	for {
		connRequestNum++

		// If this is a keep-alive connection we want to try and read the first bytes
		// within the idle time.
		if connRequestNum > 1 {
			_ = ctx.GetConn().SetReadTimeout(s.options.IdleTimeout)

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

		if !s.options.DisableOptimization {
			if err = req.ReadHeaderWithLimit(&ctx.HttpRequest.Header, zr, s.options.MaxHeaderBytes); err == nil {
				// Read body
				//if s.StreamRequestBody {
				//	err = req.ReadBodyStream(&ctx.Request, zr, s.MaxRequestBodySize, s.GetOnly, !s.DisablePreParseMultipartForm)
				//} else {
				err = req.ReadLimitBody(&ctx.HttpRequest, zr, s.options.MaxRequestBodySize, false, false)
				if err != nil {
					return
				}

				ctx.SetMethod(ctx.HttpRequest.Method())
				ctx.SetPath(ctx.HttpRequest.Path())
			}
		} else {
			request, err = ctx.GetHttpRequest()
			ctx = router.InitCtx(ctx, nil, request)
		}

		s.Handler.Serve(ctx)
		if err != nil {
			s.logger.Error("GetHttpRequest failed. Error: %s", err.Error())
			err = errParseRequest
			return
		}

		err = writeResponse(ctx, zw)
		if err != nil {
			return
		}

		_ = zw.Flush()
		_ = zr.Release()
		ctx.Reset()
	}
}

func (s *HttpServer) getRequestContext() *webctx.RequestCtx {
	return s.requestCtxPool.Get().(*webctx.RequestCtx)
}

func writeResponse(ctx *webctx.RequestCtx, w network.Writer) error {
	resp := &ctx.HttpResponse

	body := resp.Body()
	bodyLen := len(body)
	resp.Header.SetContentLength(bodyLen)

	header := resp.Header.Header()
	_, err := w.WriteBinary(header)
	if err != nil {
		return err
	}

	resp.Header.SetHeaderLength(len(header))
	// Write body
	if bodyLen > 0 {
		_, err = w.WriteBinary(body)
	}
	return err
}
