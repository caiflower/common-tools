package cache

import (
	"time"

	"github.com/patrickmn/go-cache"
)

// LocalCache 基于github.com/patrickmn/go-cache的本地cache
// 默认key没有超时时间
// 默认的清理内存中超时key的时间间隔是1min
var LocalCache = cache.New(cache.NoExpiration, 1*time.Minute)
