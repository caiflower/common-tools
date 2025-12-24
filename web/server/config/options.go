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

package config

import (
	"context"
	"net"
	"time"

	"github.com/caiflower/common-tools/pkg/tools"
	"github.com/caiflower/common-tools/web/network"
)

type ServerMode string

const (
	ServerModeStandard ServerMode = "standard" // 标准 HTTP 服务器
	ServerModeNetpoll  ServerMode = "netpoll"  // Netpoll 高性能服务器
)

type Option func(*Options) *Options
type Http2Option func(*Http2Options) *Http2Options

type Options struct {
	Name                     string        `yaml:"name" default:"default"`
	Addr                     string        `yaml:"addr" default:":8080"`
	HeaderTraceID            string        `yaml:"headerTraceID" default:"X-Request-Id"`
	ControllerRootPkgName    string        `yaml:"controllerRootPkgName" default:"controller"`
	RootPath                 string        `yaml:"rootPath"`
	ReadTimeout              time.Duration `yaml:"readTimeout" default:"20s"`
	WriteTimeout             time.Duration `yaml:"writeTimeout" default:"35s"`
	IdleTimeout              time.Duration `yaml:"idleTimeout" default:"60s"`
	HandleTimeout            time.Duration `yaml:"handleTimeout" default:"60s"`
	KeepAliveTimeout         time.Duration `yaml:"keepAliveTimeout" default:"180s"`
	EnablePprof              bool          `yaml:"enablePprof"`
	Mode                     ServerMode    `yaml:"mode" default:"netpoll"`
	LimiterEnabled           bool          `yaml:"limiterEnabled"`
	Qps                      int           `yaml:"qps"`
	Network                  string        `yaml:"netWork" default:"tcp"`
	SenseClientDisconnection bool          `yaml:"senseClientDisconnection"`
	ListenConfig             *net.ListenConfig
	OnAccept                 func(conn net.Conn) context.Context
	OnConnect                func(ctx context.Context, conn network.Conn) context.Context
	DisableOptimization      bool `yaml:"disableOptimization"`
	MaxHeaderBytes           int  `yaml:"maxHeaderBytes" default:"1048576"`
	MaxRequestBodySize       int  `yaml:"maxRequestBodySize" default:"10485760"` // 10MB
	EnableMetrics            bool `yaml:"enableMetrics"`
	DisableKeepalive         bool `yaml:"disableKeepalive"`
	EnableTrace              bool `yaml:"enableTrace"`
	H2C                      bool `yaml:"h2c"`
}

func NewOptions(opts ...Option) *Options {
	options := &Options{}
	_ = tools.DoTagFunc(options, []tools.FnObj{{Fn: tools.SetDefaultValueIfNil}})

	for _, opt := range opts {
		options = opt(options)
	}
	return options
}

func WithName(name string) Option {
	return func(opts *Options) *Options {
		opts.Name = name
		return opts
	}
}

func WithAddr(addr string) Option {
	return func(opts *Options) *Options {
		opts.Addr = addr
		return opts
	}
}

func WithReadTimeout(readTimeout time.Duration) Option {
	return func(opts *Options) *Options {
		opts.ReadTimeout = readTimeout
		return opts
	}
}

func WithWriteTimeout(writeTimeout time.Duration) Option {
	return func(opts *Options) *Options {
		opts.WriteTimeout = writeTimeout
		return opts
	}
}

func WithHandleTimeout(handleTimeout time.Duration) Option {
	return func(opts *Options) *Options {
		opts.HandleTimeout = handleTimeout
		return opts
	}
}

func WithRootPath(path string) Option {
	return func(opts *Options) *Options {
		opts.RootPath = path
		return opts
	}
}

func WithHeaderTraceID(headerTraceID string) Option {
	return func(opts *Options) *Options {
		opts.HeaderTraceID = headerTraceID
		return opts
	}
}

func WithControllerRootPkgName(controllerRootPkgName string) Option {
	return func(opts *Options) *Options {
		opts.ControllerRootPkgName = controllerRootPkgName
		return opts
	}
}

func WithEnablePprof(enablePprof bool) Option {
	return func(opts *Options) *Options {
		opts.EnablePprof = enablePprof
		return opts
	}
}

func WithMode(mode ServerMode) Option {
	return func(opts *Options) *Options {
		opts.Mode = mode
		return opts
	}
}

