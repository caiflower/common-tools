package redisd

import "github.com/redis/go-redis/v9"

// Lua scripts for atomic CAS operations.
// All scripts return 1 on success, 0 on failure.

// casWorkerAndState atomically compares worker and updates worker + state.
// KEYS[1] = entity hash key
// ARGV[1] = oldWorker, ARGV[2] = newWorker, ARGV[3] = newState
var casWorkerAndState = redis.NewScript(`
if redis.call('HGET', KEYS[1], 'worker') == ARGV[1] then
    redis.call('HSET', KEYS[1], 'worker', ARGV[2], 'state', ARGV[3])
    return 1
end
return 0
`)

// casWorkerAndRollback atomically compares worker and updates worker + rollback.
// KEYS[1] = subtask hash key
// ARGV[1] = oldWorker, ARGV[2] = newWorker, ARGV[3] = rollback
var casWorkerAndRollback = redis.NewScript(`
if redis.call('HGET', KEYS[1], 'worker') == ARGV[1] then
    redis.call('HSET', KEYS[1], 'worker', ARGV[2], 'rollback', ARGV[3])
    return 1
end
return 0
`)
