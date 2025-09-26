//go:build go1.4
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
