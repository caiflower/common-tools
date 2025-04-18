package logger

import (
	golocalv1 "github.com/caiflower/common-tools/pkg/golocal/v1"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"
)

func TestLoggerStdOut(t *testing.T) {
	logger := newLoggerHandler(&Config{
		Level:       TraceLevel,
		EnableTrace: "True",
	})
	group := sync.WaitGroup{}

	for i := 1; i <= 10; i++ {
		group.Add(1)
		go func(i int) {
			defer group.Done()
			golocalv1.PutTraceID("lt-" + strconv.Itoa(i))
			defer golocalv1.Clean()
			logger.Trace("trace" + strconv.Itoa(i))
			logger.Debug("debug" + strconv.Itoa(i))
			logger.Info("info" + strconv.Itoa(i))
			logger.Warn("warn" + strconv.Itoa(i))
			logger.Error("error" + strconv.Itoa(i))
			logger.Fatal("fatal" + strconv.Itoa(i))
		}(i)
	}

	group.Wait()
	logger.Close()
}

func TestLoggerFileOut(t *testing.T) {
	logger := newLoggerHandler(&Config{
		Level:         TraceLevel,
		EnableTrace:   "True",
		Path:          os.Getenv("HOME") + "/logger",
		RollingPolicy: RollingPolicySize,
		MaxSize:       "10KB",
		Compress:      "False",
		AppenderNum:   100,
	})
	group := sync.WaitGroup{}

	for i := 1; i <= 1000; i++ {
		group.Add(1)
		go func(i int) {
			defer group.Done()
			golocalv1.PutTraceID("lt-" + strconv.Itoa(i))
			defer golocalv1.Clean()
			logger.Trace("trace" + strconv.Itoa(i))
			logger.Debug("debug" + strconv.Itoa(i))
			logger.Info("info" + strconv.Itoa(i))
			logger.Warn("warn" + strconv.Itoa(i))
			logger.Error("error" + strconv.Itoa(i))
			logger.Fatal("fatal" + strconv.Itoa(i))
		}(i)
	}

	group.Wait()

	logger.Close()
}

func TestLoggerCut(t *testing.T) {
	logger := newLoggerHandler(&Config{
		Level:          TraceLevel,
		EnableTrace:    "True",
		Path:           os.Getenv("HOME") + "/logger",
		MaxSize:        "10KB",
		RollingPolicy:  RollingPolicyTimeAndSize,
		Compress:       "True",
		CleanBackup:    "True",
		BackupMaxCount: 5,
		BackupMaxDisk:  "10MB",
		AppenderNum:    5,
	})
	group := sync.WaitGroup{}

	for i := 1; i <= 100; i++ {
		group.Add(1)
		go func(i int) {
			defer group.Done()
			golocalv1.PutTraceID("lt-" + strconv.Itoa(i))
			defer golocalv1.Clean()
			logger.Trace("trace" + strconv.Itoa(i))
			logger.Debug("debug" + strconv.Itoa(i))
			logger.Info("info" + strconv.Itoa(i))
			logger.Warn("warn" + strconv.Itoa(i))
			logger.Error("error" + strconv.Itoa(i))
			logger.Fatal("fatal" + strconv.Itoa(i))
		}(i)
	}

	group.Wait()

	logger.Close()

	time.Sleep(2 * time.Second)
}
