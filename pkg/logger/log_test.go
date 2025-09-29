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
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

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

func TestLoggerFileOut(t *testing.T) {
	logger := newLoggerHandler(&Config{
		Level:         TraceLevel,
		EnableTrace:   "True",
		Path:          os.Getenv("HOME") + "/logger",
		RollingPolicy: RollingPolicySize,
		MaxSize:       "1MB",
		Compress:      "False",
		AppenderNum:   100,
	})
	group := sync.WaitGroup{}

	for i := 1; i <= 1000000; i++ {
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
		MaxSize:        "1MB",
		RollingPolicy:  RollingPolicyTimeAndSize,
		Compress:       "False",
		CleanBackup:    "True",
		BackupMaxCount: 5,
		BackupMaxDisk:  "10MB",
		AppenderNum:    5,
	})
	group := sync.WaitGroup{}

	for i := 1; i <= 1000000; i++ {
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
