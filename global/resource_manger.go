package global

import (
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/caiflower/common-tools/pkg/syncx"
)

// DefaultResourceManger
// 用于守护进程的优雅退出，如HTTP Server、database、cluster

type Resource interface {
	Close()
}

type resourceManger struct {
	lock      sync.Locker
	resources []Resource
	running   bool
}

var DefaultResourceManger = &resourceManger{lock: syncx.NewSpinLock()}

func (rm *resourceManger) Add(resource Resource) {
	rm.lock.Lock()
	defer rm.lock.Unlock()

	for _, v := range rm.resources {
		if v == resource {
			return
		}
	}

	rm.resources = append(rm.resources, resource)
}

func (rm *resourceManger) Signal() {
	if !rm.running {
		rm.lock.Lock()
		if !rm.running {
			rm.running = true
			sign := make(chan os.Signal)
			signal.Notify(sign, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
			rm.lock.Unlock()
			s := <-sign
			logger.Info("Accept signal %s. The application is shutting down...", s)
			rm.destroy()
			rm.running = false
		}
	}
}

func (rm *resourceManger) destroy() {
	for _, resource := range rm.resources {
		resource.Close()
	}
}
