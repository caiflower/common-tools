package logger

import (
	"fmt"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
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

	TraceLevel = "TRACE"
	DebugLevel = "DEBUG"
	InfoLevel  = "INFO"
	WarnLevel  = "WARN"
	ErrorLevel = "ERROR"
	FatalLevel = "FATAL"

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

var defaultLogger = newLoggerHandler(&Config{})

func Trace(text string, v ...interface{}) {
	defaultLogger.log(TraceLevel, text, v...)
}
func Debug(text string, v ...interface{}) {
	defaultLogger.log(DebugLevel, text, v...)
}
func Info(text string, v ...interface{}) {
	defaultLogger.log(InfoLevel, text, v...)
}
func Warn(text string, v ...interface{}) {
	defaultLogger.log(WarnLevel, text, v...)
}
func Error(text string, v ...interface{}) {
	defaultLogger.log(ErrorLevel, text, v...)
}
func Fatal(text string, v ...interface{}) {
	defaultLogger.log(FatalLevel, text, v...)
}

type LoggerHandler struct {
	lock        sync.Locker
	level       int
	dataQueue   chan data
	logAppender Appender
	running     int64
	enableColor bool
}

type Config struct {
	Level          string `yaml:"level"`             // 日志级别
	EnableTrace    string `yaml:"trace"`             // 是否开启Trace, True/False。默认False
	QueueLength    int    `yaml:"queueLength"`       // 缓存队列大小，默认50000
	AppenderNum    int    `yaml:"appenderNum"`       // 日志输出器数量，默认2
	TimeFormat     string `yaml:"timeFormat"`        // 日志时间输出格式
	Path           string `yaml:"path"`              // 日志存储目录
	FileName       string `yaml:"fileName"`          // 日志文件名称
	RollingPolicy  string `yaml:"rollingPolicy"`     // 日志切分策略。
	MaxSize        string `yaml:"maxSize"`           // 1MB, 10KB, 1GB
	MaxTime        string `yaml:"maxTime"`           // 60s, 60m, 1h
	Compress       string `yaml:"compress"`          // 是否对备份日志进行压缩, True/False。默认True
	CleanBackup    string `yaml:"cleanBackup"`       // 是否清理备份日志文件, True/False。默认True
	BackupMaxCount int    `yaml:"backupMaxCount"`    // 保留备份日志文件最大数量。log.cleanBackup=true生效，默认10
	BackupMaxDisk  string `yaml:"backupMaxDiskSize"` // 保留备份日志文件磁盘最大大小, 1MB, 10KB, 1GB。log.cleanBackup=true生效，默认1GB
	EnableColor    string `yaml:"color"`             // 是否开启颜色
}

func (lh *LoggerHandler) Close() {
	lh.lock.Lock()
	defer lh.lock.Unlock()
	if lh.dataQueue != nil {
		close(lh.dataQueue)
		lh.dataQueue = nil
	}
	for {
		if lh.running == 0 {
			break
		}
	}
	if lh.logAppender != nil {
		lh.logAppender.close()
	}
}

func DefaultLogger() *LoggerHandler {
	return defaultLogger
}

func InitLogger(config *Config) {
	defaultLogger = newLoggerHandler(config)
}

func NewLogger(config *Config) *LoggerHandler {
	return newLoggerHandler(config)
}

func newLoggerHandler(config *Config) *LoggerHandler {
	if config.Level == "" {
		config.Level = InfoLevel
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
	compress := true
	if config.Compress != "" {
		compress, _ = strconv.ParseBool(config.Compress)
	}
	enableTrace := true
	if config.EnableTrace != "" {
		enableTrace, _ = strconv.ParseBool(config.EnableTrace)
	}
	if config.BackupMaxCount <= 0 {
		config.BackupMaxCount = 10
	}
	if config.BackupMaxDisk == "" {
		config.BackupMaxDisk = "1GB"
	}
	enableCleanBackup := true
	if config.CleanBackup != "" {
		enableCleanBackup, _ = strconv.ParseBool(config.CleanBackup)
	}
	enableColor := false
	if config.EnableColor != "" {
		enableColor, _ = strconv.ParseBool(config.EnableColor)
	}

	logger := &LoggerHandler{
		level:       getLevel(config.Level),
		lock:        syncx.NewSpinLock(),
		dataQueue:   make(chan data, config.QueueLength),
		logAppender: newLogAppender(config.TimeFormat, config.Path, config.FileName, config.RollingPolicy, config.MaxTime, config.MaxSize, config.BackupMaxDisk, config.BackupMaxCount, enableTrace, compress, enableCleanBackup, enableColor),
	}

	for i := 0; i < config.AppenderNum; i++ {
		go func() {
			for d := range logger.dataQueue {
				atomic.AddInt64(&logger.running, 1)
				logger.logAppender.write(d)
				atomic.AddInt64(&logger.running, -1)
			}
		}()
	}

	return logger
}

func (lh *LoggerHandler) Trace(text string, v ...interface{}) {
	lh.log(TraceLevel, text, v...)
}

func (lh *LoggerHandler) Debug(text string, v ...interface{}) {
	lh.log(DebugLevel, text, v...)
}

func (lh *LoggerHandler) Info(text string, v ...interface{}) {
	lh.log(InfoLevel, text, v...)
}

func (lh *LoggerHandler) Warn(text string, v ...interface{}) {
	lh.log(WarnLevel, text, v...)
}

func (lh *LoggerHandler) Error(text string, v ...interface{}) {
	lh.log(ErrorLevel, text, v...)
}

func (lh *LoggerHandler) Fatal(text string, v ...interface{}) {
	lh.log(FatalLevel, text, v...)
}

func getLevel(level string) int {
	switch level {
	case TraceLevel:
		return _trace
	case DebugLevel:
		return _debug
	case InfoLevel:
		return _info
	case WarnLevel:
		return _warn
	case ErrorLevel:
		return _error
	case FatalLevel:
		return _fatal
	default:
		return _trace
	}
}

func getLevelColor(level string) string {
	switch level {
	case TraceLevel:
		return fmt.Sprintf("\033[1;37m%s\033[0m", TraceLevel)
	case DebugLevel:
		return fmt.Sprintf("\033[1;36m%s\033[0m", DebugLevel)
	case InfoLevel:
		return fmt.Sprintf("\033[1;32m%s\033[0m", InfoLevel)
	case WarnLevel:
		return fmt.Sprintf("\033[1;33m%s\033[0m", WarnLevel)
	case ErrorLevel:
		return fmt.Sprintf("\033[1;31m%s\033[0m", ErrorLevel)
	case FatalLevel:
		return fmt.Sprintf("\033[1;31m%s\033[0m", FatalLevel)
	default:
		return level
	}
}

func (lh *LoggerHandler) log(level string, text string, v ...interface{}) {
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
