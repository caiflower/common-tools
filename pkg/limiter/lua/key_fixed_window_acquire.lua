-- Key Fixed Window Concurrent Limiter: Acquire operation
-- Atomically acquires a concurrent slot for the given key.
-- Safe within Lua script: Redis executes Lua scripts atomically,
-- so no other client can modify the key between GET and INCR.
-- KEYS[1]: concurrent counter key (e.g. "key_fixed_window:{userKey}")
-- ARGV[1]: maxConcurrent (maximum allowed concurrent connections)
-- ARGV[2]: expireSeconds (TTL for leak prevention)

local key = KEYS[1]
local maxConcurrent = tonumber(ARGV[1])
local expireSeconds = tonumber(ARGV[2])

local current = tonumber(redis.call('GET', key)) or 0

if current >= maxConcurrent then
    return { 0, current }
end

local newCount = redis.call('INCR', key)
redis.call('EXPIRE', key, expireSeconds)

return { 1, newCount }
