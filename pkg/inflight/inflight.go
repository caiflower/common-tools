/*
Copyright 2018 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package inflight

import (
	"sync"

	"github.com/caiflower/common-tools/pkg/syncx"
)

// Idempotent is the interface required to manage in flight requests.
type Idempotent interface {
	// String The CSI data types are generated using a protobuf.
	// The generated structures are guaranteed to implement the Stringer interface.
	// Example: https://github.com/container-storage-interface/spec/blob/master/lib/go/csi/csi.pb.go#L3508
	// We can use the generated string as the key of our internal inflight database of requests.
	String() string
}

// InFlight is a struct used to manage in flight requests.
type InFlight struct {
	mux      sync.Locker
	inFlight map[string]bool
}

// NewInFlight instantiates a InFlight structures.
func NewInFlight() *InFlight {
	return &InFlight{
		mux:      syncx.NewSpinLock(),
		inFlight: make(map[string]bool),
	}
}

// Insert inserts the entry to the current list of inflight requests.
// Returns false when the key already exists.
func (db *InFlight) Insert(entry Idempotent) bool {
	return db.InsertString(entry.String())
}

// InsertString inserts the string key to the current list of inflight requests.
// Returns false when the key already exists.
func (db *InFlight) InsertString(key string) bool {
	db.mux.Lock()
	defer db.mux.Unlock()

	_, ok := db.inFlight[key]
	if ok {
		return false
	}

	db.inFlight[key] = true
	return true
}

// Delete removes the entry from the inFlight entries map.
// It doesn't return anything, and will do nothing if the specified key doesn't exist.
func (db *InFlight) Delete(h Idempotent) {
	db.DeleteString(h.String())
}

// DeleteString removes the string key from the inFlight entries map.
// It doesn't return anything, and will do nothing if the specified key doesn't exist.
func (db *InFlight) DeleteString(key string) {
	db.mux.Lock()
	defer db.mux.Unlock()

	delete(db.inFlight, key)
}

func (db *InFlight) InFlight(h Idempotent) bool {
	return db.InFlightString(h.String())
}

func (db *InFlight) InFlightString(key string) bool {
	db.mux.Lock()
	defer db.mux.Unlock()

	_, ok := db.inFlight[key]
	return ok
}
