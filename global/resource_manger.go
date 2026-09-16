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

// OrderHTTPServer is the recommended close order for HTTP servers. It is the
// smallest order, so an HTTP server is closed first and stops accepting new
// requests before downstream resources (kafka/redis/db) are closed.
const OrderHTTPServer = 0

// ResourceWithOrder is an optional interface that resources can implement to specify their close order.
// Lower order values mean the resource will be closed earlier.
// Resources implementing this interface will have their Order() method called to determine close priority.
// If not implemented, the resource will use the default order provided when adding to the manager.
type ResourceWithOrder interface {
	Resource
	Order() int
}

type DaemonResource interface {
	Resource
	Name() string
	Start() error
}

type packageResource struct {
	Resource
	DaemonResource
	order int
}

func (p *packageResource) Name() string {
	return "packageResource"
}

func (p *packageResource) Close() {
	if p.DaemonResource != nil {
		p.DaemonResource.Close()
	} else {
		p.Resource.Close()
	}
}

func (p *packageResource) Start() error {
	if p.DaemonResource != nil {
		return p.DaemonResource.Start()
	}
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
	rm.AddWithOrder(resource, 1)
}

// AddWithOrder adds a resource with a specific close order.
// Lower order values mean the resource will be closed earlier.
// Recommended order values:
//   - HTTP servers: OrderHTTPServer, i.e. 0 (close first, stop accepting new requests / draining)
//   - Kafka consumers: 100 (close next, to stop consuming)
//   - Kafka producers: 200
//   - Redis/DB clients: 1000 (close last, after consumers and servers)
//   - Default(Add): 1
func (rm *resourceManger) AddWithOrder(resource Resource, order int) {
	rm.lock.Lock()
	defer rm.lock.Unlock()

	for _, v := range rm.resources {
		if v == resource {
			return
		}
	}

	rm.resources = append(rm.resources, resource)
	rm.pagePackageResource = append(rm.pagePackageResource,
		packageResource{Resource: resource, order: closeOrder(resource, order)})
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
	rm.pagePackageResource = append(rm.pagePackageResource,
		packageResource{DaemonResource: daemon, order: closeOrder(daemon, order)})
}

// closeOrder returns the resource's own Order() when it implements
// ResourceWithOrder, otherwise it returns the provided default order.
func closeOrder(resource interface{}, order int) int {
	if rwo, ok := resource.(ResourceWithOrder); ok {
		return rwo.Order()
	}

	return order
}

func (rm *resourceManger) AddDaemon(daemon DaemonResource) {
	rm.AddDaemonWithOrder(daemon, 100000)
}

func (rm *resourceManger) Signal() {
	if !rm.running {
		rm.lock.Lock()
		if !rm.running {
			rm.running = true

			// start resources in reverse close order, so that downstream
			// dependencies (db/cache) are ready before the servers.
			rm.sortByOrder()

			for _, resource := range rm.pagePackageResource {
				if err := resource.Start(); err != nil {
					logger.Fatal("Signal failed. Start '%s' resource failed. Error: %s", resource.Name(), err.Error())
				}
			}

			sign := make(chan os.Signal, 1)
			signal.Notify(sign, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
			rm.lock.Unlock()
			s := <-sign
			logger.Info("Accept signal %s. The application is shutting down...", s)
			rm.destroy()
			rm.running = false
		}
	}
}

// sortByOrder sorts resources by close order descending, so that iterating in
// reverse closes them from the smallest order to the largest order.
func (rm *resourceManger) sortByOrder() {
	sort.Slice(rm.pagePackageResource, func(i, j int) bool {
		return rm.pagePackageResource[i].order > rm.pagePackageResource[j].order
	})
}

func (rm *resourceManger) destroy() {
	rm.sortByOrder()

	for i := len(rm.pagePackageResource) - 1; i >= 0; i-- {
		rm.pagePackageResource[i].Close()
	}
}
