-- 令牌桶限流器
-- KEYS[1]: 限流器唯一标识
-- ARGV[1]: 请求令牌数 (通常为1)
-- ARGV[2]: 令牌生成速率 (每秒)
-- ARGV[3]: 桶容量

local key = KEYS[1]
local requested = tonumber(ARGV[1])
local rate = tonumber(ARGV[2])
local capacity = tonumber(ARGV[3])

-- 获取当前时间（微秒精度）
local now = redis.call('TIME')
local nowMicros = tonumber(now[1]) * 1000000 + tonumber(now[2])

-- 获取桶状态
local bucket = redis.call('HMGET', key, 'tokens', 'last_time')
local tokens = tonumber(bucket[1])
local last_time = tonumber(bucket[2])

-- 初始化桶（首次请求或过期）
if not tokens or not last_time then
    tokens = capacity
    last_time = nowMicros
else
    -- 计算新增令牌（基于微秒差值）
    local elapsed = (nowMicros - last_time) / 1000000.0
    local add_tokens = elapsed * rate
    tokens = math.min(capacity, tokens + add_tokens)
    last_time = nowMicros
end

-- 判断是否允许请求
local allowed = false
if tokens >= requested then
    tokens = tokens - requested
    allowed = true
end

-- 更新桶状态并设置过期时间
redis.call('HMSET', key, 'tokens', tokens, 'last_time', last_time)
redis.call('EXPIRE', key, math.ceil(capacity / rate) + 180)

return allowed and 1 or 0