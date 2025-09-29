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

 package crontab

import (
	"fmt"
	"runtime"
	"testing"
	"time"

	"github.com/caiflower/common-tools/global"
	"github.com/caiflower/common-tools/pkg/basic"
	golocalv1 "github.com/caiflower/common-tools/pkg/golocal/v1"
	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/caiflower/common-tools/pkg/tools"
	"github.com/robfig/cron/v3"
)

type test struct {
	eid cron.EntryID
}

func TestAddCronJob(t *testing.T) {
	te := &test{}
	Start()

	id, err := AddCronJob("* * * * * *", te)
	if err != nil {
		panic(err)
	}
	te.eid = id

	go func() {
		time.Sleep(5 * time.Second)
		Close()
	}()

	global.DefaultResourceManger.Signal()
}

func (t *test) Run() {
	golocalv1.PutTraceID(tools.UUID())
	defer golocalv1.Clean()
	fmt.Println("run", time.Now(), runtime.NumGoroutine())
	logger.Info("[Crontab] jobId=%v. nextTime=%s", t.eid, basic.TimeStandard(GetCron().Entry(t.eid).Next))
}
