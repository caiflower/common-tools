-- Key Fixed Window Concurrent Limiter: Release operation
-- Atomically releases a concurrent slot for the given key.
-- Prevents counter from going below 0 (underflow protection).
-- KEYS[1]: concurrent counter key (e.g. "key_fixed_window:{userKey}")
-- ARGV[1]: expireSeconds (TTL refresh, optional - 0 means no refresh)

local key = KEYS[1]
local expireSeconds = tonumber(ARGV[1])

local current = tonumber(redis.call('GET', key)) or 0

if current <= 0 then
    return 0
end

local newCount = redis.call('DECR', key)

if newCount < 0 then
    redis.call('SET', key, 0)
    newCount = 0
end

if expireSeconds > 0 and newCount > 0 then
    redis.call('EXPIRE', key, expireSeconds)
end

return newCount
