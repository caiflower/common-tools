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
	"testing"
	"time"

	"github.com/caiflower/common-tools/pkg/logger"
)

func TestRegularJob(t *testing.T) {
	fn := func() {
		logger.Info("do testRegularJob")
	}

	job := NewRegularJob("testRegularJob", fn, WithInterval(time.Second*2))

	job.Run()

	time.Sleep(time.Second * 10)
	job.Stop()
}

func TestPanicRegularJob(t *testing.T) {
	fn := func() {
		panic("test panic")
	}

	job := NewRegularJob("testRegularJob", fn, WithInterval(time.Second*2), WithIgnorePanic())

	job.Run()

	time.Sleep(time.Second * 10)
	job.Stop()
}
