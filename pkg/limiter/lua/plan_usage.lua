-- Plan Token Budget: Usage query operation (pure read-only, no side effects)
-- _total is the authoritative source for totalUsed; buckets are for TTL-based cleanup only.
-- This script does NOT modify any data (no EXPIRE, no HDEL).
-- NOTE: adjustedTotal is an estimate based on currently visible expired buckets.
-- The actual _total deduction only happens in Allow/Refund scripts. If Usage is
-- called repeatedly without Allow/Refund, _total stays stale and adjustedTotal
-- is recalculated each time from the same expired bucket data.
-- KEYS[1]: plan:{userId}:{planId}
-- ARGV[1]: budget (weighted units)
-- ARGV[2]: window (seconds)
-- ARGV[3]: bucketSize (seconds)

local key = KEYS[1]
local budget = tonumber(ARGV[1])
local window = tonumber(ARGV[2])
local bucketSize = tonumber(ARGV[3])

local exists = redis.call('EXISTS', key)
if exists == 0 then
    return { budget, 0, {} }
end

local now = tonumber(redis.call('TIME')[1])
local windowStart = now - window

local totalUsed = tonumber(redis.call('HGET', key, '_total')) or 0

local allFields = redis.call('HGETALL', key)
local expiredSum = 0
local modelStats = {}

for i = 1, #allFields, 2 do
    local field = allFields[i]
    local value = tonumber(allFields[i + 1])

    if field == '_total' then
        -- already read above
    elseif string.sub(field, 1, 1) == '_' then
        local modelName = string.sub(field, 2)
        modelStats[#modelStats + 1] = modelName
        modelStats[#modelStats + 1] = tostring(value or 0)
    else
        local bucketTs = tonumber(field)
        if bucketTs and bucketTs <= windowStart then
            expiredSum = expiredSum + (value or 0)
        end
    end
end

local adjustedTotal = math.max(0, totalUsed - expiredSum)

return { budget - adjustedTotal, adjustedTotal, modelStats }
