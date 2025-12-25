package http1

import (
	"context"

	errs "github.com/caiflower/common-tools/web/common/errors"
	"github.com/caiflower/common-tools/web/common/webctx"
	"github.com/caiflower/common-tools/web/network"
	"github.com/caiflower/common-tools/web/protocol/http1/req"
	"github.com/caiflower/common-tools/web/protocol/suit"
	"github.com/caiflower/common-tools/web/server/config"
)

var (
	errIdleTimeout  = errs.New(errs.ErrIdleTimeout, errs.ErrorTypePrivate, nil)
	errParseRequest = errs.NewPrivate("parseRequest failed")
)

type Server struct {
	suit.Core
	config.Options
}

func (s *Server) Serve(c context.Context, conn network.Conn) (err error) {
	var (
		cancel         context.CancelFunc
		zr             network.Reader
		connRequestNum = uint64(0)
		zw             network.Writer
	)

	if s.HandleTimeout != 0 {
		c, cancel = context.WithTimeout(c, s.HandleTimeout)
		defer cancel()
	}

	ctx := s.GetCtxPool().Get().(*webctx.RequestCtx)
	ctx = ctx.SetConn(conn).SetContext(c)
	zr = ctx.GetReader()
	zw = ctx.GetWriter()

	defer func() {
		s.GetCtxPool().Put(ctx)
		_ = conn.Close()
	}()

	for {
		connRequestNum++

		// If this is a keep-alive connection we want to try and read the first bytes
		// within the idle time.
		if connRequestNum > 1 {
			_ = ctx.GetConn().SetReadTimeout(s.IdleTimeout)

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
			_ = ctx.GetConn().SetReadTimeout(s.ReadTimeout)
		}

		if err = req.ReadHeaderWithLimit(&ctx.Request.Header, zr, s.MaxHeaderBytes); err == nil {
			// Read body
			//if s.StreamRequestBody {
			//	err = req.ReadBodyStream(&ctx.Request, zr, s.MaxRequestBodySize, s.GetOnly, !s.DisablePreParseMultipartForm)
			//} else {
			err = req.ReadLimitBody(&ctx.Request, zr, s.MaxRequestBodySize, false, false)
			if err != nil {
				return
			}
		}

		s.Core.Serve(ctx)
		if err != nil {
			s.GetLogger().Error("GetHttpRequest failed. Error: %s", err.Error())
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

func writeResponse(ctx *webctx.RequestCtx, w network.Writer) error {
	resp := &ctx.Response

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
