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
	"reflect"
	"sync"
	"testing"
)

type events struct {
	mu  sync.Mutex
	log []string
}

func (e *events) add(name string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.log = append(e.log, name)
}

func (e *events) snapshot() []string {
	e.mu.Lock()
	defer e.mu.Unlock()

	return append([]string(nil), e.log...)
}

func newTestManager() *resourceManger {
	return &resourceManger{lock: &sync.Mutex{}}
}

type closeResource struct {
	name string
	ev   *events
}

func (r *closeResource) Close() { r.ev.add("close:" + r.name) }

type orderedCloseResource struct {
	closeResource
	order int
}

func (r *orderedCloseResource) Order() int { return r.order }

type plainDaemon struct {
	name string
	ev   *events
}

func (d *plainDaemon) Name() string { return d.name }
func (d *plainDaemon) Start() error { return nil }
func (d *plainDaemon) Close()       { d.ev.add("close:" + d.name) }

type orderedDaemon struct {
	name  string
	ev    *events
	order int
}

func (d *orderedDaemon) Name() string { return d.name }
func (d *orderedDaemon) Start() error { return nil }
func (d *orderedDaemon) Close()       { d.ev.add("close:" + d.name) }
func (d *orderedDaemon) Order() int   { return d.order }

func TestDestroyClosesInAscendingOrder(t *testing.T) {
	ev := &events{}
	m := newTestManager()
	m.AddWithOrder(&closeResource{"c", ev}, 300)
	m.AddWithOrder(&closeResource{"a", ev}, 100)
	m.AddWithOrder(&closeResource{"b", ev}, 200)

	m.destroy()

	want := []string{"close:a", "close:b", "close:c"}
	if got := ev.snapshot(); !reflect.DeepEqual(got, want) {
		t.Fatalf("close order = %v, want %v", got, want)
	}
}

func TestResourceWithOrderTakesPrecedence(t *testing.T) {
	ev := &events{}
	m := newTestManager()
	// the order argument (999) must be ignored in favour of Order() (5)
	m.AddWithOrder(&orderedCloseResource{closeResource{"x", ev}, 5}, 999)
	m.AddWithOrder(&closeResource{"y", ev}, 10)

	m.destroy()

	want := []string{"close:x", "close:y"}
	if got := ev.snapshot(); !reflect.DeepEqual(got, want) {
		t.Fatalf("close order = %v, want %v", got, want)
	}
}

func TestAddDeduplicatesSameResource(t *testing.T) {
	ev := &events{}
	m := newTestManager()
	r := &closeResource{"a", ev}
	m.Add(r)
	m.Add(r)
	m.AddWithOrder(r, 42)

	m.destroy()

	want := []string{"close:a"}
	if got := ev.snapshot(); !reflect.DeepEqual(got, want) {
		t.Fatalf("events = %v, want %v", got, want)
	}
}

func TestDaemonWithOrderClosesFirst(t *testing.T) {
	ev := &events{}
	m := newTestManager()
	// an HTTP server registered with AddDaemon must still be closed first
	m.AddDaemon(&orderedDaemon{"server", ev, OrderHTTPServer})
	m.AddWithOrder(&closeResource{"db", ev}, 1000)

	m.destroy()

	want := []string{"close:server", "close:db"}
	if got := ev.snapshot(); !reflect.DeepEqual(got, want) {
		t.Fatalf("close order = %v, want %v", got, want)
	}
}

func TestPlainDaemonClosesLast(t *testing.T) {
	ev := &events{}
	m := newTestManager()
	m.AddDaemon(&plainDaemon{"worker", ev})
	m.AddWithOrder(&closeResource{"db", ev}, 1000)

	m.destroy()

	want := []string{"close:db", "close:worker"}
	if got := ev.snapshot(); !reflect.DeepEqual(got, want) {
		t.Fatalf("close order = %v, want %v", got, want)
	}
}
