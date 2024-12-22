package crontab

import (
	"github.com/caiflower/common-tools/global"
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

func (c *CronManger) Start() {
	c.cron.Start()
	global.DefaultResourceManger.Add(c)
}

func Start() {
	DefaultCronManger.Start()
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
