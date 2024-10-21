package nio

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/caiflower/common-tools/pkg/e"
	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/caiflower/common-tools/pkg/syncx"
)

type IClient interface {
	Connect() error
	Write(flag uint8, data interface{}) error
	Close()
}

type Client struct {
	socket
	session *Session
	logger  logger.ILog
}

func NewClientWithAllArgs(config *Config, locker sync.Locker, logger logger.ILog, handler *Handler) *Client {
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
	if config.Timeout <= 0 {
		config.Timeout = 10
	}
	if handler.codec == nil {
		handler.codec = GetZipCodec(logger)
	}
	handler.logger = logger
	return &Client{
		socket: socket{
			addr:    config.Addr,
			handler: handler,
			closed:  true,
			timeout: config.Timeout,
		},
		logger: logger,
	}
}

func NewClient(config *Config, handler *Handler) *Client {
	return NewClientWithAllArgs(config, syncx.NewSpinLock(), logger.DefaultLogger(), handler)
}

func (c *Client) Connect() error {
	c.logger.Info("[client] Connect to server %s ...", c.addr)

	// 连接
	connection, err := net.DialTimeout("tcp", c.addr, 10*time.Second)
	if err != nil {
		c.logger.Error("[client] Connect to server %s err: %s", c.addr, err.Error())
		return err
	}

	session := &Session{
		codec:      c.handler.codec,
		attribute:  make(map[string]interface{}),
		logger:     c.logger,
		connection: connection,
		client:     c,
	}

	ctx, _ := context.WithTimeout(context.Background(), time.Duration(c.timeout)*time.Second)
	if err = c.syncSession(ctx, session); err != nil {
		c.logger.Error("[client] syncSession failed. addr: %s err: %s", c.addr, err.Error())
		connection.Close()
		return err
	}

	if c.handler.OnSessionConnected != nil {
		func() {
			defer e.OnError("")
			c.handler.OnSessionConnected(session)
		}()
	}
	c.logger.Info("[client] [%d] Connect to server %s success. Local %s", session.id, c.addr, connection.LocalAddr())

	c.closed = false
	c.session = session
	go c.handler.readIO(&c.socket, session)

	return nil
}

func (c *Client) syncSession(ctx context.Context, session *Session) error {
	var sessionId int64
	firstData := bytes.NewBuffer([]byte{})
	firstDataLen := 8
	for {
		select {
		case <-ctx.Done():
			return errors.New("syncSession timeout")
		default:
			buf := make([]byte, firstDataLen)
			l, ioErr := session.connection.Read(buf)
			if ioErr != nil {
				return ioErr
			}
			if l > 0 {
				firstData.Write(buf[0:l])
			}
			firstDataLen = firstDataLen - l
		}
		if firstDataLen == 0 {
			break
		}
	}
	if err := binary.Read(firstData, binary.BigEndian, &sessionId); err != nil {
		return err
	}
	session.id = sessionId
	return nil
}

func (c *Client) Write(flag uint8, data interface{}) error {
	if err := c.session.WriteMsg(&Msg{flag: flag, body: data}); err != nil {
		c.logger.Error("[client] Write err: %s", err.Error())
	}
	return nil
}

func (c *Client) Close() {
	err := c.session.connection.Close()
	if err != nil {
		if strings.Contains(err.Error(), "use of closed network") {
			return
		} else {
			c.logger.Warn("[client] [%d] close connection err: %s", c.session.id, err.Error())
			return
		}
	}
	if c.handler.OnSessionClosed != nil {
		defer e.OnError("")
		c.handler.OnSessionClosed(c.session)
	}
	c.closed = true
}
