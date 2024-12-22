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
