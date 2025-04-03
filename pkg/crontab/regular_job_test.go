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
