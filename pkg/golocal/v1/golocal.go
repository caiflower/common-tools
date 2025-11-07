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

// +build go1.4

package v1

import (
	"context"
	"sync"

	"github.com/modern-go/gls"
)

const (
	RequestID = "X-Request-ID"
	GoContext = "Go-Context"
)

var localMap sync.Map

func getGoID() int64 {
	return gls.GoID()
}

func getMapByGoID(goID int64) *sync.Map {
	value, _ := localMap.Load(goID)
	if value == nil {
		_tmp := &sync.Map{}
		localMap.Store(goID, _tmp)
		return _tmp
	}
	return value.(*sync.Map)
}

func GetLocalMap() *sync.Map {
	return getMapByGoID(getGoID())
}

func PutLocalMap(_map *sync.Map) {
	localMap.Store(getGoID(), _map)
}

func PutTraceID(value string) {
	m := getMapByGoID(getGoID())
	m.Store(RequestID, value)
}

func GetTraceID() string {
	m := getMapByGoID(getGoID())
	if v, ok := m.Load(RequestID); ok {
		return v.(string)
	} else {
		return ""
	}
}

func Put(key string, value interface{}) {
	getMapByGoID(getGoID()).Store(key, value)
}

func Get(key string) interface{} {
	if v, ok := getMapByGoID(getGoID()).Load(key); ok {
		return v
	} else {
		return nil
	}
}

func Clean() {
	id := getGoID()
	if v := getMapByGoID(id); v != nil {
		localMap.Delete(id)
	}
}

func PutContext(ctx context.Context) {
	m := getMapByGoID(getGoID())
	m.Store(GoContext, ctx)
}

func GetContext() context.Context {
	m := getMapByGoID(getGoID())
	if v, ok := m.Load(GoContext); ok {
		return v.(context.Context)
	} else {
		background := context.Background()
		PutContext(background)
		return background
	}
}
