package nio

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"github.com/caiflower/common-tools/pkg/e"
	"github.com/caiflower/common-tools/pkg/logger"
	"net"
	"time"
)

type IClient interface {
	Connect() error
	Write(data interface{}) error
	Close()
}

type Client struct {
	socket
	session *Session
	logger  logger.ILog
}

func NewClientWithAllArgs(config *Config, logger logger.ILog, handler *Handler) *Client {
	if config.Timeout <= 0 {
		config.Timeout = 10
	}
	if handler.codec == nil {
		handler.codec = GetZipCodec(logger)
	}
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
	return NewClientWithAllArgs(config, logger.DefaultLogger(), handler)
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
	}

	ctx, _ := context.WithTimeout(context.Background(), time.Duration(c.timeout)*time.Second)
	if err = c.syncSession(ctx, session); err != nil {
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
			len, ioErr := session.connection.Read(buf)
			if ioErr != nil {
				return ioErr
			}
			if len > 0 {
				firstData.Write(buf[0:len])
			}
			firstDataLen = firstDataLen - len
		}
		if firstDataLen == 0 {
			break
		}
	}
	binary.Read(firstData, binary.BigEndian, &sessionId)
	session.id = sessionId
	return nil
}

func (c *Client) Write(data interface{}) error {
	return nil
}

func (c *Client) write(flag uint32, data interface{}) {

}

func (c *Client) Close() {
	c.session.connection.Close()
}
