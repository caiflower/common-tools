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

type Options struct {
	Name                     string        `yaml:"name" default:"default"`
	Addr                     string        `yaml:"addr" default:":8080"`
	ReadTimeout              time.Duration `yaml:"readTimeout" default:"20s"`
	WriteTimeout             time.Duration `yaml:"writeTimeout" default:"35s"`
	HandleTimeout            time.Duration `yaml:"handleTimeout" default:"60s"`
	KeepAliveTimeout         time.Duration `yaml:"keepAliveTimeout" default:"60s"`
	RootPath                 string        `yaml:"rootPath"`
	HeaderTraceID            string        `yaml:"headerTraceID" default:"X-Request-Id"`
	ControllerRootPkgName    string        `yaml:"controllerRootPkgName" default:"controller"`
	EnablePprof              bool          `yaml:"enablePprof"`
	Mode                     ServerMode    `yaml:"mode" default:"standard"`
	LimiterEnabled           bool          `yaml:"limiterEnabled"`
	Qps                      int           `yaml:"qps"`
	Network                  string        `yaml:"netWork" default:"tcp"`
	SenseClientDisconnection bool          `yaml:"senseClientDisconnection"`
	ListenConfig             *net.ListenConfig
	OnAccept                 func(conn net.Conn) context.Context
	OnConnect                func(ctx context.Context, conn network.Conn) context.Context
	DisableOptimization      bool `yaml:"optimization"`
	MaxHeaderBytes           int  `yaml:"maxHeaderBytes" default:"1048576"`
	MaxRequestBodySize       int  `yaml:"maxRequestBodySize" default:"10485760"` // 10MB
	EnableMetrics            bool `yaml:"enableMetrics"`
}

func NewOptions(opts []Option) *Options {
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
