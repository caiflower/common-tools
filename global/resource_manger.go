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

package global

import (
	"os"
	"os/signal"
	"sort"
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

type DaemonResource interface {
	Resource
	Name() string
	Start() error
}

type packageResource struct {
	Resource
	order int
}

func (p *packageResource) Name() string {
	return "packageResource"
}

func (p *packageResource) Start() error {
	return nil
}

type resourceManger struct {
	lock                sync.Locker
	resources           []Resource
	daemons             []DaemonResource
	pagePackageResource []packageResource
	running             bool
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
	rm.pagePackageResource = append(rm.pagePackageResource, packageResource{resource, 1000000000})
}

func (rm *resourceManger) AddDaemonWithOrder(daemon DaemonResource, order int) {
	rm.lock.Lock()
	defer rm.lock.Unlock()

	for _, v := range rm.daemons {
		if v == daemon {
			return
		}
	}

	rm.daemons = append(rm.daemons, daemon)
	rm.pagePackageResource = append(rm.pagePackageResource, packageResource{daemon, order})
}

func (rm *resourceManger) AddDaemon(daemon DaemonResource) {
	rm.AddDaemonWithOrder(daemon, 100000)
}

func (rm *resourceManger) Signal() {
	if !rm.running {
		rm.lock.Lock()
		if !rm.running {
			rm.running = true

			sort.Slice(rm.pagePackageResource, func(i, j int) bool {
				return rm.pagePackageResource[i].order > rm.pagePackageResource[j].order
			})

			for _, resource := range rm.pagePackageResource {
				if err := resource.Start(); err != nil {
					logger.Fatal("Signal failed. Start '%s' resource failed. Error: %s", resource.Name(), err.Error())
				}
			}

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
	for _, resource := range rm.pagePackageResource {
		resource.Close()
	}
}
