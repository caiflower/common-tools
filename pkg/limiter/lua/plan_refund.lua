-- Plan Token Budget: Refund operation
-- _total is the authoritative source for totalUsed; buckets are for TTL-based cleanup only.
-- Refund reduces _total directly; bucket reduction is best-effort for monitoring accuracy.
-- KEYS[1]: plan:{userId}:{planId}
-- ARGV[1]: refund tokens (raw)
-- ARGV[2]: budget (weighted units)
-- ARGV[3]: window (seconds)
-- ARGV[4]: bucketSize (seconds)
-- ARGV[5]: model name
-- ARGV[6]: model weight

local key = KEYS[1]
local refundTokens = tonumber(ARGV[1])
local budget = tonumber(ARGV[2])
local window = tonumber(ARGV[3])
local bucketSize = tonumber(ARGV[4])
local model = ARGV[5]
local weight = tonumber(ARGV[6])

local now = tonumber(redis.call('TIME')[1])
local currentBucket = math.floor(now / bucketSize) * bucketSize
local windowStart = now - window

local totalUsed = tonumber(redis.call('HGET', key, '_total')) or 0

local allFields = redis.call('HGETALL', key)
local modelUsed = 0
local expiredSum = 0
local expiredFields = {}

for i = 1, #allFields, 2 do
    local field = allFields[i]
    local value = tonumber(allFields[i + 1])

    if field == '_total' then
        -- already read above
    elseif field == '_' .. model then
        modelUsed = value or 0
    elseif string.sub(field, 1, 1) == '_' then
        -- other model fields, skip
    else
        local bucketTs = tonumber(field)
        if bucketTs and bucketTs <= windowStart then
            expiredSum = expiredSum + (value or 0)
            expiredFields[#expiredFields + 1] = field
        end
    end
end

for i = 1, #expiredFields do
    redis.call('HDEL', key, expiredFields[i])
end

if expiredSum > 0 then
    totalUsed = math.max(0, totalUsed - expiredSum)
end

local weightedRefund = math.floor(refundTokens * weight)
local newTotal = math.max(0, totalUsed - weightedRefund)
local newModelUsed = math.max(0, modelUsed - refundTokens)

local currentBucketStr = tostring(currentBucket)
local currentBucketUsed = tonumber(redis.call('HGET', key, currentBucketStr)) or 0
local newBucketUsed = math.max(0, currentBucketUsed - weightedRefund)

if newBucketUsed > 0 then
    redis.call('HSET', key, currentBucketStr, newBucketUsed)
else
    redis.call('HDEL', key, currentBucketStr)
end

redis.call('HSET', key, '_total', newTotal)
redis.call('HSET', key, '_' .. model, newModelUsed)

redis.call('EXPIRE', key, window + 3600)

return { budget - newTotal, newTotal }
