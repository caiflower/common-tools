package logger

import (
	"os"
	"strconv"
	"sync"
	"testing"

	golocalv1 "github.com/caiflower/common-tools/pkg/golocal/v1"
)

func TestLoggerStdOut(t *testing.T) {
	logger := newLoggerHandler(&Config{
		Level:       Trace,
		EnableTrace: true,
	})
	group := sync.WaitGroup{}

	for i := 1; i <= 10; i++ {
		group.Add(1)
		go func(i int) {
			defer group.Done()
			golocalv1.PutTraceID("lt-" + strconv.Itoa(i))
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
		Level:       Trace,
		EnableTrace: true,
		Path:        os.Getenv("HOME"),
	})
	group := sync.WaitGroup{}

	for i := 1; i <= 10; i++ {
		group.Add(1)
		go func(i int) {
			defer group.Done()
			golocalv1.PutTraceID("lt-" + strconv.Itoa(i))
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
		Level:         Trace,
		EnableTrace:   true,
		Path:          os.Getenv("HOME"),
		MaxSize:       "10KB",
		RollingPolicy: RollingPolicySize,
	})
	group := sync.WaitGroup{}

	for i := 11; i <= 40; i++ {
		group.Add(1)
		go func(i int) {
			defer group.Done()
			golocalv1.PutTraceID("lt-" + strconv.Itoa(i))
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