func WithQps(enable bool, qps int) Option {
	return func(opts *Options) *Options {
		opts.Qps = qps
		opts.LimiterEnabled = enable
		return opts
	}
}

func WithSenseClientDisconnection(senseClientDisconnection bool) Option {
	return func(opts *Options) *Options {
		opts.SenseClientDisconnection = senseClientDisconnection
		return opts
	}
}

func WithOnAccept(onAccept func(conn net.Conn) context.Context) Option {
	return func(opts *Options) *Options {
		opts.OnAccept = onAccept
		return opts
	}
}

func WithOnConnect(onConnect func(context.Context, network.Conn) context.Context) Option {
	return func(opts *Options) *Options {
		opts.OnConnect = onConnect
		return opts
	}
}

func WithListenConfig(listenConfig *net.ListenConfig) Option {
	return func(opts *Options) *Options {
		opts.ListenConfig = listenConfig
		return opts
	}
}

func WithDisableOptimization(opt bool) Option {
	return func(opts *Options) *Options {
		opts.DisableOptimization = opt
		return opts
	}
}

func WithDisableKeepalive(opt bool) Option {
	return func(opts *Options) *Options {
		opts.DisableKeepalive = opt
		return opts
	}
}

func WithH2C(opt bool) Option {
	return func(opts *Options) *Options {
		opts.H2C = opt
		return opts
	}
}

type Http2Options struct {
	Options
	// MaxUploadBufferPerConnection is the size of the initial flow
	// control window for each connections. The HTTP/2 spec does not
	// allow this to be smaller than 65535 or larger than 2^32-1.
	// If the value is outside this range, a default value will be
	// used instead.
	MaxUploadBufferPerConnection int32

	// MaxUploadBufferPerStream is the size of the initial flow control
	// window for each stream. The HTTP/2 spec does not allow this to
	// be larger than 2^32-1. If the value is zero or larger than the
	// maximum, a default value will be used instead.
	MaxUploadBufferPerStream int32

	// MaxReadFrameSize optionally specifies the largest frame
	// this server is willing to read. A valid value is between
	// 16k and 16M, inclusive. If zero or otherwise invalid, a
	// default value is used.
	MaxReadFrameSize uint32

	// MaxConcurrentStreams optionally specifies the number of
	// concurrent streams that each client may have open at a
	// time. This is unrelated to the number of http.Handler goroutines
	// which may be active globally, which is MaxHandlers.
	// If zero, MaxConcurrentStreams defaults to at least 100, per
	// the HTTP/2 spec's recommendations.
	MaxConcurrentStreams uint32

	// PermitProhibitedCipherSuites, if true, permits the use of
	// cipher suites prohibited by the HTTP/2 spec.
	PermitProhibitedCipherSuites bool
}

func NewHttp2Options(http1 Options, opts ...Http2Option) *Http2Options {
	options := &Http2Options{Options: http1}
	_ = tools.DoTagFunc(options, []tools.FnObj{{Fn: tools.SetDefaultValueIfNil}})

	for _, opt := range opts {
		options = opt(options)
	}
	return options
}

func WithMaxUploadBufferPerConnection(maxUploadBufferPerConnection int32) Http2Option {
	return func(opts *Http2Options) *Http2Options {
		opts.MaxUploadBufferPerConnection = maxUploadBufferPerConnection
		return opts
	}
}

func WithMaxUploadBufferPerStream(maxUploadBufferPerStream int32) Http2Option {
	return func(opts *Http2Options) *Http2Options {
		opts.MaxUploadBufferPerStream = maxUploadBufferPerStream
		return opts
	}
}

func WithMaxReadFrameSize(maxReadFrameSize uint32) Http2Option {
	return func(opts *Http2Options) *Http2Options {
		opts.MaxReadFrameSize = maxReadFrameSize
		return opts
	}
}

func WithMaxConcurrentStreams(maxConcurrentStreams uint32) Http2Option {
	return func(opts *Http2Options) *Http2Options {
		opts.MaxConcurrentStreams = maxConcurrentStreams
		return opts
	}
}

func WithPermitProhibitedCipherSuites(permitProhibitedCipherSuites bool) Http2Option {
	return func(opts *Http2Options) *Http2Options {
		opts.PermitProhibitedCipherSuites = permitProhibitedCipherSuites
		return opts
	}
}
