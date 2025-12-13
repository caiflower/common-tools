package cfg

import (
	"reflect"

	"github.com/caiflower/common-tools/pkg/tools"
)

type ServerMode string

const (
	ServerModeStandard ServerMode = "standard" // 标准 HTTP 服务器
	ServerModeNetpoll  ServerMode = "netpoll"  // Netpoll 高性能服务器
)

type Option func(*Options) *Options

type Options struct {
	Name                  string     `yaml:"name" default:"default"`
	Port                  uint       `yaml:"port" default:"8080"`
	ReadTimeout           uint       `yaml:"readTimeout" default:"20"`
	WriteTimeout          uint       `yaml:"writeTimeout" default:"35"`
	HandleTimeout         *uint      `yaml:"handleTimeout" default:"60"` // 请求总处理超时时间
	RootPath              string     `yaml:"rootPath"`                   // 可以为空
	HeaderTraceID         string     `yaml:"headerTraceID" default:"X-Request-Id"`
	ControllerRootPkgName string     `yaml:"controllerRootPkgName" default:"controller"`
	EnablePprof           bool       `yaml:"enablePprof"`
	Mode                  ServerMode `yaml:"mode" default:"standard"`
	LimiterEnabled        bool       `yaml:"limiterEnabled"`
	Qps                   int        `yaml:"qps"`
}

func NewOptions(opts []Option) *Options {
	options := &Options{}
	_ = tools.DoTagFunc(options, nil, []func(reflect.StructField, reflect.Value, interface{}) error{tools.SetDefaultValueIfNil})

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

func WithPort(port uint) Option {
	return func(opts *Options) *Options {
		opts.Port = port
		return opts
	}
}

func WithReadTimeout(readTimeout uint) Option {
	return func(opts *Options) *Options {
		opts.ReadTimeout = readTimeout
		return opts
	}
}

func WithWriteTimeout(writeTimeout uint) Option {
	return func(opts *Options) *Options {
		opts.WriteTimeout = writeTimeout
		return opts
	}
}

func WithHandleTimeout(handleTimeout uint) Option {
	return func(opts *Options) *Options {
		opts.HandleTimeout = &handleTimeout
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
