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

package logger

import (
	"errors"
	"fmt"
	"strconv"
	"sync"
	"testing"

	golocalv1 "github.com/caiflower/common-tools/pkg/golocal/v1"
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

// func TestLoggerFileOut(t *testing.T) {
// 	logger := newLoggerHandler(&Config{
// 		Level:         TraceLevel,
// 		EnableTrace:   "True",
// 		Path:          os.Getenv("HOME") + "/logger",
// 		RollingPolicy: RollingPolicySize,
// 		MaxSize:       "1MB",
// 		Compress:      "False",
// 		AppenderNum:   100,
// 	})
// 	group := sync.WaitGroup{}

// 	for i := 1; i <= 1000000; i++ {
// 		group.Add(1)
// 		go func(i int) {
// 			defer group.Done()
// 			golocalv1.PutTraceID("lt-" + strconv.Itoa(i))
// 			defer golocalv1.Clean()
// 			logger.Trace("trace" + strconv.Itoa(i))
// 			logger.Debug("debug" + strconv.Itoa(i))
// 			logger.Info("info" + strconv.Itoa(i))
// 			logger.Warn("warn" + strconv.Itoa(i))
// 			logger.Error("error" + strconv.Itoa(i))
// 			logger.Fatal("fatal" + strconv.Itoa(i))
// 		}(i)
// 	}

// 	group.Wait()

// 	logger.Close()
// }

// func TestLoggerCut(t *testing.T) {
// 	logger := newLoggerHandler(&Config{
// 		Level:          TraceLevel,
// 		EnableTrace:    "True",
// 		Path:           os.Getenv("HOME") + "/logger",
// 		MaxSize:        "1MB",
// 		RollingPolicy:  RollingPolicyTimeAndSize,
// 		Compress:       "False",
// 		CleanBackup:    "True",
// 		BackupMaxCount: 5,
// 		BackupMaxDisk:  "10MB",
// 		AppenderNum:    5,
// 	})
// 	group := sync.WaitGroup{}

// 	for i := 1; i <= 1000000; i++ {
// 		group.Add(1)
// 		go func(i int) {
// 			defer group.Done()
// 			golocalv1.PutTraceID("lt-" + strconv.Itoa(i))
// 			defer golocalv1.Clean()
// 			logger.Trace("trace" + strconv.Itoa(i))
// 			logger.Debug("debug" + strconv.Itoa(i))
// 			logger.Info("info" + strconv.Itoa(i))
// 			logger.Warn("warn" + strconv.Itoa(i))
// 			logger.Error("error" + strconv.Itoa(i))
// 			logger.Fatal("fatal" + strconv.Itoa(i))
// 		}(i)
// 	}

// 	group.Wait()

// 	logger.Close()

// 	time.Sleep(2 * time.Second)
// }

func TestLoggerNonFMethods(t *testing.T) {
	logger := newLoggerHandler(&Config{Level: TraceLevel})

	// These should all work without go vet warnings
	err := fmt.Errorf("something went wrong")
	logger.Trace(err.Error())
	logger.Debug(err.Error())
	logger.Info(err.Error())
	logger.Warn(err.Error())
	logger.Error(err.Error())

	// Plain text
	logger.Trace("plain trace")
	logger.Debug("plain debug")
	logger.Info("plain info")
	logger.Warn("plain warn")
	logger.Error("plain error")

	logger.Close()
}

func TestLoggerFMethods(t *testing.T) {
	logger := newLoggerHandler(&Config{Level: TraceLevel})

	logger.Tracef("trace %s %d", "hello", 1)
	logger.Debugf("debug %s %d", "hello", 2)
	logger.Infof("info %s %d", "hello", 3)
	logger.Warnf("warn %s %d", "hello", 4)
	logger.Errorf("error %s %d", "hello", 5)

	logger.Close()
}

func TestLoggerFMethodsFormatting(t *testing.T) {
	logger := newMockLogger()
	mock := logger.logAppender.(*mockAppender)

	logger.Tracef("trace %s %d", "hello", 1)
	logger.Debugf("debug %s %d", "hello", 2)
	logger.Infof("info %s %d", "hello", 3)
	logger.Warnf("warn %s %d", "hello", 4)
	logger.Errorf("user %s failed with code %d", "alice", 403)
	logger.Errorf("simple error message")

	logger.Close()

	mock.mu.Lock()
	defer mock.mu.Unlock()

	want := []string{
		"trace hello 1",
		"debug hello 2",
		"info hello 3",
		"warn hello 4",
		"user alice failed with code 403",
		"simple error message",
	}
	if len(mock.results) != len(want) {
		t.Fatalf("expected %d log entries, got %d", len(want), len(mock.results))
	}
	for i, w := range want {
		if mock.results[i].content != w {
			t.Errorf("log[%d].content = %q, want %q", i, mock.results[i].content, w)
		}
	}
}

func TestLoggerNonFFormatting(t *testing.T) {
	logger := newMockLogger()
	mock := logger.logAppender.(*mockAppender)

	err := fmt.Errorf("connection timeout")
	logger.Error(err.Error())
	logger.Info("all systems operational")

	logger.Close()

	mock.mu.Lock()
	defer mock.mu.Unlock()

	if len(mock.results) != 2 {
		t.Fatalf("expected 2 log entries, got %d", len(mock.results))
	}

	if mock.results[0].content != "connection timeout" {
		t.Errorf("first log content = %q, want %q", mock.results[0].content, "connection timeout")
	}

	if mock.results[1].content != "all systems operational" {
		t.Errorf("second log content = %q, want %q", mock.results[1].content, "all systems operational")
	}
}

func newMockLogger() *LoggerHandler {
	logger := &LoggerHandler{
		level:     _trace,
		lock:      sync.RWMutex{},
		dataQueue: make(chan data, 100),
		closeChan: make(chan struct{}, 1),
		running:   true,
	}
	mock := &mockAppender{results: make([]data, 0)}
	logger.logAppender = mock
	go func() {
		for d := range logger.dataQueue {
			mock.write(d)
		}
		logger.closeChan <- struct{}{}
	}()
	return logger
}

type mockAppender struct {
	mu      sync.Mutex
	results []data
}

func (m *mockAppender) write(d data) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.results = append(m.results, d)
}
func (m *mockAppender) needCutLog() bool { return false }
func (m *mockAppender) rollingCutLog()   {}
func (m *mockAppender) loadLoggerFiles() {}
func (m *mockAppender) compressLog()     {}
func (m *mockAppender) cleanLog()        {}
func (m *mockAppender) close()           {}
func TestLoggerVetNoWarning(t *testing.T) {
	logger := newMockLogger()
	mock := logger.logAppender.(*mockAppender)

	// Simulates: logger.Error(err.Error()) — non-constant string, no vet warning
	err := errors.New("100% connection refused")
	logger.Error(err.Error())

	logger.Close()

	mock.mu.Lock()
	defer mock.mu.Unlock()

	if len(mock.results) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(mock.results))
	}
	// % should be preserved literally, not interpreted as format verb
	if mock.results[0].content != "100% connection refused" {
		t.Errorf("content = %q, want %q", mock.results[0].content, "100% connection refused")
	}
}
