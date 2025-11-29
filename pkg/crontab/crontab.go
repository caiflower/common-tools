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
	"github.com/caiflower/common-tools/pkg/basic"
	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/robfig/cron/v3"
)

var DefaultCronManger = NewCronTabManger("DefaultCronManger")

type CronManger struct {
	name string
	cron *cron.Cron
}

func NewCronTabManger(name string) *CronManger {
	return &CronManger{name: name, cron: cron.New(cron.WithSeconds())}
}

func (c *CronManger) GetCron() *cron.Cron {
	return c.cron
}
func GetCron() *cron.Cron {
	return DefaultCronManger.GetCron()
}

func (c *CronManger) Start() error {
	c.cron.Start()
	return nil
}

func (c *CronManger) Close() {
	c.cron.Stop()
}
func Close() {
	DefaultCronManger.Close()
}

func (c *CronManger) AddCronJob(spec string, job cron.Job) (cron.EntryID, error) {
	eid, err := c.cron.AddJob(spec, job)
	if err != nil {
		logger.Error("[Crontab] Add crontab failed. spec=%s. err=%v", spec, err)
	}
	logger.Info("[Crontab] Add crontab. spec=%s. jobId=%v. nextTime=%s", spec, eid, basic.TimeStandard(c.cron.Entry(eid).Next))
	return eid, err
}
func AddCronJob(spec string, job cron.Job) (cron.EntryID, error) {
	return DefaultCronManger.AddCronJob(spec, job)
}

func (c *CronManger) RemoveCronJob(id cron.EntryID) {
	c.cron.Remove(id)
}
func RemoveCronJob(id cron.EntryID) {
	DefaultCronManger.RemoveCronJob(id)
}
