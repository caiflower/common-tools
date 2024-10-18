package logger

import (
	"fmt"
	"runtime"
	"sync"
	"time"

	golocalv1 "github.com/caiflower/common-tools/pkg/golocal/v1"
	"github.com/caiflower/common-tools/pkg/syncx"
)

const (
	_trace = iota
	_debug
	_info
	_warn
	_error
	_fatal

	Trace = "TRACE"
	Debug = "DEBUG"
	Info  = "INFO"
	Warn  = "WARN"
	Error = "ERROR"
	Fatal = "FATAL"

	_timeFormat = "2006-01-02 15:04:05"

	RollingPolicyTime        = "time"
	RollingPolicySize        = "size"
	RollingPolicyTimeAndSize = "timeAndSize"
	RollingPolicyClose       = "close"
)

type ILog interface {
	Trace(text string, v ...interface{})
	Debug(text string, v ...interface{})
	Info(text string, v ...interface{})
	Warn(text string, v ...interface{})
	Error(text string, v ...interface{})
	Fatal(text string, v ...interface{})
}

type data struct {
	timestamp time.Time
	traceID   string
	position  string
	level     string
	content   string
}

var defaultLogger loggerHandler

type loggerHandler struct {
	lock        sync.Locker
	level       int
	dataQueue   chan data
	logAppender Appender
}

type Config struct {
	Level         string `yaml:"log.level"`
	EnableTrace   bool   `yaml:"log.enableTrace"`
	QueueLength   int    `yaml:"log.queueLength"`
	AppenderNum   int    `yaml:"log.appenderNum"`
	TimeFormat    string `yaml:"log.timeFormat"`
	Path          string `yaml:"log.path"`
	FileName      string `yaml:"log.fileName"`
	Compress      bool   `yaml:"log.compress"`
	RollingPolicy string `yaml:"log.rollingPolicy"`
	MaxSize       string `yaml:"log.maxSize"` // 1MB, 10KB, 1GB
	MaxTime       string `yaml:"log.maxTime"` // 60s, 60m, 1h
}

func (lh *loggerHandler) Close() {
	lh.lock.Lock()
	defer lh.lock.Unlock()
	if lh.dataQueue != nil {
		for {
			if len(lh.dataQueue) == 0 {
				close(lh.dataQueue)
				lh.dataQueue = nil
				break
			}
		}
	}
	if lh.logAppender != nil {
		lh.logAppender.close()
	}
}

func newLoggerHandler(config *Config) *loggerHandler {
	if config.Level == "" {
		config.Level = Info
	}
	if config.QueueLength == 0 {
		config.QueueLength = 50000
	}
	if config.AppenderNum <= 0 {
		config.AppenderNum = 2
	}
	if config.TimeFormat == "" {
		config.TimeFormat = _timeFormat
	}
	if config.FileName == "" {
		config.FileName = "app.log"
	}
	if config.RollingPolicy == "" {
		config.RollingPolicy = RollingPolicyTimeAndSize
	}
	if config.MaxSize == "" {
		config.MaxSize = "500MB"
	}
	if config.MaxTime == "" {
		config.MaxTime = "24h"
	}

	logger := &loggerHandler{
		level:       getLevel(config.Level),
		lock:        syncx.NewSpinLock(),
		dataQueue:   make(chan data, config.QueueLength),
		logAppender: newLogAppender(config.TimeFormat, config.Path, config.FileName, config.RollingPolicy, config.MaxTime, config.MaxSize, config.EnableTrace),
	}

	for i := 0; i < config.AppenderNum; i++ {
		go func() {
			for d := range logger.dataQueue {
				logger.logAppender.write(d)
			}
		}()
	}
	return logger
}

func (lh *loggerHandler) Trace(text string, v ...interface{}) {
	lh.log(Trace, text, v...)
}

func (lh *loggerHandler) Debug(text string, v ...interface{}) {
	lh.log(Debug, text, v...)
}

func (lh *loggerHandler) Info(text string, v ...interface{}) {
	lh.log(Info, text, v...)
}

func (lh *loggerHandler) Warn(text string, v ...interface{}) {
	lh.log(Warn, text, v...)
}

func (lh *loggerHandler) Error(text string, v ...interface{}) {
	lh.log(Error, text, v...)
}

func (lh *loggerHandler) Fatal(text string, v ...interface{}) {
	lh.log(Fatal, text, v...)
}

func getLevel(level string) int {
	switch level {
	case Trace:
		return _trace
	case Debug:
		return _debug
	case Info:
		return _info
	case Warn:
		return _warn
	case Error:
		return _error
	case Fatal:
		return _fatal
	default:
		return _trace
	}
}

func (lh *loggerHandler) log(level string, text string, v ...interface{}) {
	if lh.level > getLevel(level) {
		return
	}

	_, file, line, _ := runtime.Caller(2)
	short := file
	for i := len(file) - 1; i > 0; i-- {
		if file[i] == '/' {
			short = file[i+1:]
			break
		}
	}
	file = short

	if lh.dataQueue != nil {
		lh.dataQueue <- data{
			timestamp: time.Now(),
			level:     level,
			content:   fmt.Sprintf(text, v...),
			traceID:   golocalv1.GetTraceID(),
			position:  fmt.Sprintf("%s:%d", file, line),
		}
	}
}
